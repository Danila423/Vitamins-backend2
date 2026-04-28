package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type Repository interface {
	GetUserAnalyticsConsent(ctx context.Context, userID int64) (bool, error)
	UpdateUserAnalyticsConsent(ctx context.Context, userID int64, consent bool) error
	IngestBatch(ctx context.Context, events []IngestEventRecord) (int, error)
	Export(ctx context.Context, params ExportQueryParams) ([]ExportQueryRow, error)
}

type IngestEventRecord struct {
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

type ExportQueryParams struct {
	From      pgtype.Timestamptz
	To        pgtype.Timestamptz
	EventName pgtype.Text
	UserID    pgtype.Int8
	Limit     int
	Offset    int
}

type ExportQueryRow struct {
	EventID     pgtype.UUID
	OccurredAt  pgtype.Timestamptz
	ReceivedAt  pgtype.Timestamptz
	UserID      pgtype.Int8
	AnonymousID pgtype.UUID
	SessionID   pgtype.UUID
	EventName   string
	Properties  []byte
	RequestID   pgtype.Text
	AppVersion  pgtype.Text
	Platform    pgtype.Text
}
