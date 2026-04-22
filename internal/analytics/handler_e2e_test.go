//go:build integration

package analytics_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vitamins-backend_2/internal/analytics"
	"vitamins-backend_2/internal/auth"
	"vitamins-backend_2/internal/db"
	"vitamins-backend_2/internal/testutil"

	"github.com/gavv/httpexpect/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestAnalytics_E2E_IngestThenExport spins up the real analytics handler stack
// against the test database and walks an authenticated user through:
//  1. granting consent
//  2. ingesting a single event
//  3. the admin exporting the same event back as JSONL.
//
// It avoids any external infrastructure besides Postgres and uses the same
// admin token middleware as production.
func TestAnalytics_E2E_IngestThenExport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := testutil.NewTestPool(t)
	testutil.ResetTables(t, pool)

	q := db.New(pool)
	svc := analytics.NewService(q, pool)
	jwt := auth.NewJWTManager("e2e-analytics", time.Minute, time.Minute)
	h := analytics.NewHandler(svc)

	r := gin.New()
	api := r.Group("/api/v1")
	ingest := api.Group("")
	ingest.Use(auth.OptionalAuthMiddleware(jwt))
	ingest.POST("/analytics/events", h.IngestEvents)

	authed := api.Group("/analytics")
	authed.Use(auth.AuthMiddleware(jwt))
	authed.POST("/consent", h.SetConsent)

	admin := r.Group("/api/v1/admin")
	admin.Use(auth.AdminTokenMiddleware("admin-secret"))
	admin.GET("/analytics/export", h.Export)

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	ctx := context.Background()
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        "e2e-analytics@test.local",
		PasswordHash: "hash",
	})
	require.NoError(t, err)
	pair, err := jwt.GenerateTokenPair(user.ID)
	require.NoError(t, err)
	bearer := "Bearer " + pair.AccessToken

	e := httpexpect.Default(t, ts.URL)

	e.POST("/api/v1/analytics/consent").
		WithHeader("Authorization", bearer).
		WithJSON(map[string]any{"consent": true}).
		Expect().Status(http.StatusOK).
		JSON().Object().Value("consent").Boolean().IsTrue()

	occurredAt := time.Now().UTC().Format(time.RFC3339)
	const eventID = "11111111-1111-1111-1111-111111111111"
	const sessionID = "22222222-2222-2222-2222-222222222222"
	e.POST("/api/v1/analytics/events").
		WithHeader("Authorization", bearer).
		WithJSON(map[string]any{
			"events": []map[string]any{{
				"event_id":    eventID,
				"occurred_at": occurredAt,
				"event_name":  "auth.login_success",
				"session_id":  sessionID,
				"properties":  map[string]any{"screen": "login"},
			}},
		}).
		Expect().Status(http.StatusOK).
		JSON().Object().Value("accepted").Number().IsEqual(1)

	body := e.GET("/api/v1/admin/analytics/export").
		WithHeader("X-Admin-Token", "admin-secret").
		WithQuery("format", "jsonl").
		Expect().Status(http.StatusOK).
		Body().Raw()

	if !strings.Contains(body, eventID) {
		t.Fatalf("export body missing event id %s, got: %s", eventID, body)
	}

	// JSONL admin export marshals analytics.ExportRow directly, so the field
	// names follow Go's PascalCase. We intentionally don't change that
	// contract — it's the format the existing admin tooling relies on.
	var row map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &row); err != nil {
		t.Fatalf("export jsonl parse: %v (body=%s)", err, body)
	}
	if row["EventName"] != "auth.login_success" {
		t.Fatalf("EventName mismatch in export: %v (full row=%v)", row["EventName"], row)
	}
	if row["EventID"] != eventID {
		t.Fatalf("EventID mismatch in export: %v", row["EventID"])
	}
}
