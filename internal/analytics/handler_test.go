package analytics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vitamins-backend_2/internal/analytics"
	"vitamins-backend_2/internal/analytics/mocks"
	"vitamins-backend_2/internal/auth"

	"github.com/gavv/httpexpect/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
)

func TestAnalyticsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newServer := func(svc analytics.ServiceAPI, adminToken string) (*httptest.Server, *auth.JWTManager) {
		jwt := auth.NewJWTManager("analytics-handler-secret", time.Minute, time.Minute)
		r := gin.New()
		h := analytics.NewHandler(svc)

		api := r.Group("/api/v1")
		ingest := api.Group("")
		ingest.Use(auth.OptionalAuthMiddleware(jwt))
		ingest.POST("/analytics/events", h.IngestEvents)

		authed := api.Group("/analytics")
		authed.Use(auth.AuthMiddleware(jwt))
		authed.POST("/consent", h.SetConsent)
		authed.GET("/consent", h.GetConsent)

		admin := r.Group("/api/v1/admin")
		admin.Use(auth.AdminTokenMiddleware(adminToken))
		admin.GET("/analytics/export", h.Export)
		return httptest.NewServer(r), jwt
	}

	accessToken := func(t *testing.T, jwt *auth.JWTManager, userID int64) string {
		t.Helper()
		pair, err := jwt.GenerateTokenPair(userID)
		if err != nil {
			t.Fatalf("generate token pair: %v", err)
		}
		return pair.AccessToken
	}

	t.Run("ingest bad request", func(t *testing.T) {
		svc := mocks.NewServiceAPI(t)
		ts, _ := newServer(svc, "")
		t.Cleanup(ts.Close)

		e := httpexpect.Default(t, ts.URL)
		e.POST("/api/v1/analytics/events").WithBytes([]byte("{")).Expect().
			Status(http.StatusBadRequest).
			JSON().Object().HasValue("code", "BAD_REQUEST")
	})

	t.Run("ingest invalid bearer", func(t *testing.T) {
		svc := mocks.NewServiceAPI(t)
		ts, _ := newServer(svc, "")
		t.Cleanup(ts.Close)

		e := httpexpect.Default(t, ts.URL)
		e.POST("/api/v1/analytics/events").
			WithHeader("Authorization", "bad token").
			WithJSON(map[string]any{"events": []any{}}).
			Expect().
			Status(http.StatusUnauthorized).
			JSON().Object().HasValue("code", "INVALID_TOKEN")
	})

	t.Run("ingest maps domain error", func(t *testing.T) {
		svc := mocks.NewServiceAPI(t)
		ts, _ := newServer(svc, "")
		t.Cleanup(ts.Close)

		req := analytics.BatchRequest{Events: []analytics.EventInput{{
			EventID:    "bad",
			OccurredAt: "2026-04-22T10:00:00Z",
			EventName:  "app.open",
			SessionID:  "11111111-1111-1111-1111-111111111111",
		}}}
		svc.On("Ingest", mock.Anything, (*int64)(nil), req).
			Return(analytics.IngestResponse{}, analytics.ErrInvalidEventID).Once()

		e := httpexpect.Default(t, ts.URL)
		e.POST("/api/v1/analytics/events").WithJSON(req).Expect().
			Status(http.StatusBadRequest).
			JSON().Object().HasValue("code", "INVALID_EVENT_ID")
	})

	t.Run("set consent unauthorized", func(t *testing.T) {
		svc := mocks.NewServiceAPI(t)
		ts, _ := newServer(svc, "")
		t.Cleanup(ts.Close)

		e := httpexpect.Default(t, ts.URL)
		e.POST("/api/v1/analytics/consent").WithJSON(map[string]any{"consent": true}).Expect().
			Status(http.StatusUnauthorized).
			JSON().Object().HasValue("code", "AUTH_REQUIRED")
	})

	t.Run("get consent success", func(t *testing.T) {
		svc := mocks.NewServiceAPI(t)
		ts, jwt := newServer(svc, "")
		t.Cleanup(ts.Close)
		access := accessToken(t, jwt, 77)

		svc.On("GetConsent", mock.Anything, int64(77)).Return(true, nil).Once()

		e := httpexpect.Default(t, ts.URL)
		e.GET("/api/v1/analytics/consent").
			WithHeader("Authorization", "Bearer "+access).
			Expect().
			Status(http.StatusOK).
			JSON().Object().HasValue("consent", true)
	})

	t.Run("export requires admin token", func(t *testing.T) {
		svc := mocks.NewServiceAPI(t)
		ts, _ := newServer(svc, "secret")
		t.Cleanup(ts.Close)

		e := httpexpect.Default(t, ts.URL)
		e.GET("/api/v1/admin/analytics/export").Expect().
			Status(http.StatusUnauthorized).
			JSON().Object().HasValue("code", "ADMIN_REQUIRED")
	})

	t.Run("export wrong admin token rejected", func(t *testing.T) {
		svc := mocks.NewServiceAPI(t)
		ts, _ := newServer(svc, "secret")
		t.Cleanup(ts.Close)

		e := httpexpect.Default(t, ts.URL)
		e.GET("/api/v1/admin/analytics/export").
			WithHeader("X-Admin-Token", "wrong").
			Expect().
			Status(http.StatusUnauthorized).
			JSON().Object().HasValue("code", "ADMIN_REQUIRED")
	})

	t.Run("export csv success", func(t *testing.T) {
		svc := mocks.NewServiceAPI(t)
		ts, _ := newServer(svc, "secret")
		t.Cleanup(ts.Close)

		uid := int64(10)
		rows := []analytics.ExportRow{{
			EventID:    "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			OccurredAt: "2026-04-22T10:00:00Z",
			ReceivedAt: "2026-04-22T10:00:01Z",
			UserID:     &uid,
			SessionID:  "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
			EventName:  "auth.login_success",
			Properties: `{"screen":"login"}`,
		}}
		svc.On("Export", mock.Anything, mock.AnythingOfType("service.ExportFilter")).Return(rows, nil).Once()

		e := httpexpect.Default(t, ts.URL)
		req := e.GET("/api/v1/admin/analytics/export").
			WithHeader("X-Admin-Token", "secret").
			WithQuery("format", "csv").
			Expect().
			Status(http.StatusOK)

		req.Header("Content-Type").Contains("text/csv")
		req.Body().Contains("event_id")
		req.Body().Contains("auth.login_success")
	})

	t.Run("export jsonl success", func(t *testing.T) {
		svc := mocks.NewServiceAPI(t)
		ts, _ := newServer(svc, "secret")
		t.Cleanup(ts.Close)

		uid := int64(11)
		rows := []analytics.ExportRow{{
			EventID:    "cccccccc-cccc-cccc-cccc-cccccccccccc",
			OccurredAt: "2026-04-22T11:00:00Z",
			ReceivedAt: "2026-04-22T11:00:01Z",
			UserID:     &uid,
			SessionID:  "dddddddd-dddd-dddd-dddd-dddddddddddd",
			EventName:  "app.open",
			Properties: `{"screen":"home"}`,
		}}
		svc.On("Export", mock.Anything, mock.AnythingOfType("service.ExportFilter")).Return(rows, nil).Once()

		e := httpexpect.Default(t, ts.URL)
		req := e.GET("/api/v1/admin/analytics/export").
			WithHeader("X-Admin-Token", "secret").
			WithQuery("format", "jsonl").
			Expect().
			Status(http.StatusOK)

		req.Header("Content-Type").Contains("application/x-ndjson")
		req.Body().Contains("app.open")
		req.Body().Contains("cccccccc-cccc-cccc-cccc-cccccccccccc")
	})
}
