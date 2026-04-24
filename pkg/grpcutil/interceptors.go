package grpcutil

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func LoggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		code := status.Code(err)
		logger.Info("gRPC call",
			slog.String("method", info.FullMethod),
			slog.Duration("duration", duration),
			slog.String("code", code.String()),
		)

		return resp, err
	}
}

func RecoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				err = status.Errorf(codes.Internal, "panic: %v", r)
			}
		}()
		return handler(ctx, req)
	}
}

func MetricsInterceptor(reg *prometheus.Registry) grpc.UnaryServerInterceptor {
	requestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_requests_total",
	}, []string{"method", "code"})

	requestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "grpc_request_duration_seconds",
	}, []string{"method", "code"})

	reg.MustRegister(requestsTotal, requestDuration)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		code := status.Code(err).String()
		requestsTotal.WithLabelValues(info.FullMethod, code).Inc()
		requestDuration.WithLabelValues(info.FullMethod, code).Observe(duration.Seconds())

		return resp, err
	}
}
