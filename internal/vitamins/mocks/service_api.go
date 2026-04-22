package mocks

import (
	"context"

	"vitamins-backend_2/internal/vitamins"

	"github.com/stretchr/testify/mock"
)

type ServiceAPI struct {
	mock.Mock
}

func NewServiceAPI(t interface {
	mock.TestingT
	Cleanup(func())
}) *ServiceAPI {
	m := &ServiceAPI{}
	m.Mock.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

func (m *ServiceAPI) ListCatalog(ctx context.Context) ([]vitamins.CatalogItem, error) {
	args := m.Called(ctx)
	if v, ok := args.Get(0).([]vitamins.CatalogItem); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ServiceAPI) CreateReminder(ctx context.Context, userID int64, req vitamins.CreateReminderRequest) (vitamins.ReminderResponse, error) {
	args := m.Called(ctx, userID, req)
	if v, ok := args.Get(0).(vitamins.ReminderResponse); ok {
		return v, args.Error(1)
	}
	return vitamins.ReminderResponse{}, args.Error(1)
}

func (m *ServiceAPI) ListReminders(ctx context.Context, userID int64) ([]vitamins.ReminderResponse, error) {
	args := m.Called(ctx, userID)
	if v, ok := args.Get(0).([]vitamins.ReminderResponse); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ServiceAPI) GetReminder(ctx context.Context, userID, id int64) (vitamins.ReminderResponse, error) {
	args := m.Called(ctx, userID, id)
	if v, ok := args.Get(0).(vitamins.ReminderResponse); ok {
		return v, args.Error(1)
	}
	return vitamins.ReminderResponse{}, args.Error(1)
}

func (m *ServiceAPI) UpdateReminder(ctx context.Context, userID, id int64, req vitamins.UpdateReminderRequest) (vitamins.ReminderResponse, error) {
	args := m.Called(ctx, userID, id, req)
	if v, ok := args.Get(0).(vitamins.ReminderResponse); ok {
		return v, args.Error(1)
	}
	return vitamins.ReminderResponse{}, args.Error(1)
}

func (m *ServiceAPI) SetReminderActive(ctx context.Context, userID, id int64, active bool) (vitamins.ReminderResponse, error) {
	args := m.Called(ctx, userID, id, active)
	if v, ok := args.Get(0).(vitamins.ReminderResponse); ok {
		return v, args.Error(1)
	}
	return vitamins.ReminderResponse{}, args.Error(1)
}
