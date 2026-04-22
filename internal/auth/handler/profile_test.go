package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gavv/httpexpect/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"

	"vitamins-backend_2/internal/auth"
	"vitamins-backend_2/internal/auth/handler"
	"vitamins-backend_2/internal/auth/mocks"
)

func TestProfileHandlers(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	makeServer := func(svc auth.ServiceAPI) (*httptest.Server, *auth.JWTManager) {
		jwt := auth.NewJWTManager("profile-test-secret", time.Minute, time.Minute)
		r := gin.New()
		h := handler.NewHandler(svc)
		g := r.Group("/api/v1/users")
		g.Use(auth.AuthMiddleware(jwt))
		g.GET("/me", h.GetProfile)
		g.PATCH("/me", h.UpdateProfile)
		return httptest.NewServer(r), jwt
	}

	token := func(t *testing.T, jwt *auth.JWTManager, userID int64) string {
		t.Helper()
		pair, err := jwt.GenerateTokenPair(userID)
		if err != nil {
			t.Fatalf("generate token: %v", err)
		}
		return pair.AccessToken
	}

	t.Run("get profile success", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		ts, jwt := makeServer(svc)
		t.Cleanup(ts.Close)

		access := token(t, jwt, 42)
		svc.On("GetProfile", mock.Anything, int64(42)).Return(auth.UserProfile{
			ID:        42,
			Email:     "u@test.local",
			FirstName: "Dan",
			LastName:  "Nest",
		}, nil).Once()

		e := httpexpect.Default(t, ts.URL)
		e.GET("/api/v1/users/me").
			WithHeader("Authorization", "Bearer "+access).
			Expect().
			Status(http.StatusOK).
			JSON().Object().
			HasValue("id", float64(42)).
			HasValue("email", "u@test.local").
			HasValue("firstName", "Dan").
			HasValue("lastName", "Nest")
	})

	t.Run("get profile maps not found", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		ts, jwt := makeServer(svc)
		t.Cleanup(ts.Close)

		access := token(t, jwt, 10)
		svc.On("GetProfile", mock.Anything, int64(10)).Return(auth.UserProfile{}, auth.ErrUserNotFound).Once()

		e := httpexpect.Default(t, ts.URL)
		e.GET("/api/v1/users/me").
			WithHeader("Authorization", "Bearer "+access).
			Expect().
			Status(http.StatusNotFound).
			JSON().Object().
			HasValue("code", "USER_NOT_FOUND")
	})

	t.Run("patch profile success", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		ts, jwt := makeServer(svc)
		t.Cleanup(ts.Close)

		access := token(t, jwt, 77)
		first := "Dan"
		last := "Nest"
		email := "new@test.local"
		svc.On("UpdateProfile", mock.Anything, int64(77), auth.ProfileUpdate{
			FirstName: &first,
			LastName:  &last,
			Email:     &email,
		}).Return(auth.UserProfile{
			ID:        77,
			Email:     email,
			FirstName: first,
			LastName:  last,
		}, nil).Once()

		e := httpexpect.Default(t, ts.URL)
		e.PATCH("/api/v1/users/me").
			WithHeader("Authorization", "Bearer "+access).
			WithJSON(map[string]any{
				"firstName": first,
				"lastName":  last,
				"email":     email,
			}).
			Expect().
			Status(http.StatusOK).
			JSON().Object().
			HasValue("id", float64(77))
	})

	t.Run("patch profile maps no fields error", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		ts, jwt := makeServer(svc)
		t.Cleanup(ts.Close)

		access := token(t, jwt, 55)
		svc.On("UpdateProfile", mock.Anything, int64(55), auth.ProfileUpdate{}).
			Return(auth.UserProfile{}, auth.ErrNoFieldsToUpdate).Once()

		e := httpexpect.Default(t, ts.URL)
		e.PATCH("/api/v1/users/me").
			WithHeader("Authorization", "Bearer "+access).
			WithJSON(map[string]any{}).
			Expect().
			Status(http.StatusBadRequest).
			JSON().Object().
			HasValue("code", "NO_FIELDS_TO_UPDATE")
	})
}
