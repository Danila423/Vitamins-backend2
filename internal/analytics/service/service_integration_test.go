//go:build integration

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"vitamins-backend_2/internal/db"
	"vitamins-backend_2/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Ingest_ConsentAndPIISanitization_Integration(t *testing.T) {
	pool := testutil.NewTestPool(t)
	testutil.ResetTables(t, pool)

	q := db.New(pool)
	svc := NewService(q, pool)
	ctx := context.Background()

	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        "analytics@test.local",
		PasswordHash: "hash",
	})
	require.NoError(t, err)

	tokenUserID := user.ID
	_, err = svc.Ingest(ctx, &tokenUserID, BatchRequest{
		Events: []EventInput{
			{
				EventID:    "11111111-1111-1111-1111-111111111111",
				OccurredAt: time.Now().UTC().Format(time.RFC3339),
				EventName:  "auth.login_success",
				SessionID:  "22222222-2222-2222-2222-222222222222",
				Properties: map[string]any{"screen": "login"},
			},
		},
	})
	assert.ErrorIs(t, err, ErrAnonymousRequired)

	require.NoError(t, svc.SetConsent(ctx, user.ID, true))

	resp, err := svc.Ingest(ctx, &tokenUserID, BatchRequest{
		Events: []EventInput{
			{
				EventID:    "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				OccurredAt: time.Now().UTC().Format(time.RFC3339),
				EventName:  "auth.login_success",
				SessionID:  "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
				Properties: map[string]any{
					"email": "private@test.local",
					"note":  "mail private@test.local",
				},
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Accepted)
	assert.Equal(t, 0, resp.Deduplicated)

	exported, err := svc.Export(ctx, ExportFilter{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, exported, 1)
	require.Equal(t, "auth.login_success", exported[0].EventName)
	require.NotNil(t, exported[0].UserID)
	assert.Equal(t, user.ID, *exported[0].UserID)

	var props map[string]any
	require.NoError(t, json.Unmarshal([]byte(exported[0].Properties), &props))
	_, hasEmailKey := props["email"]
	assert.False(t, hasEmailKey)
	assert.Equal(t, "[redacted]", props["note"])
}

func TestService_Ingest_DedupAndExportFilter_Integration(t *testing.T) {
	pool := testutil.NewTestPool(t)
	testutil.ResetTables(t, pool)

	q := db.New(pool)
	svc := NewService(q, pool)
	ctx := context.Background()

	anon := "99999999-9999-9999-9999-999999999999"
	eventID := "aaaaaaaa-1111-2222-3333-bbbbbbbbbbbb"
	occurredAt := time.Now().UTC().Format(time.RFC3339)
	req := BatchRequest{
		Events: []EventInput{
			{
				EventID:     eventID,
				OccurredAt:  occurredAt,
				EventName:   "vitamins.reminder_created",
				SessionID:   "11111111-2222-3333-4444-555555555555",
				AnonymousID: &anon,
				Properties:  map[string]any{"form": "capsule"},
			},
			{
				EventID:     eventID,
				OccurredAt:  occurredAt,
				EventName:   "vitamins.reminder_created",
				SessionID:   "11111111-2222-3333-4444-555555555555",
				AnonymousID: &anon,
			},
		},
	}

	resp, err := svc.Ingest(ctx, nil, req)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Accepted)
	assert.Equal(t, 1, resp.Deduplicated)

	eventName := "vitamins.reminder_created"
	rows, err := svc.Export(ctx, ExportFilter{
		EventName: &eventName,
		Limit:     100,
		Offset:    0,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, eventName, rows[0].EventName)
}
