package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Level          string
	Environment    string
	ServiceName    string
	ServiceVersion string
}

type ctxKey string

const (
	requestIDKey ctxKey = "request_id"
	traceIDKey   ctxKey = "trace_id"
	spanIDKey    ctxKey = "span_id"
)

func New(cfg Config) *slog.Logger {
	level := parseLevel(cfg.Level)
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Key = "timestamp"
			}
			return a
		},
	})
	return slog.New(handler).With(
		"service.name", strings.TrimSpace(cfg.ServiceName),
		"service.version", strings.TrimSpace(cfg.ServiceVersion),
		"env", strings.TrimSpace(cfg.Environment),
	)
}

func parseLevel(v string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	if strings.TrimSpace(requestID) == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, requestID)
}

func WithTrace(ctx context.Context, traceID, spanID string) context.Context {
	if strings.TrimSpace(traceID) != "" {
		ctx = context.WithValue(ctx, traceIDKey, strings.TrimSpace(traceID))
	}
	if strings.TrimSpace(spanID) != "" {
		ctx = context.WithValue(ctx, spanIDKey, strings.TrimSpace(spanID))
	}
	return ctx
}

func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return strings.TrimSpace(v)
}

func TraceFromContext(ctx context.Context) (traceID, spanID string) {
	traceID, _ = ctx.Value(traceIDKey).(string)
	spanID, _ = ctx.Value(spanIDKey).(string)
	return strings.TrimSpace(traceID), strings.TrimSpace(spanID)
}

func WithContext(l *slog.Logger, ctx context.Context) *slog.Logger {
	if l == nil {
		l = slog.Default()
	}
	if v := RequestIDFromContext(ctx); v != "" {
		l = l.With("request_id", v)
	}
	traceID, spanID := TraceFromContext(ctx)
	if traceID != "" {
		l = l.With("trace_id", traceID)
	}
	if spanID != "" {
		l = l.With("span_id", spanID)
	}
	return l
}

func MaskEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	local := parts[0]
	if len(local) <= 2 {
		return local[:1] + "***@" + parts[1]
	}
	return local[:1] + "***" + local[len(local)-1:] + "@" + parts[1]
}
