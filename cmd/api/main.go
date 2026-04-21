// @title           Vitamins API
// @version         1.0
// @description     API для регистрации и авторизации
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @host      localhost:8080
// @BasePath  /api/v1

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	docs "vitamins-backend_2/docs" // <- обязательно
	"vitamins-backend_2/internal/analytics"
	"vitamins-backend_2/internal/auth"
	"vitamins-backend_2/internal/cache"
	"vitamins-backend_2/internal/config"
	"vitamins-backend_2/internal/db"
	"vitamins-backend_2/internal/logger"
	"vitamins-backend_2/internal/mailer"
	"vitamins-backend_2/internal/metrics"
	"vitamins-backend_2/internal/vitamins"
)

func main() {
	cfg := config.Load()
	appLogger := logger.New(logger.Config{
		Level:          cfg.LogLevel,
		Environment:    cfg.Environment,
		ServiceName:    cfg.ServiceName,
		ServiceVersion: cfg.ServiceVersion,
	})
	slog.SetDefault(appLogger)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		appLogger.Error("database connect failed", "operation", "bootstrap.db_connect", "error", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	q := db.New(pool)
	jwt := auth.NewJWTManager(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	var m auth.Mailer
	if cfg.SMTPHost != "" && cfg.SMTPPort != "" && cfg.SMTPUser != "" && cfg.SMTPPass != "" && cfg.SMTPFrom != "" {
		m = mailer.NewSMTPMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom)
	}
	var redisStore auth.RedisStore
	if cfg.RedisAddr != "" {
		client := redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		defer client.Close()
		redisStore = cache.NewRedisStore(client)
	}
	svc := auth.NewService(q, jwt, m, redisStore, auth.PasswordResetConfig{
		CodeTTL:     cfg.ResetCodeTTL,
		SessionTTL:  cfg.ResetSessionTTL,
		MaxAttempts: cfg.ResetMaxAttempts,
		RateLimit:   cfg.ResetRateLimit,
	})
	h := auth.NewHandler(svc)
	vitSvc := vitamins.NewService(q, pool)
	vitHandler := vitamins.NewHandler(vitSvc)
	analyticsSvc := analytics.NewService(q, pool)
	analyticsHandler := analytics.NewHandler(analyticsSvc, jwt, cfg.AdminToken)

	docs.SwaggerInfo.BasePath = "/api/v1"

	r := gin.New()
	r.Use(logger.RequestIDMiddleware())
	r.Use(metrics.Middleware())
	r.Use(logger.RequestLoggingMiddleware(appLogger))
	r.Use(logger.ErrorLoggingMiddleware(appLogger))
	r.Use(logger.RecoveryMiddleware(appLogger))

	// Swagger РОУТ – именно на r, не в /api/v1
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/swagger/index.html")
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metrics.Registry(), promhttp.HandlerOpts{})))

	api := r.Group("/api/v1")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", h.Register)
			authGroup.POST("/login", h.Login)
			authGroup.POST("/refresh", h.Refresh)
			authGroup.POST("/password/reset/request", h.RequestPasswordReset)
			authGroup.POST("/password/reset/verify", h.VerifyPasswordResetCode)
			authGroup.POST("/password/reset/confirm", h.ConfirmPasswordReset)
		}
		userGroup := api.Group("/users")
		userGroup.Use(auth.AuthMiddleware(jwt))
		{
			userGroup.GET("/me", h.GetProfile)
			userGroup.PATCH("/me", h.UpdateProfile)
			userGroup.POST("/me/password/change/request", h.RequestPasswordChange)
			userGroup.POST("/me/password/change/verify", h.VerifyPasswordChangeCode)
			userGroup.POST("/me/password/change/confirm", h.ConfirmPasswordChange)
		}
		vitGroup := api.Group("/vitamins")
		{
			vitGroup.GET("/catalog", vitHandler.ListCatalog)
			vitAuth := vitGroup.Group("")
			vitAuth.Use(auth.AuthMiddleware(jwt))
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
			analyticsGroup.POST("/events", analyticsHandler.IngestEvents)
			analyticsAuth := analyticsGroup.Group("")
			analyticsAuth.Use(auth.AuthMiddleware(jwt))
			{
				analyticsAuth.POST("/consent", analyticsHandler.SetConsent)
				analyticsAuth.GET("/consent", analyticsHandler.GetConsent)
			}
		}
		adminGroup := api.Group("/admin")
		{
			adminGroup.GET("/analytics/export", analyticsHandler.Export)
		}
	}

	appLogger.Info("http server starting", "operation", "bootstrap.http_start", "http.port", cfg.HTTPPort)
	if err := r.Run(":" + cfg.HTTPPort); err != nil {
		appLogger.Error("http server stopped", "operation", "bootstrap.http_start", "error", err.Error())
	}
}
