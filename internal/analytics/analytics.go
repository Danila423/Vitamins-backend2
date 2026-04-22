// Package analytics is a thin facade that preserves the historical public
// API of the analytics feature. The implementation now lives in two
// sub-packages:
//
//   - internal/analytics/service — business logic, JSON DTOs, the repository
//     port and its sqlc adapter.
//   - internal/analytics/handler — HTTP handlers mapped to gin routes plus the
//     ErrorResponse JSON contract.
//
// External callers (cmd/api/main.go, mockery-generated mocks, e2e tests)
// continue to import `internal/analytics` and work against these re-exports so
// the frontend contract stays unchanged.
package analytics

import (
	"vitamins-backend_2/internal/analytics/handler"
	"vitamins-backend_2/internal/analytics/service"
)

// --- service layer re-exports ---

type (
	Service        = service.Service
	ServiceAPI     = service.ServiceAPI
	Repository     = service.Repository
	SQLCRepository = service.SQLCRepository

	BatchRequest    = service.BatchRequest
	EventInput      = service.EventInput
	IngestResponse  = service.IngestResponse
	ConsentRequest  = service.ConsentRequest
	ConsentResponse = service.ConsentResponse
	ExportFilter    = service.ExportFilter
	ExportRow       = service.ExportRow

	IngestEventRecord = service.IngestEventRecord
	ExportQueryParams = service.ExportQueryParams
	ExportQueryRow    = service.ExportQueryRow
)

var (
	NewService         = service.NewService
	NewServiceWithDeps = service.NewServiceWithDeps
	NewRepository      = service.NewRepository

	ErrEmptyBatch         = service.ErrEmptyBatch
	ErrBatchTooLarge      = service.ErrBatchTooLarge
	ErrInvalidEventID     = service.ErrInvalidEventID
	ErrInvalidOccurredAt  = service.ErrInvalidOccurredAt
	ErrInvalidEventName   = service.ErrInvalidEventName
	ErrInvalidSessionID   = service.ErrInvalidSessionID
	ErrInvalidAnonymousID = service.ErrInvalidAnonymousID
	ErrAnonymousRequired  = service.ErrAnonymousRequired
	ErrConsentRequired    = service.ErrConsentRequired
	ErrUserNotFound       = service.ErrUserNotFound
)

// --- handler layer re-exports ---

type (
	Handler       = handler.Handler
	ErrorResponse = handler.ErrorResponse
)

var NewHandler = handler.NewHandler
