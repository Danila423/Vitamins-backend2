package service

import "context"

//go:generate mockery --name ServiceAPI --dir . --output ../mocks --outpkg mocks --filename service_api.go
type ServiceAPI interface {
	Ingest(ctx context.Context, tokenUserID *int64, req BatchRequest) (IngestResponse, error)
	SetConsent(ctx context.Context, userID int64, consent bool) error
	GetConsent(ctx context.Context, userID int64) (bool, error)
	Export(ctx context.Context, filter ExportFilter) ([]ExportRow, error)
}
