//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gavv/httpexpect/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func gatewayURL(t *testing.T) string {
	t.Helper()
	u := os.Getenv("GATEWAY_URL")
	if u == "" {
		u = "http://localhost:8080"
	}
	return u
}

func requireGateway(t *testing.T, baseURL string) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	_, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("gateway not reachable at %s, skipping e2e: %v", baseURL, err)
	}
}

func TestAuthFlowE2E_RegisterRefreshGetProfile(t *testing.T) {
	baseURL := gatewayURL(t)
	requireGateway(t, baseURL)

	e := httpexpect.Default(t, baseURL)
	email := fmt.Sprintf("e2e-%d@test.local", time.Now().UnixNano())

	registerResp := e.POST("/api/v1/auth/register").
		WithJSON(map[string]any{
			"email":    email,
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
		"email":     email,
		"firstName": "",
		"lastName":  "",
	}
	for k, v := range want {
		require.Empty(t, cmp.Diff(v, profile[k]))
	}
}
