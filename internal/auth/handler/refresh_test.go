package handler_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gavv/httpexpect/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"vitamins-backend_2/internal/auth"
	"vitamins-backend_2/internal/auth/handler"
	"vitamins-backend_2/internal/auth/mocks"
)

func TestRefreshHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	makeServer := func(svc auth.ServiceAPI) *httptest.Server {
		r := gin.New()
		h := handler.NewHandler(svc)
		r.POST("/api/v1/auth/refresh", h.Refresh)
		return httptest.NewServer(r)
	}

	t.Run("uses bearer token with priority over body", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		svc.On("Refresh", mock.Anything, "from-header").Return(&auth.TokenPair{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
		}, nil).Once()

		ts := makeServer(svc)
		t.Cleanup(ts.Close)

		e := httpexpect.Default(t, ts.URL)
		e.POST("/api/v1/auth/refresh").
			WithHeader("Authorization", "Bearer from-header").
			WithJSON(map[string]any{
				"refreshToken":  "from-body",
				"refresh_token": "from-body-snake",
			}).
			Expect().
			Status(http.StatusOK).
			JSON().Object().
			HasValue("accessToken", "new-access").
			HasValue("refreshToken", "new-refresh")
	})

	t.Run("accepts snake case token in body", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		svc.On("Refresh", mock.Anything, "snake-token").Return(&auth.TokenPair{
			AccessToken:  "a2",
			RefreshToken: "r2",
		}, nil).Once()

		ts := makeServer(svc)
		t.Cleanup(ts.Close)

		e := httpexpect.Default(t, ts.URL)
		e.POST("/api/v1/auth/refresh").
			WithJSON(map[string]any{"refresh_token": "snake-token"}).
			Expect().
			Status(http.StatusOK).
			JSON().Object().
			HasValue("accessToken", "a2")
	})

	t.Run("returns 400 on invalid json", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		ts := makeServer(svc)
		t.Cleanup(ts.Close)

		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/refresh", strings.NewReader("{"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		_, _ = io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 401 for invalid refresh token", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		svc.On("Refresh", mock.Anything, "bad").Return((*auth.TokenPair)(nil), auth.ErrInvalidCredentials).Once()

		ts := makeServer(svc)
		t.Cleanup(ts.Close)

		e := httpexpect.Default(t, ts.URL)
		e.POST("/api/v1/auth/refresh").
			WithJSON(map[string]any{"refreshToken": "bad"}).
			Expect().
			Status(http.StatusUnauthorized).
			JSON().Object().
			HasValue("code", "INVALID_REFRESH_TOKEN")
	})

	t.Run("returns 500 for unexpected service error", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewServiceAPI(t)
		svc.On("Refresh", mock.Anything, "oops").Return((*auth.TokenPair)(nil), errors.New("boom")).Once()

		ts := makeServer(svc)
		t.Cleanup(ts.Close)

		e := httpexpect.Default(t, ts.URL)
		e.POST("/api/v1/auth/refresh").
			WithJSON(map[string]any{"refreshToken": "oops"}).
			Expect().
			Status(http.StatusInternalServerError)
	})
}
