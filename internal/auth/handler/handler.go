// Package handler wires HTTP handlers for user authentication, profile
// management and password flows. It translates service-layer errors into
// HTTP status codes + stable error codes consumed by the frontend, so the
// JSON shapes and `code` values here are part of the public API.
//
// The handler implementation is split across several files for readability,
// but they all live in the same package and share the same `Handler` type:
//
//   - handler.go          — types shared by all handlers (errors, utilities).
//   - auth.go             — Register / Login / Refresh.
//   - password_reset.go   — anonymous "forgot password" flow.
//   - profile.go          — /users/me endpoints.
//   - password_change.go  — authenticated password change flow.
package handler

import (
	"log/slog"
	"strings"

	appLogger "vitamins-backend_2/internal/logger"

	"github.com/gin-gonic/gin"

	"vitamins-backend_2/internal/auth/service"
)

// ErrorResponse is the wire format for any non-2xx response. The frontend
// matches on the Code field, so values are part of the public contract.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Handler struct{ s service.ServiceAPI }

func NewHandler(s service.ServiceAPI) *Handler { return &Handler{s} }

func send(c *gin.Context, code int, k, m string) {
	c.JSON(code, ErrorResponse{Code: k, Message: m})
}

func logApp(c *gin.Context) *slog.Logger {
	return appLogger.WithContext(slog.Default(), c.Request.Context()).With("channel", "app")
}

func logAudit(c *gin.Context) *slog.Logger {
	return appLogger.WithContext(slog.Default(), c.Request.Context()).With("channel", "audit")
}

// extractBearerToken returns the raw token from an "Authorization: Bearer <…>"
// header, or empty string when the header is missing/malformed.
func extractBearerToken(authHeader string) string {
	value := strings.TrimSpace(authHeader)
	if value == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(value, prefix))
}
