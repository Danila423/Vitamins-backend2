package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	"vitamins-backend_2/pkg/db"
	"vitamins-backend_2/pkg/grpcutil"
	"vitamins-backend_2/pkg/healthcheck"
	"vitamins-backend_2/pkg/logger"
	vitgrpc "vitamins-backend_2/services/vitamins/internal/grpc"
	"vitamins-backend_2/services/vitamins/internal/service"

	vitaminsv1 "vitamins-backend_2/gen/go/vitamins/v1"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	grpcPort := envOrDefault("GRPC_PORT", "50052")
	metricsPort := envOrDefault("METRICS_PORT", "9100")
	parallelism := envIntOrDefault("VITAMINS_LIST_PARALLELISM", 8)
	logLevel := os.Getenv("LOG_LEVEL")
	appEnv := os.Getenv("APP_ENV")
	serviceName := envOrDefault("SERVICE_NAME", "vitamins-service")
	serviceVersion := os.Getenv("SERVICE_VERSION")

	appLogger := logger.New(logger.Config{
		Level:          logLevel,
		Environment:    appEnv,
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
	})
	slog.SetDefault(appLogger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		appLogger.Error("database connect failed", "error", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Apply(ctx, pool); err != nil {
		appLogger.Error("migrations failed", "error", err.Error())
		os.Exit(1)
	}

	q := db.New(pool)
	svc := service.NewServiceWithConfig(q, pool, service.ServiceConfig{ListParallelism: parallelism})

	metricsRegistry := prometheus.NewRegistry()

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			recovery.UnaryServerInterceptor(),
			grpcutil.MetricsInterceptor(metricsRegistry),
		),
	)
	vitaminsv1.RegisterVitaminsServiceServer(grpcServer, vitgrpc.NewServer(svc))

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		appLogger.Error("failed to listen", "port", grpcPort, "error", err.Error())
		os.Exit(1)
	}

	health := healthcheck.New()
	health.AddLiveness("self", func(_ context.Context) error { return nil })
	health.AddReadiness("postgres", func(ctx context.Context) error { return pool.Ping(ctx) })

	healthHandler := health.Handler()
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{Registry: metricsRegistry}))
	metricsMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { healthHandler.ServeHTTP(w, r) })
	metricsMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) { healthHandler.ServeHTTP(w, r) })
	metricsSrv := &http.Server{
		Addr:              ":" + metricsPort,
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		appLogger.Info("metrics server starting", "port", metricsPort)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("metrics server error", "error", err.Error())
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		appLogger.Info("shutting down gRPC server")
		grpcServer.GracefulStop()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

	appLogger.Info("gRPC server starting", "port", grpcPort)
	if err := grpcServer.Serve(lis); err != nil {
		appLogger.Error("gRPC server stopped", "error", err.Error())
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
