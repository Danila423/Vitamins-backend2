package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "vitamins-backend_2/gen/go/auth/v1"
	"vitamins-backend_2/pkg/cache"
	"vitamins-backend_2/pkg/db"
	"vitamins-backend_2/pkg/grpcutil"
	"vitamins-backend_2/pkg/healthcheck"
	"vitamins-backend_2/pkg/logger"
	"vitamins-backend_2/pkg/mailer"
	"vitamins-backend_2/pkg/rabbitmq"
	authgrpc "vitamins-backend_2/services/auth/internal/grpc"
	authmailer "vitamins-backend_2/services/auth/internal/mailer"
	"vitamins-backend_2/services/auth/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	dbURL := requireEnv("DATABASE_URL")
	jwtSecret := requireEnv("JWT_SECRET")
	grpcPort := getEnv("GRPC_PORT", "50051")
	metricsPort := getEnv("METRICS_PORT", "9100")

	log := logger.New(logger.Config{
		Level:          getEnv("LOG_LEVEL", "info"),
		Environment:    getEnv("APP_ENV", "local"),
		ServiceName:    getEnv("SERVICE_NAME", "auth-service"),
		ServiceVersion: getEnv("SERVICE_VERSION", "dev"),
	})
	slog.SetDefault(log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()

	if err := db.Apply(ctx, pool); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	q := db.New(pool)

	accessTTL, err := getDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return err
	}
	refreshTTL, err := getDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour)
	if err != nil {
		return err
	}
	jwtMgr := service.NewJWTManager(jwtSecret, accessTTL, refreshTTL)

	var redisStore service.RedisStore
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		redisDB, err := getInt("REDIS_DB", 0)
		if err != nil {
			return err
		}
		client := redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       redisDB,
		})
		redisStore = cache.NewRedisStore(client)
		log.Info("redis connected", "addr", addr)
	}

	var m service.Mailer
	var rmqPublisher *rabbitmq.Publisher
	if rmqURL := os.Getenv("RABBITMQ_URL"); rmqURL != "" {
		pub, err := rabbitmq.NewPublisher(rmqURL, log)
		if err != nil {
			return fmt.Errorf("rabbitmq publisher: %w", err)
		}
		rmqPublisher = pub
		defer func() { _ = pub.Close() }()
		m = authmailer.NewPublisherMailer(pub, log)
		log.Info("rabbitmq publisher mailer configured", "url_masked", maskAMQP(rmqURL))
	} else if host := os.Getenv("SMTP_HOST"); host != "" {
		m = mailer.NewSMTPMailer(
			host,
			getEnv("SMTP_PORT", "587"),
			os.Getenv("SMTP_USER"),
			os.Getenv("SMTP_PASS"),
			os.Getenv("SMTP_FROM"),
		)
		log.Info("smtp mailer configured (fallback, no RABBITMQ_URL)", "host", host)
	}
	_ = rmqPublisher

	resetCodeTTL, err := getDuration("RESET_CODE_TTL", 10*time.Minute)
	if err != nil {
		return err
	}
	resetSessionTTL, err := getDuration("RESET_SESSION_TTL", 15*time.Minute)
	if err != nil {
		return err
	}
	resetMaxAttempts, err := getInt("RESET_MAX_ATTEMPTS", 5)
	if err != nil {
		return err
	}
	resetRateLimit, err := getDuration("RESET_RATE_LIMIT", time.Minute)
	if err != nil {
		return err
	}

	svc := service.NewService(q, jwtMgr, m, redisStore, service.PasswordResetConfig{
		CodeTTL:     resetCodeTTL,
		SessionTTL:  resetSessionTTL,
		MaxAttempts: resetMaxAttempts,
		RateLimit:   resetRateLimit,
	})

	logOpts := []logging.Option{
		logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
	}
	recoveryOpt := recovery.WithRecoveryHandlerContext(func(ctx context.Context, p any) error {
		slog.ErrorContext(ctx, "grpc panic recovered", "panic", p)
		return status.Errorf(codes.Internal, "internal error")
	})

	metricsRegistry := prometheus.NewRegistry()

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			logging.UnaryServerInterceptor(interceptorLogger(log), logOpts...),
			recovery.UnaryServerInterceptor(recoveryOpt),
			grpcutil.MetricsInterceptor(metricsRegistry),
		),
	)
	authv1.RegisterAuthServiceServer(srv, authgrpc.NewServer(svc))

	lc := net.ListenConfig{}
	lis, err := lc.Listen(ctx, "tcp", ":"+grpcPort)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	health := healthcheck.New()
	health.AddLiveness("self", func(_ context.Context) error { return nil })
	health.AddReadiness("postgres", func(ctx context.Context) error { return pool.Ping(ctx) })
	if redisStore != nil {
		health.AddReadiness("redis", func(ctx context.Context) error {
			if addr := os.Getenv("REDIS_ADDR"); addr != "" {
				client := redis.NewClient(&redis.Options{Addr: addr, Password: os.Getenv("REDIS_PASSWORD")})
				defer func() { _ = client.Close() }()
				return client.Ping(ctx).Err()
			}
			return nil
		})
	}

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
		log.Info("starting metrics server", "port", metricsPort)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("metrics server error", "error", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Info("shutting down grpc server")
		srv.GracefulStop()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

	log.Info("starting grpc server", "port", grpcPort)
	return srv.Serve(lis)
}

func interceptorLogger(l *slog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		l.Log(ctx, slog.Level(lvl), msg, fields...)
	})
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

func getDuration(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("env %s must be a duration (e.g. 15m, 24h): %w", key, err)
	}
	return d, nil
}

func maskAMQP(url string) string {
	// amqp://user:pass@host:port → amqp://***@host:port
	at := -1
	for i := len(url) - 1; i >= 0; i-- {
		if url[i] == '@' {
			at = i
			break
		}
	}
	schemeEnd := 0
	for i := 0; i < len(url)-2; i++ {
		if url[i] == ':' && url[i+1] == '/' && url[i+2] == '/' {
			schemeEnd = i + 3
			break
		}
	}
	if at > schemeEnd {
		return url[:schemeEnd] + "***" + url[at:]
	}
	return url
}

func getInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("env %s must be int: %w", key, err)
	}
	return n, nil
}
