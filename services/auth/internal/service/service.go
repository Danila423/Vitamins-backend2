package service

import (
	"errors"
	"time"

	"vitamins-backend_2/pkg/db"
	"vitamins-backend_2/pkg/jwt"
	"vitamins-backend_2/services/auth/internal/validation"
)

type (
	JWTManager = jwt.JWTManager
	TokenPair  = jwt.TokenPair
	Claims     = jwt.Claims
)

const (
	TokenTypeAccess  = jwt.TokenTypeAccess
	TokenTypeRefresh = jwt.TokenTypeRefresh
)

var (
	NewJWTManager = jwt.NewJWTManager

	newJTI = jwt.NewJTI
)

var (
	ErrEmailRequired        = validation.ErrEmailRequired
	ErrPasswordRequired     = validation.ErrPasswordRequired
	ErrInvalidEmailFormat   = validation.ErrInvalidEmailFormat
	ErrInvalidPasswordRules = validation.ErrInvalidPasswordRules
	ErrPasswordMismatch     = validation.ErrPasswordMismatch

	ValidateEmail         = validation.ValidateEmail
	ValidatePassword      = validation.ValidatePassword
	ValidateEmailPassword = validation.ValidateEmailPassword
)

var (
	ErrInvalidCredentials   = errors.New("INVALID_CREDENTIALS")
	ErrEmailAlreadyExists   = errors.New("EMAIL_ALREADY_EXISTS")
	ErrResetCodeRequired    = errors.New("RESET_CODE_REQUIRED")
	ErrResetCodeInvalid     = errors.New("RESET_CODE_INVALID")
	ErrResetCodeExpired     = errors.New("RESET_CODE_EXPIRED")
	ErrResetCodeAttempts    = errors.New("RESET_CODE_TOO_MANY_ATTEMPTS")
	ErrResetTokenRequired   = errors.New("RESET_TOKEN_REQUIRED")
	ErrResetSessionInvalid  = errors.New("RESET_SESSION_INVALID")
	ErrResetSessionExpired  = errors.New("RESET_SESSION_EXPIRED")
	ErrMailerNotConfigured  = errors.New("MAILER_NOT_CONFIGURED")
	ErrTooManyRequests      = errors.New("TOO_MANY_REQUESTS")
	ErrRedisNotConfigured   = errors.New("REDIS_NOT_CONFIGURED")
	ErrUserNotFound         = errors.New("USER_NOT_FOUND")
	ErrNoFieldsToUpdate     = errors.New("NO_FIELDS_TO_UPDATE")
	ErrChangeCodeRequired   = errors.New("CHANGE_CODE_REQUIRED")
	ErrChangeCodeInvalid    = errors.New("CHANGE_CODE_INVALID")
	ErrChangeCodeAttempts   = errors.New("CHANGE_CODE_TOO_MANY_ATTEMPTS")
	ErrChangeTokenRequired  = errors.New("CHANGE_TOKEN_REQUIRED")
	ErrChangeSessionInvalid = errors.New("CHANGE_SESSION_INVALID")
	ErrChangeSessionExpired = errors.New("CHANGE_SESSION_EXPIRED")
)

type PasswordResetConfig struct {
	CodeTTL     time.Duration
	SessionTTL  time.Duration
	MaxAttempts int
	RateLimit   time.Duration
}

func applyResetDefaults(cfg PasswordResetConfig) PasswordResetConfig {
	if cfg.CodeTTL == 0 {
		cfg.CodeTTL = 10 * time.Minute
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 15 * time.Minute
	}
	if cfg.MaxAttempts == 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.RateLimit == 0 {
		cfg.RateLimit = time.Minute
	}
	return cfg
}

type Service struct {
	*AuthService
	*ProfileService
	*PasswordResetService
}

func NewAuthService(users UserRepository, tokens *JWTManager, r RedisStore) *AuthService {
	return &AuthService{users: users, tokens: tokens, redis: r}
}

func NewProfileService(users UserRepository) *ProfileService {
	return &ProfileService{users: users}
}

func NewPasswordResetService(users UserRepository, m Mailer, r RedisStore, cfg PasswordResetConfig) *PasswordResetService {
	return &PasswordResetService{users: users, mailer: m, redis: r, cfg: applyResetDefaults(cfg), now: time.Now}
}

func NewService(q *db.Queries, j *JWTManager, m Mailer, r RedisStore, cfg PasswordResetConfig) *Service {
	return NewServiceWithDeps(newSQLCUserRepository(q), j, m, r, cfg)
}

func NewServiceWithDeps(users UserRepository, tokens *JWTManager, m Mailer, r RedisStore, cfg PasswordResetConfig) *Service {
	return &Service{
		AuthService:          NewAuthService(users, tokens, r),
		ProfileService:       NewProfileService(users),
		PasswordResetService: NewPasswordResetService(users, m, r, cfg),
	}
}
