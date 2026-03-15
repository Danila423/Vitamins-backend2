package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort         string
	DatabaseURL      string
	JWTSecret        string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	AdminToken       string
	SMTPHost         string
	SMTPPort         string
	SMTPUser         string
	SMTPPass         string
	SMTPFrom         string
	ResetCodeTTL     time.Duration
	ResetSessionTTL  time.Duration
	ResetMaxAttempts int
	ResetRateLimit   time.Duration
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
}

func Load() *Config {
	get := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		if def == "" {
			log.Fatalf("env %s required", key)
		}
		return def
	}
	getOptional := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}
	getInt := func(key string, def int) int {
		if v := os.Getenv(key); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				log.Fatalf("env %s must be int", key)
			}
			return n
		}
		return def
	}

	return &Config{
		HTTPPort:         get("HTTP_PORT", "8080"),
		DatabaseURL:      get("DATABASE_URL", ""),
		JWTSecret:        get("JWT_SECRET", ""),
		AccessTokenTTL:   15 * time.Minute,
		RefreshTokenTTL:  30 * 24 * time.Hour,
		AdminToken:       getOptional("ADMIN_TOKEN", ""),
		SMTPHost:         getOptional("SMTP_HOST", ""),
		SMTPPort:         getOptional("SMTP_PORT", "587"),
		SMTPUser:         getOptional("SMTP_USER", ""),
		SMTPPass:         getOptional("SMTP_PASS", ""),
		SMTPFrom:         getOptional("SMTP_FROM", ""),
		ResetCodeTTL:     10 * time.Minute,
		ResetSessionTTL:  15 * time.Minute,
		ResetMaxAttempts: 5,
		ResetRateLimit:   time.Minute,
		RedisAddr:        getOptional("REDIS_ADDR", ""),
		RedisPassword:    getOptional("REDIS_PASSWORD", ""),
		RedisDB:          getInt("REDIS_DB", 0),
	}
}
