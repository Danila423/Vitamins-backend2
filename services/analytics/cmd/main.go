package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vitamins-backend_2/pkg/db"
	"vitamins-backend_2/pkg/grpcutil"
	"vitamins-backend_2/pkg/healthcheck"
	"vitamins-backend_2/pkg/logger"
	"vitamins-backend_2/pkg/rabbitmq"
	analyticsgrpc "vitamins-backend_2/services/analytics/internal/grpc"
	"vitamins-backend_2/services/analytics/internal/service"

	analyticsv1 "vitamins-backend_2/gen/go/analytics/v1"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50053"
	}

	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9100"
	}

	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "analytics-service"
	}

	appLogger := logger.New(logger.Config{
		Level:          os.Getenv("LOG_LEVEL"),
		Environment:    os.Getenv("APP_ENV"),
		ServiceName:    serviceName,
		ServiceVersion: os.Getenv("SERVICE_VERSION"),
	})
	slog.SetDefault(appLogger)

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Apply(ctx, pool); err != nil {
		slog.Error("failed to apply migrations", "error", err)
		os.Exit(1)
	}

	q := db.New(pool)
	svc := service.NewService(q, pool)

	metricsRegistry := prometheus.NewRegistry()

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			recovery.UnaryServerInterceptor(),
			grpcutil.MetricsInterceptor(metricsRegistry),
		),
	)
	analyticsv1.RegisterAnalyticsServiceServer(srv, analyticsgrpc.NewServer(svc))

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		slog.Error("failed to listen", "port", grpcPort, "error", err)
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
		slog.Info("metrics server starting", "port", metricsPort)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("metrics server error", "error", err)
		}
	}()

	go func() {
		slog.Info("gRPC server starting", "port", grpcPort)
		if err := srv.Serve(lis); err != nil {
			slog.Error("gRPC server failed", "error", err)
		}
	}()

	var consumer *rabbitmq.Consumer
	if rmqURL := os.Getenv("RABBITMQ_URL"); rmqURL != "" {
		consumer, err = rabbitmq.NewConsumerWithBindings(
			rmqURL,
			rabbitmq.QueueAnalytics,
			rabbitmq.ExchangeEvents,
			[]string{rabbitmq.RoutingKeyAnalyticsEvent},
			appLogger,
		)
		if err != nil {
			slog.Error("failed to create rabbitmq consumer", "error", err)
			os.Exit(1)
		}
		defer consumer.Close()

		handler := func(ctx context.Context, body []byte) error {
			var batch rabbitmq.AnalyticsBatch
			if err := json.Unmarshal(body, &batch); err != nil {
				slog.Error("decode analytics batch", "error", err)
				return err
			}
			events := make([]service.EventInput, 0, len(batch.Events))
			for _, e := range batch.Events {
				events = append(events, service.EventInput{
					EventID:     e.EventID,
					OccurredAt:  e.OccurredAt,
					EventName:   e.EventName,
					SessionID:   e.SessionID,
					UserID:      e.UserID,
					AnonymousID: e.AnonymousID,
					Properties:  e.Properties,
					RequestID:   e.RequestID,
					AppVersion:  e.AppVersion,
					Platform:    e.Platform,
				})
			}
			req := service.BatchRequest{
				BatchID: batch.BatchID,
				SentAt:  batch.SentAt,
				Events:  events,
			}
			resp, err := svc.Ingest(ctx, batch.TokenUserID, req)
			if err != nil {
				slog.Error("ingest from queue failed", "error", err)
				return err
			}
			slog.Info("ingested batch from queue",
				"accepted", resp.Accepted,
				"deduplicated", resp.Deduplicated,
			)
			return nil
		}
		go func() {
			slog.Info("analytics rabbitmq consumer starting", "queue", rabbitmq.QueueAnalytics)
			if err := consumer.Consume(ctx, handler); err != nil && ctx.Err() == nil {
				slog.Error("consumer stopped", "error", err)
			}
		}()
	}

	<-ctx.Done()
	slog.Info("shutting down gracefully")
	srv.GracefulStop()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = metricsSrv.Shutdown(shutdownCtx)
}
