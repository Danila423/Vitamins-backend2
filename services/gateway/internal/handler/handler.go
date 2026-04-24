package handler

import (
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"

	appLogger "vitamins-backend_2/pkg/logger"
)

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func send(c *gin.Context, code int, k, m string) {
	c.JSON(code, errorResponse{Code: k, Message: m})
}

func logApp(c *gin.Context) *slog.Logger {
	return appLogger.WithContext(slog.Default(), c.Request.Context()).With("channel", "app")
}

func logAudit(c *gin.Context) *slog.Logger {
	return appLogger.WithContext(slog.Default(), c.Request.Context()).With("channel", "audit")
}

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
