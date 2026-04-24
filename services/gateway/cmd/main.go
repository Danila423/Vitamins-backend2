package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	analyticsv1 "vitamins-backend_2/gen/go/analytics/v1"
	authv1 "vitamins-backend_2/gen/go/auth/v1"
	vitaminsv1 "vitamins-backend_2/gen/go/vitamins/v1"

	docs "vitamins-backend_2/docs"
	"vitamins-backend_2/pkg/jwt"
	"vitamins-backend_2/pkg/logger"
	"vitamins-backend_2/pkg/metrics"
	"vitamins-backend_2/pkg/rabbitmq"
	"vitamins-backend_2/services/gateway/internal/handler"
	"vitamins-backend_2/services/gateway/internal/middleware"
)

func main() {
	httpPort := envOrDefault("HTTP_PORT", "8080")
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Default().Error("JWT_SECRET is required", "operation", "bootstrap.config")
		os.Exit(1)
	}
	adminToken := os.Getenv("ADMIN_TOKEN")
	authAddr := envOrDefault("AUTH_SERVICE_ADDR", "localhost:50051")
	vitaminsAddr := envOrDefault("VITAMINS_SERVICE_ADDR", "localhost:50052")
	analyticsAddr := envOrDefault("ANALYTICS_SERVICE_ADDR", "localhost:50053")
	accessTTL := parseDuration(os.Getenv("ACCESS_TOKEN_TTL"), 15*time.Minute)
	refreshTTL := parseDuration(os.Getenv("REFRESH_TOKEN_TTL"), 7*24*time.Hour)
	logLevel := envOrDefault("LOG_LEVEL", "info")
	appEnv := envOrDefault("APP_ENV", "production")
	serviceName := envOrDefault("SERVICE_NAME", "gateway")
	serviceVersion := os.Getenv("SERVICE_VERSION")

	appLogger := logger.New(logger.Config{
		Level:          logLevel,
		Environment:    appEnv,
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
	})
	slog.SetDefault(appLogger)

	authConn, err := grpc.NewClient(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		appLogger.Error("failed to connect to auth service", "operation", "bootstrap.grpc_connect", "addr", authAddr, "error", err.Error())
		os.Exit(1)
	}
	defer authConn.Close()

	vitaminsConn, err := grpc.NewClient(vitaminsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		appLogger.Error("failed to connect to vitamins service", "operation", "bootstrap.grpc_connect", "addr", vitaminsAddr, "error", err.Error())
		os.Exit(1)
	}
	defer vitaminsConn.Close()

	analyticsConn, err := grpc.NewClient(analyticsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		appLogger.Error("failed to connect to analytics service", "operation", "bootstrap.grpc_connect", "addr", analyticsAddr, "error", err.Error())
		os.Exit(1)
	}
	defer analyticsConn.Close()

	authClient := authv1.NewAuthServiceClient(authConn)
	vitaminsClient := vitaminsv1.NewVitaminsServiceClient(vitaminsConn)
	analyticsClient := analyticsv1.NewAnalyticsServiceClient(analyticsConn)

	jwtMgr := jwt.NewJWTManager(jwtSecret, accessTTL, refreshTTL)

	authHandler := handler.NewAuthHandler(authClient)
	vitHandler := handler.NewVitaminsHandler(vitaminsClient)
	analyticsHandler := handler.NewAnalyticsHandler(analyticsClient)

	if rmqURL := os.Getenv("RABBITMQ_URL"); rmqURL != "" {
		pub, err := rabbitmq.NewPublisher(rmqURL, appLogger)
		if err != nil {
			appLogger.Error("failed to create rabbitmq publisher",
				"operation", "bootstrap.rabbitmq_publisher",
				"error", err.Error())
			os.Exit(1)
		}
		defer pub.Close()
		analyticsHandler = analyticsHandler.WithEventPublisher(pub)
		appLogger.Info("analytics ingest uses rabbitmq", "operation", "bootstrap.rabbitmq_publisher")
	}

	docs.SwaggerInfo.BasePath = "/api/v1"

	r := gin.New()
	r.Use(logger.RequestIDMiddleware())
	r.Use(metrics.Middleware())
	r.Use(logger.RequestLoggingMiddleware(appLogger))
	r.Use(logger.ErrorLoggingMiddleware(appLogger))
	r.Use(logger.RecoveryMiddleware(appLogger))

	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/swagger/index.html")
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metrics.Registry(), promhttp.HandlerOpts{})))
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		checks := map[string]string{}
		allOK := true
		for name, conn := range map[string]*grpc.ClientConn{"auth": authConn, "vitamins": vitaminsConn, "analytics": analyticsConn} {
			state := conn.GetState().String()
			checks[name] = state
			if state == "TRANSIENT_FAILURE" || state == "SHUTDOWN" {
				allOK = false
			}
		}
		_ = ctx
		if allOK {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "checks": checks})
		} else {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "checks": checks})
		}
	})
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/refresh", authHandler.Refresh)
			authGroup.POST("/password/reset/request", authHandler.RequestPasswordReset)
			authGroup.POST("/password/reset/verify", authHandler.VerifyPasswordResetCode)
			authGroup.POST("/password/reset/confirm", authHandler.ConfirmPasswordReset)
		}

		userGroup := api.Group("/users")
		userGroup.Use(middleware.AuthMiddleware(jwtMgr))
		{
			userGroup.GET("/me", authHandler.GetProfile)
			userGroup.PATCH("/me", authHandler.UpdateProfile)
			userGroup.POST("/me/password/change/request", authHandler.RequestPasswordChange)
			userGroup.POST("/me/password/change/verify", authHandler.VerifyPasswordChangeCode)
			userGroup.POST("/me/password/change/confirm", authHandler.ConfirmPasswordChange)
		}

		vitGroup := api.Group("/vitamins")
		{
			vitGroup.GET("/catalog", vitHandler.ListCatalog)
			vitAuth := vitGroup.Group("")
			vitAuth.Use(middleware.AuthMiddleware(jwtMgr))
			{
				vitAuth.POST("/reminders", vitHandler.CreateReminder)
				vitAuth.GET("/reminders", vitHandler.ListReminders)
				vitAuth.GET("/reminders/:id", vitHandler.GetReminder)
				vitAuth.PATCH("/reminders/:id", vitHandler.UpdateReminder)
				vitAuth.DELETE("/reminders/:id", vitHandler.DeleteReminder)
				vitAuth.POST("/reminders/:id/enable", vitHandler.EnableReminder)
				vitAuth.POST("/reminders/:id/disable", vitHandler.DisableReminder)
			}
		}

		analyticsGroup := api.Group("/analytics")
		{
			ingest := analyticsGroup.Group("")
			ingest.Use(middleware.OptionalAuthMiddleware(jwtMgr))
			ingest.POST("/events", analyticsHandler.IngestEvents)

			analyticsAuth := analyticsGroup.Group("")
			analyticsAuth.Use(middleware.AuthMiddleware(jwtMgr))
			{
				analyticsAuth.POST("/consent", analyticsHandler.SetConsent)
				analyticsAuth.GET("/consent", analyticsHandler.GetConsent)
			}
		}

		adminGroup := api.Group("/admin")
		adminGroup.Use(middleware.AdminTokenMiddleware(adminToken))
		{
			adminGroup.GET("/analytics/export", analyticsHandler.Export)
		}
	}

	srv := &http.Server{
		Addr:    ":" + httpPort,
		Handler: r,
	}

	go func() {
		appLogger.Info("http server starting", "operation", "bootstrap.http_start", "http.port", httpPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("http server stopped", "operation", "bootstrap.http_start", "error", err.Error())
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("shutting down server", "operation", "bootstrap.shutdown")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		appLogger.Error("server forced to shutdown", "operation", "bootstrap.shutdown", "error", err.Error())
	}
	appLogger.Info("server exited", "operation", "bootstrap.shutdown")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}
