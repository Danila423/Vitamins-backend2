package logger

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	headerRequestID = "X-Request-ID"
	ginRequestIDKey = "request_id"
)

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(headerRequestID))
		if requestID == "" {
			requestID = newRequestID()
		}
		c.Set(ginRequestIDKey, requestID)
		c.Writer.Header().Set(headerRequestID, requestID)

		traceID, spanID := parseTraceHeaders(c)
		ctx := WithRequestID(c.Request.Context(), requestID)
		ctx = WithTrace(ctx, traceID, spanID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func RequestLoggingMiddleware(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		ctx := c.Request.Context()
		log := WithContext(base, ctx).With(
			"channel", "app",
			"http.method", c.Request.Method,
			"http.path", c.Request.URL.Path,
			"remote_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
			"bytes_in", c.Request.ContentLength,
		)

		log.InfoContext(ctx, "http.request.started", "operation", "http.request")
		c.Next()

		duration := time.Since(start)
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		status := c.Writer.Status()
		attrs := []any{
			"operation", "http.request",
			"http.route", route,
			"http.status_code", status,
			"bytes_out", c.Writer.Size(),
			"duration_ms", duration.Milliseconds(),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "error", c.Errors.Last().Error())
		}
		switch {
		case status >= http.StatusInternalServerError:
			log.ErrorContext(ctx, "http.request.finished", attrs...)
		case status >= http.StatusBadRequest:
			log.WarnContext(ctx, "http.request.finished", attrs...)
		default:
			log.InfoContext(ctx, "http.request.finished", attrs...)
		}
	}
}

func ErrorLoggingMiddleware(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		ctx := c.Request.Context()
		log := WithContext(base, ctx).With("channel", "app")
		for _, e := range c.Errors {
			log.ErrorContext(ctx, "http.error",
				"operation", "http.error",
				"error", e.Error(),
			)
		}
	}
}

func RecoveryMiddleware(base *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		ctx := c.Request.Context()
		log := WithContext(base, ctx).With("channel", "app")
		log.ErrorContext(ctx, "panic recovered",
			"operation", "http.panic_recovery",
			"error", fmt.Sprint(recovered),
			"http.method", c.Request.Method,
			"http.path", c.Request.URL.Path,
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"code":    "INTERNAL_ERROR",
			"message": "Что-то пошло не так.",
		})
	})
}

func RequestIDFromGin(c *gin.Context) string {
	v, _ := c.Get(ginRequestIDKey)
	requestID, _ := v.(string)
	return strings.TrimSpace(requestID)
}

func parseTraceHeaders(c *gin.Context) (traceID, spanID string) {
	traceParent := strings.TrimSpace(c.GetHeader("traceparent"))
	if traceParent != "" {
		parts := strings.Split(traceParent, "-")
		if len(parts) >= 4 {
			traceID = parts[1]
			spanID = parts[2]
		}
	}
	if traceID == "" {
		traceID = strings.TrimSpace(c.GetHeader("X-B3-TraceId"))
	}
	if spanID == "" {
		spanID = strings.TrimSpace(c.GetHeader("X-B3-SpanId"))
	}
	return traceID, spanID
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

