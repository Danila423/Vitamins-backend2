package service

import "context"

//go:generate mockery --name ServiceAPI --dir . --output ../mocks --outpkg mocks --filename service_api.go
type ServiceAPI interface {
	ListCatalog(ctx context.Context) ([]CatalogItem, error)
	CreateReminder(ctx context.Context, userID int64, req CreateReminderRequest) (ReminderResponse, error)
	ListReminders(ctx context.Context, userID int64) ([]ReminderResponse, error)
	GetReminder(ctx context.Context, userID, id int64) (ReminderResponse, error)
	UpdateReminder(ctx context.Context, userID, id int64, req UpdateReminderRequest) (ReminderResponse, error)
	SetReminderActive(ctx context.Context, userID, id int64, active bool) (ReminderResponse, error)
}
