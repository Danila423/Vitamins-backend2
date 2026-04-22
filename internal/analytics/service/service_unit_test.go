package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Ingest_ValidationAndConsent(t *testing.T) {
	t.Parallel()

	t.Run("empty batch", func(t *testing.T) {
		t.Parallel()
		svc := NewServiceWithDeps(&stubRepo{})
		_, err := svc.Ingest(context.Background(), nil, BatchRequest{})
		require.ErrorIs(t, err, ErrEmptyBatch)
	})

	t.Run("invalid event id", func(t *testing.T) {
		t.Parallel()
		svc := NewServiceWithDeps(&stubRepo{})
		_, err := svc.Ingest(context.Background(), nil, BatchRequest{Events: []EventInput{{
			EventID:    "bad",
			OccurredAt: time.Now().UTC().Format(time.RFC3339),
			EventName:  "app.open",
			SessionID:  "22222222-2222-2222-2222-222222222222",
		}}})
		require.ErrorIs(t, err, ErrInvalidEventID)
	})

	t.Run("no consent requires anonymous", func(t *testing.T) {
		t.Parallel()
		repo := &stubRepo{getConsentFn: func(ctx context.Context, userID int64) (bool, error) { return false, nil }}
		svc := NewServiceWithDeps(repo)
		uid := int64(10)
		_, err := svc.Ingest(context.Background(), &uid, BatchRequest{Events: []EventInput{{
			EventID:    "11111111-1111-1111-1111-111111111111",
			OccurredAt: time.Now().UTC().Format(time.RFC3339),
			EventName:  "app.open",
			SessionID:  "22222222-2222-2222-2222-222222222222",
		}}})
		require.ErrorIs(t, err, ErrAnonymousRequired)
	})

	t.Run("consent true stores user id", func(t *testing.T) {
		t.Parallel()
		repo := &stubRepo{
			getConsentFn: func(ctx context.Context, userID int64) (bool, error) { return true, nil },
			ingestBatchFn: func(ctx context.Context, events []IngestEventRecord) (int, error) {
				require.Len(t, events, 1)
				assert.True(t, events[0].UserID.Valid)
				assert.Equal(t, int64(5), events[0].UserID.Int64)
				return 1, nil
			},
		}
		svc := NewServiceWithDeps(repo)
		uid := int64(5)
		resp, err := svc.Ingest(context.Background(), &uid, BatchRequest{Events: []EventInput{{
			EventID:    "11111111-1111-1111-1111-111111111111",
			OccurredAt: time.Now().UTC().Format(time.RFC3339),
			EventName:  "auth.login_success",
			SessionID:  "22222222-2222-2222-2222-222222222222",
			Properties: map[string]any{"email": "private@test.local"},
		}}})
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Accepted)
		assert.Equal(t, 0, resp.Deduplicated)
	})
}

func TestService_Export_MapsRows(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		exportFn: func(ctx context.Context, params ExportQueryParams) ([]ExportQueryRow, error) {
			return []ExportQueryRow{{
				EventID:     mustUUID(t, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
				OccurredAt:  pgtype.Timestamptz{Time: time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC), Valid: true},
				ReceivedAt:  pgtype.Timestamptz{Time: time.Date(2026, 4, 22, 10, 0, 1, 0, time.UTC), Valid: true},
				UserID:      pgtype.Int8{Int64: 7, Valid: true},
				AnonymousID: pgtype.UUID{},
				SessionID:   mustUUID(t, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
				EventName:   "app.open",
				Properties:  []byte(`{"screen":"home"}`),
			}}, nil
		},
	}
	service := NewServiceWithDeps(repo)

	rows, err := service.Export(context.Background(), ExportFilter{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "app.open", rows[0].EventName)
	require.NotNil(t, rows[0].UserID)
	assert.Equal(t, int64(7), *rows[0].UserID)
}

func mustUUID(t *testing.T, raw string) pgtype.UUID {
	t.Helper()
	u, err := parseUUID(raw)
	require.NoError(t, err)
	return u
}

type stubRepo struct {
	getConsentFn  func(ctx context.Context, userID int64) (bool, error)
	setConsentFn  func(ctx context.Context, userID int64, consent bool) error
	ingestBatchFn func(ctx context.Context, events []IngestEventRecord) (int, error)
	exportFn      func(ctx context.Context, params ExportQueryParams) ([]ExportQueryRow, error)
}

func (s *stubRepo) GetUserAnalyticsConsent(ctx context.Context, userID int64) (bool, error) {
	if s.getConsentFn != nil {
		return s.getConsentFn(ctx, userID)
	}
	return false, nil
}

func (s *stubRepo) UpdateUserAnalyticsConsent(ctx context.Context, userID int64, consent bool) error {
	if s.setConsentFn != nil {
		return s.setConsentFn(ctx, userID, consent)
	}
	return nil
}

func (s *stubRepo) IngestBatch(ctx context.Context, events []IngestEventRecord) (int, error) {
	if s.ingestBatchFn != nil {
		return s.ingestBatchFn(ctx, events)
	}
	return 0, errors.New("unexpected call IngestBatch")
}

func (s *stubRepo) Export(ctx context.Context, params ExportQueryParams) ([]ExportQueryRow, error) {
	if s.exportFn != nil {
		return s.exportFn(ctx, params)
	}
	return nil, errors.New("unexpected call Export")
}
