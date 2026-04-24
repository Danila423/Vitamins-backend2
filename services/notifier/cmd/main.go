package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vitamins-backend_2/pkg/healthcheck"
	"vitamins-backend_2/pkg/logger"
	"vitamins-backend_2/pkg/mailer"
	"vitamins-backend_2/pkg/rabbitmq"
)

// Notifier is a standalone worker that consumes password reset / password
// change events from RabbitMQ and delivers the one-time code over SMTP.
// It exists so that the auth-service can publish asynchronously and does not
// have to wait for (or depend on) the mail provider's availability.
func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	rmqURL := requireEnv("RABBITMQ_URL")
	smtpHost := requireEnv("SMTP_HOST")
	smtpPort := getEnv("SMTP_PORT", "587")
	smtpUser := requireEnv("SMTP_USER")
	smtpPass := requireEnv("SMTP_PASS")
	smtpFrom := requireEnv("SMTP_FROM")
	metricsPort := getEnv("METRICS_PORT", "9100")

	log := logger.New(logger.Config{
		Level:          getEnv("LOG_LEVEL", "info"),
		Environment:    getEnv("APP_ENV", "local"),
		ServiceName:    getEnv("SERVICE_NAME", "notifier"),
		ServiceVersion: getEnv("SERVICE_VERSION", "dev"),
	})
	slog.SetDefault(log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	smtp := mailer.NewSMTPMailer(smtpHost, smtpPort, smtpUser, smtpPass, smtpFrom)

	routingKeys := []string{
		rabbitmq.RoutingKeyPasswordResetRequested,
		rabbitmq.RoutingKeyPasswordChangeRequested,
	}

	consumer, err := rabbitmq.NewConsumerWithBindings(
		rmqURL,
		rabbitmq.QueueNotifications,
		rabbitmq.ExchangeEvents,
		routingKeys,
		log,
	)
	if err != nil {
		return fmt.Errorf("rabbitmq consumer: %w", err)
	}
	defer func() { _ = consumer.Close() }()

	handler := func(ctx context.Context, body []byte) error {
		var evt rabbitmq.PasswordResetEvent
		if err := json.Unmarshal(body, &evt); err != nil {
			log.Error("decode event", "error", err)
			return err
		}
		if evt.Email == "" || evt.Code == "" {
			log.Warn("skip event: empty email or code")
			return nil
		}
		sendCtx, sendCancel := context.WithTimeout(ctx, 30*time.Second)
		defer sendCancel()
		if err := smtp.SendOneTimeCode(sendCtx, evt.Email, evt.Subject, evt.Code); err != nil {
			log.Error("send email failed",
				"error", err,
				"email_masked", logger.MaskEmail(evt.Email),
				"subject", evt.Subject,
			)
			return err
		}
		log.Info("email sent",
			"email_masked", logger.MaskEmail(evt.Email),
			"subject", evt.Subject,
		)
		return nil
	}

	health := healthcheck.New()
	health.AddLiveness("self", func(_ context.Context) error { return nil })
	healthHandler := health.Handler()
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { healthHandler.ServeHTTP(w, r) })
	metricsMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) { healthHandler.ServeHTTP(w, r) })
	metricsSrv := &http.Server{
		Addr:              ":" + metricsPort,
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info("starting health server", "port", metricsPort)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("health server error", "error", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Info("shutdown signal received")
		cancel()
		shutdownCtx, sc := context.WithTimeout(context.Background(), 5*time.Second)
		defer sc()
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

	log.Info("notifier started, consuming queue", "queue", rabbitmq.QueueNotifications)
	if err := consumer.Consume(ctx, handler); err != nil && ctx.Err() == nil {
		return fmt.Errorf("consume: %w", err)
	}
	return nil
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env variable not set", "key", key)
		os.Exit(1)
	}
	return v
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
