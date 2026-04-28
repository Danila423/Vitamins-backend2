package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort                string
	DatabaseURL             string
	JWTSecret               string
	LogLevel                string
	Environment             string
	ServiceName             string
	ServiceVersion          string
	AccessTokenTTL          time.Duration
	RefreshTokenTTL         time.Duration
	AdminToken              string
	SMTPHost                string
	SMTPPort                string
	SMTPUser                string
	SMTPPass                string
	SMTPFrom                string
	ResetCodeTTL            time.Duration
	ResetSessionTTL         time.Duration
	ResetMaxAttempts        int
	ResetRateLimit          time.Duration
	RedisAddr               string
	RedisPassword           string
	RedisDB                 int
	VitaminsListParallelism int
}

func Load() (*Config, error) {
	required := func(key string) (string, error) {
		v := os.Getenv(key)
		if v == "" {
			return "", fmt.Errorf("env %s required", key)
		}
		return v, nil
	}
	getOptional := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}
	getInt := func(key string, def int) (int, error) {
		if v := os.Getenv(key); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				return 0, fmt.Errorf("env %s must be int: %w", key, err)
			}
			return n, nil
		}
		return def, nil
	}
	getDuration := func(key string, def time.Duration) (time.Duration, error) {
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

	dbURL, err := required("DATABASE_URL")
	if err != nil {
		return nil, err
	}
	jwtSecret, err := required("JWT_SECRET")
	if err != nil {
		return nil, err
	}
	redisDB, err := getInt("REDIS_DB", 0)
	if err != nil {
		return nil, err
	}
	accessTTL, err := getDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return nil, err
	}
	refreshTTL, err := getDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour)
	if err != nil {
		return nil, err
	}
	resetCodeTTL, err := getDuration("RESET_CODE_TTL", 10*time.Minute)
	if err != nil {
		return nil, err
	}
	resetSessionTTL, err := getDuration("RESET_SESSION_TTL", 15*time.Minute)
	if err != nil {
		return nil, err
	}
	resetRateLimit, err := getDuration("RESET_RATE_LIMIT", time.Minute)
	if err != nil {
		return nil, err
	}
	resetMaxAttempts, err := getInt("RESET_MAX_ATTEMPTS", 5)
	if err != nil {
		return nil, err
	}
	parallelism, err := getInt("VITAMINS_LIST_PARALLELISM", 8)
	if err != nil {
		return nil, err
	}

	return &Config{
		HTTPPort:                getOptional("HTTP_PORT", "8080"),
		DatabaseURL:             dbURL,
		JWTSecret:               jwtSecret,
		LogLevel:                getOptional("LOG_LEVEL", "info"),
		Environment:             getOptional("APP_ENV", "local"),
		ServiceName:             getOptional("SERVICE_NAME", "vitamins-backend"),
		ServiceVersion:          getOptional("SERVICE_VERSION", "dev"),
		AccessTokenTTL:          accessTTL,
		RefreshTokenTTL:         refreshTTL,
		AdminToken:              getOptional("ADMIN_TOKEN", ""),
		SMTPHost:                getOptional("SMTP_HOST", ""),
		SMTPPort:                getOptional("SMTP_PORT", "587"),
		SMTPUser:                getOptional("SMTP_USER", ""),
		SMTPPass:                getOptional("SMTP_PASS", ""),
		SMTPFrom:                getOptional("SMTP_FROM", ""),
		ResetCodeTTL:            resetCodeTTL,
		ResetSessionTTL:         resetSessionTTL,
		ResetMaxAttempts:        resetMaxAttempts,
		ResetRateLimit:          resetRateLimit,
		RedisAddr:               getOptional("REDIS_ADDR", ""),
		RedisPassword:           getOptional("REDIS_PASSWORD", ""),
		RedisDB:                 redisDB,
		VitaminsListParallelism: parallelism,
	}, nil
}

func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}
	return cfg
}
