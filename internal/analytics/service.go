package analytics

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"vitamins-backend_2/internal/db"
)

const insertAnalyticsEvent = `
INSERT INTO analytics_events (
    event_id, occurred_at, received_at, user_id, anonymous_id, session_id,
    event_name, properties, request_id, app_version, platform
) VALUES (
    $1, $2, now(), $3, $4, $5, $6, $7, $8, $9, $10
) ON CONFLICT (event_id) DO NOTHING
`

const listAnalyticsEvents = `
SELECT event_id, occurred_at, received_at, user_id, anonymous_id, session_id,
       event_name, properties, request_id, app_version, platform
FROM analytics_events
WHERE ($1::timestamptz IS NULL OR occurred_at >= $1)
  AND ($2::timestamptz IS NULL OR occurred_at <= $2)
  AND ($3::text IS NULL OR event_name = $3)
  AND ($4::bigint IS NULL OR user_id = $4)
ORDER BY occurred_at
LIMIT $5 OFFSET $6
`

type Service struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewService(q *db.Queries, pool *pgxpool.Pool) *Service {
	return &Service{q: q, pool: pool}
}

type ingestEvent struct {
	EventID     pgtype.UUID
	OccurredAt  pgtype.Timestamptz
	UserID      pgtype.Int8
	AnonymousID pgtype.UUID
	SessionID   pgtype.UUID
	EventName   string
	Properties  []byte
	RequestID   pgtype.Text
	AppVersion  pgtype.Text
	Platform    pgtype.Text
}

func (s *Service) Ingest(ctx context.Context, tokenUserID *int64, req BatchRequest) (IngestResponse, error) {
	if len(req.Events) == 0 {
		return IngestResponse{}, ErrEmptyBatch
	}
	if len(req.Events) > maxBatchSize {
		return IngestResponse{}, ErrBatchTooLarge
	}

	consent := false
	if tokenUserID != nil {
		var err error
		consent, err = s.q.GetUserAnalyticsConsent(ctx, *tokenUserID)
		if err != nil {
			return IngestResponse{}, ErrUserNotFound
		}
	}

	events := make([]ingestEvent, 0, len(req.Events))
	for _, e := range req.Events {
		if e.EventName == "" {
			return IngestResponse{}, ErrInvalidEventName
		}
		eventID, err := parseUUID(e.EventID)
		if err != nil {
			return IngestResponse{}, ErrInvalidEventID
		}
		sessionID, err := parseUUIDAllowEmpty(e.SessionID, ErrInvalidSessionID)
		if err != nil {
			return IngestResponse{}, err
		}
		occurredAt, err := parseTimestamp(e.OccurredAt)
		if err != nil {
			return IngestResponse{}, ErrInvalidOccurredAt
		}

		var anon pgtype.UUID
		if e.AnonymousID != nil && *e.AnonymousID != "" {
			anon, err = parseUUIDAllowEmpty(*e.AnonymousID, ErrInvalidAnonymousID)
			if err != nil {
				return IngestResponse{}, err
			}
		}

		var userID pgtype.Int8
		if tokenUserID != nil && consent {
			userID = pgtype.Int8{Int64: *tokenUserID, Valid: true}
		}

		if !userID.Valid && !anon.Valid {
			return IngestResponse{}, ErrAnonymousRequired
		}

		props := sanitizeProperties(e.Properties)
		if props == nil {
			props = map[string]any{}
		}
		propsBytes, err := json.Marshal(props)
		if err != nil {
			return IngestResponse{}, err
		}
		requestID := textFromPtr(e.RequestID)
		appVersion := textFromPtr(e.AppVersion)
		platform := textFromPtr(e.Platform)

		events = append(events, ingestEvent{
			EventID:     eventID,
			OccurredAt:  pgtype.Timestamptz{Time: occurredAt, Valid: true},
			UserID:      userID,
			AnonymousID: anon,
			SessionID:   sessionID,
			EventName:   e.EventName,
			Properties:  propsBytes,
			RequestID:   requestID,
			AppVersion:  appVersion,
			Platform:    platform,
		})
	}

	batch := &pgx.Batch{}
	for _, e := range events {
		batch.Queue(insertAnalyticsEvent,
			e.EventID,
			e.OccurredAt,
			nullInt8(e.UserID),
			nullUUID(e.AnonymousID),
			e.SessionID,
			e.EventName,
			e.Properties,
			e.RequestID,
			e.AppVersion,
			e.Platform,
		)
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()

	accepted := 0
	for range events {
		ct, err := br.Exec()
		if err != nil {
			return IngestResponse{}, err
		}
		if ct.RowsAffected() > 0 {
			accepted++
		}
	}
	return IngestResponse{
		Accepted:     accepted,
		Deduplicated: len(events) - accepted,
	}, nil
}

func (s *Service) SetConsent(ctx context.Context, userID int64, consent bool) error {
	return s.q.UpdateUserAnalyticsConsent(ctx, userID, consent)
}

func (s *Service) GetConsent(ctx context.Context, userID int64) (bool, error) {
	return s.q.GetUserAnalyticsConsent(ctx, userID)
}

func (s *Service) Export(ctx context.Context, filter ExportFilter) ([]ExportRow, error) {
	from, err := parseFilterTime(filter.From)
	if err != nil {
		return nil, err
	}
	to, err := parseFilterTime(filter.To)
	if err != nil {
		return nil, err
	}
	eventName := textFromPtr(filter.EventName)
	userID := int8FromPtr(filter.UserID)

	rows, err := s.pool.Query(ctx, listAnalyticsEvents, from, to, eventName, userID, filter.Limit, filter.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ExportRow
	for rows.Next() {
		var row ExportRow
		var eventID pgtype.UUID
		var occurredAt pgtype.Timestamptz
		var receivedAt pgtype.Timestamptz
		var userID pgtype.Int8
		var anonID pgtype.UUID
		var sessionID pgtype.UUID
		var eventName string
		var props []byte
		var requestID pgtype.Text
		var appVersion pgtype.Text
		var platform pgtype.Text
		if err := rows.Scan(&eventID, &occurredAt, &receivedAt, &userID, &anonID, &sessionID, &eventName, &props, &requestID, &appVersion, &platform); err != nil {
			return nil, err
		}
		row.EventID = uuidToString(eventID)
		row.OccurredAt = occurredAt.Time.Format(time.RFC3339Nano)
		row.ReceivedAt = receivedAt.Time.Format(time.RFC3339Nano)
		if userID.Valid {
			row.UserID = &userID.Int64
		}
		if anonID.Valid {
			anonStr := uuidToString(anonID)
			row.AnonymousID = &anonStr
		}
		row.SessionID = uuidToString(sessionID)
		row.EventName = eventName
		row.Properties = string(props)
		if requestID.Valid {
			v := requestID.String
			row.RequestID = &v
		}
		if appVersion.Valid {
			v := appVersion.String
			row.AppVersion = &v
		}
		if platform.Valid {
			v := platform.String
			row.Platform = &v
		}
		out = append(out, row)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

func parseFilterTime(raw *string) (pgtype.Timestamptz, error) {
	if raw == nil || *raw == "" {
		return pgtype.Timestamptz{}, nil
	}
	t, err := time.Parse(time.RFC3339Nano, *raw)
	if err != nil {
		t, err = time.Parse(time.RFC3339, *raw)
	}
	if err != nil {
		return pgtype.Timestamptz{}, err
	}
	return pgtype.Timestamptz{Time: t, Valid: true}, nil
}

func textFromPtr(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	value := strings.TrimSpace(*v)
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func int8FromPtr(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func nullUUID(v pgtype.UUID) pgtype.UUID {
	if v.Valid {
		return v
	}
	return pgtype.UUID{}
}

func nullInt8(v pgtype.Int8) pgtype.Int8 {
	if v.Valid {
		return v
	}
	return pgtype.Int8{}
}
