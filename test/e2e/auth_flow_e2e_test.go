//go:build e2e

package e2e

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gavv/httpexpect/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"vitamins-backend_2/internal/auth"
	"vitamins-backend_2/internal/db"
	"vitamins-backend_2/internal/testutil"
)

func TestAuthFlowE2E_RegisterRefreshGetProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pool := testutil.NewTestPool(t)
	testutil.ResetTables(t, pool)

	q := db.New(pool)
	jwt := auth.NewJWTManager("e2e-secret", 2*time.Minute, 10*time.Minute)
	svc := auth.NewService(q, jwt, nil, nil, auth.PasswordResetConfig{})
	h := auth.NewHandler(svc)

	r := gin.New()
	api := r.Group("/api/v1")
	authGroup := api.Group("/auth")
	authGroup.POST("/register", h.Register)
	authGroup.POST("/refresh", h.Refresh)
	users := api.Group("/users")
	users.Use(auth.AuthMiddleware(jwt))
	users.GET("/me", h.GetProfile)

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	e := httpexpect.Default(t, ts.URL)

	registerResp := e.POST("/api/v1/auth/register").
		WithJSON(map[string]any{
			"email":    "e2e@test.local",
			"password": "Passw0rd1",
		}).
		Expect().
		Status(200).
		JSON().Object()

	refreshToken := registerResp.Value("refreshToken").String().Raw()

	refreshResp := e.POST("/api/v1/auth/refresh").
		WithHeader("Authorization", "Bearer "+refreshToken).
		WithJSON(map[string]any{
			"refresh_token": refreshToken,
		}).
		Expect().
		Status(200).
		JSON().Object()

	access := refreshResp.Value("accessToken").String().Raw()

	profile := e.GET("/api/v1/users/me").
		WithHeader("Authorization", "Bearer "+access).
		Expect().
		Status(200).
		JSON().Object().Raw()

	want := map[string]any{
		"email":     "e2e@test.local",
		"firstName": "",
		"lastName":  "",
	}
	for k, v := range want {
		require.Empty(t, cmp.Diff(v, profile[k]))
	}
}
