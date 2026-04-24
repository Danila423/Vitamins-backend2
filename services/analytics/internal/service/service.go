// Package service implements the analytics use-cases (ingest, consent and
// admin export). Keeping it separate from the HTTP handler lets tests target
// the business logic directly with fakes.
package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"vitamins-backend_2/pkg/db"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	repo Repository
}

func NewService(q *db.Queries, pool *pgxpool.Pool) *Service {
	return NewServiceWithDeps(NewRepository(q, pool))
}

func NewServiceWithDeps(repo Repository) *Service {
	return &Service{repo: repo}
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
		consent, err = s.repo.GetUserAnalyticsConsent(ctx, *tokenUserID)
		if err != nil {
			return IngestResponse{}, ErrUserNotFound
		}
	}

	events := make([]IngestEventRecord, 0, len(req.Events))
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

		events = append(events, IngestEventRecord{
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

	accepted, err := s.repo.IngestBatch(ctx, events)
	if err != nil {
		return IngestResponse{}, err
	}

	return IngestResponse{
		Accepted:     accepted,
		Deduplicated: len(events) - accepted,
	}, nil
}

func (s *Service) SetConsent(ctx context.Context, userID int64, consent bool) error {
	return s.repo.UpdateUserAnalyticsConsent(ctx, userID, consent)
}

func (s *Service) GetConsent(ctx context.Context, userID int64) (bool, error) {
	return s.repo.GetUserAnalyticsConsent(ctx, userID)
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

	rows, err := s.repo.Export(ctx, ExportQueryParams{
		From:      from,
		To:        to,
		EventName: textFromPtr(filter.EventName),
		UserID:    int8FromPtr(filter.UserID),
		Limit:     filter.Limit,
		Offset:    filter.Offset,
	})
	if err != nil {
		return nil, err
	}

	out := make([]ExportRow, 0, len(rows))
	for _, r := range rows {
		row := ExportRow{
			EventID:    uuidToString(r.EventID),
			OccurredAt: r.OccurredAt.Time.Format(time.RFC3339Nano),
			ReceivedAt: r.ReceivedAt.Time.Format(time.RFC3339Nano),
			SessionID:  uuidToString(r.SessionID),
			EventName:  r.EventName,
			Properties: string(r.Properties),
		}
		if r.UserID.Valid {
			row.UserID = &r.UserID.Int64
		}
		if r.AnonymousID.Valid {
			anonStr := uuidToString(r.AnonymousID)
			row.AnonymousID = &anonStr
		}
		if r.RequestID.Valid {
			v := r.RequestID.String
			row.RequestID = &v
		}
		if r.AppVersion.Valid {
			v := r.AppVersion.String
			row.AppVersion = &v
		}
		if r.Platform.Valid {
			v := r.Platform.String
			row.Platform = &v
		}
		out = append(out, row)
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
