// Package vitamins is a thin facade that preserves the historical public API
// of the vitamins feature. The implementation now lives in two sub-packages:
//
//   - internal/vitamins/service — business logic, JSON DTOs, ports and sqlc
//     repository adapter.
//   - internal/vitamins/handler — HTTP handlers mapped to gin routes.
//
// External callers (cmd/api/main.go, mockery-generated mocks, e2e tests)
// continue to import `internal/vitamins` and work against these re-exports so
// the frontend contract stays unchanged.
package vitamins

import (
	"vitamins-backend_2/internal/vitamins/handler"
	"vitamins-backend_2/internal/vitamins/service"
)

// --- service layer re-exports ---

type (
	Service        = service.Service
	ServiceConfig  = service.ServiceConfig
	ServiceAPI     = service.ServiceAPI
	SQLCRepository = service.SQLCRepository

	ReminderRepository = service.ReminderRepository
	TxManager          = service.TxManager

	CreateReminderRequest        = service.CreateReminderRequest
	UpdateReminderRequest        = service.UpdateReminderRequest
	CourseInput                  = service.CourseInput
	ScheduleInput                = service.ScheduleInput
	NotificationPreferencesInput = service.NotificationPreferencesInput
	ContentOverridesInput        = service.ContentOverridesInput

	CatalogItem                     = service.CatalogItem
	CourseResponse                  = service.CourseResponse
	ScheduleResponse                = service.ScheduleResponse
	NotificationPreferencesResponse = service.NotificationPreferencesResponse
	ContentOverridesResponse        = service.ContentOverridesResponse
	ReminderResponse                = service.ReminderResponse
)

var (
	NewService           = service.NewService
	NewServiceWithConfig = service.NewServiceWithConfig
	NewServiceWithDeps   = service.NewServiceWithDeps
	NewRepository        = service.NewRepository

	ErrCatalogNotFound       = service.ErrCatalogNotFound
	ErrNameRequired          = service.ErrNameRequired
	ErrReminderNotFound      = service.ErrReminderNotFound
	ErrNoFieldsToUpdate      = service.ErrNoFieldsToUpdate
	ErrTimezoneRequired      = service.ErrTimezoneRequired
	ErrInvalidCondition      = service.ErrInvalidCondition
	ErrInvalidForm           = service.ErrInvalidForm
	ErrInvalidDays           = service.ErrInvalidDays
	ErrInvalidTimes          = service.ErrInvalidTimes
	ErrStartDateRequired     = service.ErrStartDateRequired
	ErrInvalidDate           = service.ErrInvalidDate
	ErrInvalidCourseDuration = service.ErrInvalidCourseDuration
)

// --- handler layer re-exports ---

type (
	Handler       = handler.Handler
	ErrorResponse = handler.ErrorResponse
)

var NewHandler = handler.NewHandler
