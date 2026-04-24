package service

import (
	"context"

	"vitamins-backend_2/pkg/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
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

type SQLCRepository struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewRepository(q *db.Queries, pool *pgxpool.Pool) *SQLCRepository {
	return &SQLCRepository{q: q, pool: pool}
}

func (r *SQLCRepository) GetUserAnalyticsConsent(ctx context.Context, userID int64) (bool, error) {
	return r.q.GetUserAnalyticsConsent(ctx, userID)
}

func (r *SQLCRepository) UpdateUserAnalyticsConsent(ctx context.Context, userID int64, consent bool) error {
	return r.q.UpdateUserAnalyticsConsent(ctx, userID, consent)
}

func (r *SQLCRepository) IngestBatch(ctx context.Context, events []IngestEventRecord) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

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

	br := tx.SendBatch(ctx, batch)
	accepted := 0
	var execErr error
	for range events {
		ct, err := br.Exec()
		if err != nil {
			execErr = err
			break
		}
		if ct.RowsAffected() > 0 {
			accepted++
		}
	}
	if closeErr := br.Close(); execErr == nil && closeErr != nil {
		execErr = closeErr
	}
	if execErr != nil {
		return 0, execErr
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return accepted, nil
}

func (r *SQLCRepository) Export(ctx context.Context, params ExportQueryParams) ([]ExportQueryRow, error) {
	rows, err := r.pool.Query(ctx, listAnalyticsEvents, params.From, params.To, params.EventName, params.UserID, params.Limit, params.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ExportQueryRow, 0)
	for rows.Next() {
		var row ExportQueryRow
		if err := rows.Scan(
			&row.EventID,
			&row.OccurredAt,
			&row.ReceivedAt,
			&row.UserID,
			&row.AnonymousID,
			&row.SessionID,
			&row.EventName,
			&row.Properties,
			&row.RequestID,
			&row.AppVersion,
			&row.Platform,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
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
