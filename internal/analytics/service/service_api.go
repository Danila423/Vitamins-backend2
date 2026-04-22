package service

import "context"

// ServiceAPI describes analytics operations used by handlers.
//
//go:generate mockery --name ServiceAPI --dir . --output ../mocks --outpkg mocks --filename service_api.go
type ServiceAPI interface { //nolint:revive // renaming would break the public API
	Ingest(ctx context.Context, tokenUserID *int64, req BatchRequest) (IngestResponse, error)
	SetConsent(ctx context.Context, userID int64, consent bool) error
	GetConsent(ctx context.Context, userID int64) (bool, error)
	Export(ctx context.Context, filter ExportFilter) ([]ExportRow, error)
}
