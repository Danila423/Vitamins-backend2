//go:build integration

package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vitamins-backend_2/internal/auth"
	"vitamins-backend_2/internal/db"
	"vitamins-backend_2/internal/testutil"
	"vitamins-backend_2/internal/vitamins"

	"github.com/gavv/httpexpect/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestVitaminsHandler_E2E_FullReminderLifecycle wires the real HTTP handler,
// service and sqlc repository against a real test database, then exercises the
// full reminder lifecycle (create -> read -> disable -> delete) over HTTP. It's
// intentionally minimal but uses no mocks so it catches regressions in any of
// the layers.
func TestVitaminsHandler_E2E_FullReminderLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := testutil.NewTestPool(t)
	testutil.ResetTables(t, pool)

	q := db.New(pool)
	svc := vitamins.NewService(q, pool)
	jwt := auth.NewJWTManager("e2e-secret", time.Minute, time.Minute)
	h := vitamins.NewHandler(svc)

	r := gin.New()
	api := r.Group("/api/v1/vitamins")
	api.GET("/catalog", h.ListCatalog)
	authed := api.Group("")
	authed.Use(auth.AuthMiddleware(jwt))
	authed.POST("/reminders", h.CreateReminder)
	authed.GET("/reminders", h.ListReminders)
	authed.GET("/reminders/:id", h.GetReminder)
	authed.PATCH("/reminders/:id", h.UpdateReminder)
	authed.POST("/reminders/:id/enable", h.EnableReminder)
	authed.POST("/reminders/:id/disable", h.DisableReminder)
	authed.DELETE("/reminders/:id", h.DeleteReminder)

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	ctx := context.Background()
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        "e2e-user@test.local",
		PasswordHash: "hash",
	})
	require.NoError(t, err)
	pair, err := jwt.GenerateTokenPair(user.ID)
	require.NoError(t, err)
	bearer := "Bearer " + pair.AccessToken

	e := httpexpect.Default(t, ts.URL)

	createPayload := map[string]any{
		"name":      "Vitamin C",
		"form":      "tablet",
		"condition": "after_meal",
		"dose":      "500",
		"course": map[string]any{
			"start_date":    "2026-04-01",
			"duration_days": 10,
			"timezone":      "Europe/Moscow",
		},
		"schedule": map[string]any{
			"days":  []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"},
			"times": []string{"09:00"},
		},
	}
	created := e.POST("/api/v1/vitamins/reminders").
		WithHeader("Authorization", bearer).
		WithJSON(createPayload).
		Expect().
		Status(http.StatusOK).
		JSON().Object()
	created.Value("name").String().IsEqual("Vitamin C")
	created.Value("is_active").Boolean().IsTrue()
	id := created.Value("id").Number().Raw()

	e.GET("/api/v1/vitamins/reminders").
		WithHeader("Authorization", bearer).
		Expect().Status(http.StatusOK).
		JSON().Array().Length().IsEqual(1)

	e.GET("/api/v1/vitamins/reminders/"+numToStr(id)).
		WithHeader("Authorization", bearer).
		Expect().Status(http.StatusOK).
		JSON().Object().Value("name").String().IsEqual("Vitamin C")

	e.POST("/api/v1/vitamins/reminders/"+numToStr(id)+"/disable").
		WithHeader("Authorization", bearer).
		Expect().Status(http.StatusOK).
		JSON().Object().Value("is_active").Boolean().IsFalse()

	e.POST("/api/v1/vitamins/reminders/"+numToStr(id)+"/enable").
		WithHeader("Authorization", bearer).
		Expect().Status(http.StatusOK).
		JSON().Object().Value("is_active").Boolean().IsTrue()

	// Delete is implemented as a soft-delete (is_active=false); the resource
	// is still visible afterwards. We verify that contract instead of inventing
	// a new one — the frontend already relies on this behavior.
	e.DELETE("/api/v1/vitamins/reminders/"+numToStr(id)).
		WithHeader("Authorization", bearer).
		Expect().Status(http.StatusOK).
		JSON().Object().Value("is_active").Boolean().IsFalse()
}

func numToStr(v float64) string {
	n := int64(v)
	if float64(n) != v {
		return ""
	}
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
