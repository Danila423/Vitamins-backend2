package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vitamins-backend_2/internal/auth"
	"vitamins-backend_2/internal/vitamins"
	"vitamins-backend_2/internal/vitamins/mocks"

	"github.com/gavv/httpexpect/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
)

func TestVitaminsHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	newServer := func(svc vitamins.ServiceAPI) (*httptest.Server, *auth.JWTManager) {
		jwt := auth.NewJWTManager("vitamins-handler-secret", time.Minute, time.Minute)
		r := gin.New()
		h := vitamins.NewHandler(svc)

		api := r.Group("/api/v1/vitamins")
		api.GET("/catalog", h.ListCatalog)

		authed := api.Group("")
		authed.Use(auth.AuthMiddleware(jwt))
		authed.POST("/reminders", h.CreateReminder)
		authed.GET("/reminders/:id", h.GetReminder)
		authed.PATCH("/reminders/:id", h.UpdateReminder)
		authed.DELETE("/reminders/:id", h.DeleteReminder)

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

	t.Run("list catalog success", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		ts, _ := newServer(svc)
		t.Cleanup(ts.Close)

		name := "Vitamin D"
		svc.On("ListCatalog", mock.Anything).Return([]vitamins.CatalogItem{{ID: 1, DisplayName: name}}, nil).Once()

		e := httpexpect.Default(t, ts.URL)
		e.GET("/api/v1/vitamins/catalog").
			Expect().
			Status(http.StatusOK).
			JSON().Array().Value(0).Object().HasValue("display_name", name)
	})

	t.Run("list catalog internal error", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		ts, _ := newServer(svc)
		t.Cleanup(ts.Close)

		svc.On("ListCatalog", mock.Anything).Return([]vitamins.CatalogItem(nil), assertAnError()).Once()

		e := httpexpect.Default(t, ts.URL)
		e.GET("/api/v1/vitamins/catalog").
			Expect().
			Status(http.StatusInternalServerError).
			JSON().Object().HasValue("code", "INTERNAL_ERROR")
	})

	t.Run("create reminder unauthorized", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		ts, _ := newServer(svc)
		t.Cleanup(ts.Close)

		e := httpexpect.Default(t, ts.URL)
		e.POST("/api/v1/vitamins/reminders").
			WithJSON(map[string]any{}).
			Expect().
			Status(http.StatusUnauthorized).
			JSON().Object().HasValue("code", "AUTH_REQUIRED")
	})

	t.Run("create reminder maps catalog not found", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		ts, jwt := newServer(svc)
		t.Cleanup(ts.Close)

		access := accessToken(t, jwt, 7)
		name := "Magnesium"
		form := "capsule"
		condition := "after_meal"
		payload := map[string]any{
			"catalog_id": 999,
			"name":       name,
			"form":       form,
			"condition":  condition,
			"course": map[string]any{
				"start_date": "2026-04-01",
				"timezone":   "Europe/Moscow",
			},
			"schedule": map[string]any{
				"days":  []string{"mon"},
				"times": []string{"08:00"},
			},
		}

		svc.On("CreateReminder", mock.Anything, int64(7), mock.AnythingOfType("service.CreateReminderRequest")).
			Return(vitamins.ReminderResponse{}, vitamins.ErrCatalogNotFound).Once()

		e := httpexpect.Default(t, ts.URL)
		e.POST("/api/v1/vitamins/reminders").
			WithHeader("Authorization", "Bearer "+access).
			WithJSON(payload).
			Expect().
			Status(http.StatusNotFound).
			JSON().Object().HasValue("code", "CATALOG_NOT_FOUND")
	})

	t.Run("get reminder invalid id", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		ts, jwt := newServer(svc)
		t.Cleanup(ts.Close)
		access := accessToken(t, jwt, 7)

		e := httpexpect.Default(t, ts.URL)
		e.GET("/api/v1/vitamins/reminders/bad").
			WithHeader("Authorization", "Bearer "+access).
			Expect().
			Status(http.StatusBadRequest).
			JSON().Object().HasValue("code", "INVALID_ID")
	})

	t.Run("update reminder maps no fields", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		ts, jwt := newServer(svc)
		t.Cleanup(ts.Close)
		access := accessToken(t, jwt, 17)

		svc.On("UpdateReminder", mock.Anything, int64(17), int64(1), vitamins.UpdateReminderRequest{}).
			Return(vitamins.ReminderResponse{}, vitamins.ErrNoFieldsToUpdate).Once()

		e := httpexpect.Default(t, ts.URL)
		e.PATCH("/api/v1/vitamins/reminders/1").
			WithHeader("Authorization", "Bearer "+access).
			WithJSON(map[string]any{}).
			Expect().
			Status(http.StatusBadRequest).
			JSON().Object().HasValue("code", "NO_FIELDS_TO_UPDATE")
	})

	t.Run("delete reminder maps not found", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		ts, jwt := newServer(svc)
		t.Cleanup(ts.Close)
		access := accessToken(t, jwt, 17)

		svc.On("SetReminderActive", mock.Anything, int64(17), int64(55), false).
			Return(vitamins.ReminderResponse{}, vitamins.ErrReminderNotFound).Once()

		e := httpexpect.Default(t, ts.URL)
		e.DELETE("/api/v1/vitamins/reminders/55").
			WithHeader("Authorization", "Bearer "+access).
			Expect().
			Status(http.StatusNotFound).
			JSON().Object().HasValue("code", "REMINDER_NOT_FOUND")
	})
}

func assertAnError() error {
	return &tmpError{}
}

type tmpError struct{}

func (e *tmpError) Error() string { return "boom" }
