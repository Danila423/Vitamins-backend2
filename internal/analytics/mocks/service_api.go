package mocks

import (
	"context"

	"vitamins-backend_2/internal/analytics"

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

func (m *ServiceAPI) Ingest(ctx context.Context, tokenUserID *int64, req analytics.BatchRequest) (analytics.IngestResponse, error) {
	args := m.Called(ctx, tokenUserID, req)
	if v, ok := args.Get(0).(analytics.IngestResponse); ok {
		return v, args.Error(1)
	}
	return analytics.IngestResponse{}, args.Error(1)
}

func (m *ServiceAPI) SetConsent(ctx context.Context, userID int64, consent bool) error {
	args := m.Called(ctx, userID, consent)
	return args.Error(0)
}

func (m *ServiceAPI) GetConsent(ctx context.Context, userID int64) (bool, error) {
	args := m.Called(ctx, userID)
	if v, ok := args.Get(0).(bool); ok {
		return v, args.Error(1)
	}
	return false, args.Error(1)
}

func (m *ServiceAPI) Export(ctx context.Context, filter analytics.ExportFilter) ([]analytics.ExportRow, error) {
	args := m.Called(ctx, filter)
	if v, ok := args.Get(0).([]analytics.ExportRow); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
