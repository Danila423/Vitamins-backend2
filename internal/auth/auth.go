// Package auth is a thin facade that preserves the historical public API of
// the `auth` module. The actual implementation lives in sub-packages:
//
//   - internal/auth/service    — register/login/refresh, profile and password
//     flows, validation, JWT, repository adapter, ports.
//   - internal/auth/handler    — HTTP handlers mapped to gin routes.
//   - internal/auth/middleware — AuthMiddleware, OptionalAuthMiddleware and
//     AdminTokenMiddleware, plus UserIDFromContext.
//
// Outside callers (cmd/api/main.go, other features, e2e tests, mockery-
// generated mocks) continue to import `internal/auth` and work against the
// re-exports defined here so the frontend contract and mock signatures stay
// unchanged.
package auth

import (
	"vitamins-backend_2/internal/auth/handler"
	"vitamins-backend_2/internal/auth/middleware"
	"vitamins-backend_2/internal/auth/service"
)

// --- service layer re-exports ---

type (
	Service              = service.Service
	AuthService          = service.AuthService
	ProfileService       = service.ProfileService
	PasswordResetService = service.PasswordResetService
	ServiceAPI           = service.ServiceAPI

	JWTManager = service.JWTManager
	TokenPair  = service.TokenPair
	Claims     = service.Claims

	User           = service.User
	UserProfile    = service.UserProfile
	UserRepository = service.UserRepository
	Mailer         = service.Mailer
	RedisStore     = service.RedisStore
	TokenProvider  = service.TokenProvider

	ProfileUpdate       = service.ProfileUpdate
	PasswordResetConfig = service.PasswordResetConfig
)

const (
	TokenTypeAccess  = service.TokenTypeAccess
	TokenTypeRefresh = service.TokenTypeRefresh
)

var (
	// Auth / credentials / generic
	ErrInvalidCredentials = service.ErrInvalidCredentials
	ErrEmailAlreadyExists = service.ErrEmailAlreadyExists
	ErrUserNotFound       = service.ErrUserNotFound
	ErrNoFieldsToUpdate   = service.ErrNoFieldsToUpdate
	ErrEmailConflict      = service.ErrEmailConflict

	// Validation
	ErrEmailRequired        = service.ErrEmailRequired
	ErrPasswordRequired     = service.ErrPasswordRequired
	ErrInvalidEmailFormat   = service.ErrInvalidEmailFormat
	ErrInvalidPasswordRules = service.ErrInvalidPasswordRules
	ErrPasswordMismatch     = service.ErrPasswordMismatch

	// Password reset flow
	ErrResetCodeRequired   = service.ErrResetCodeRequired
	ErrResetCodeInvalid    = service.ErrResetCodeInvalid
	ErrResetCodeExpired    = service.ErrResetCodeExpired
	ErrResetCodeAttempts   = service.ErrResetCodeAttempts
	ErrResetTokenRequired  = service.ErrResetTokenRequired
	ErrResetSessionInvalid = service.ErrResetSessionInvalid
	ErrResetSessionExpired = service.ErrResetSessionExpired

	// Password change flow
	ErrChangeCodeRequired   = service.ErrChangeCodeRequired
	ErrChangeCodeInvalid    = service.ErrChangeCodeInvalid
	ErrChangeCodeAttempts   = service.ErrChangeCodeAttempts
	ErrChangeTokenRequired  = service.ErrChangeTokenRequired
	ErrChangeSessionInvalid = service.ErrChangeSessionInvalid
	ErrChangeSessionExpired = service.ErrChangeSessionExpired

	// Infra
	ErrMailerNotConfigured = service.ErrMailerNotConfigured
	ErrTooManyRequests     = service.ErrTooManyRequests
	ErrRedisNotConfigured  = service.ErrRedisNotConfigured
)

// Service constructors
var (
	NewService              = service.NewService
	NewServiceWithDeps      = service.NewServiceWithDeps
	NewAuthService          = service.NewAuthService
	NewProfileService       = service.NewProfileService
	NewPasswordResetService = service.NewPasswordResetService

	NewJWTManager = service.NewJWTManager

	ValidateEmail         = service.ValidateEmail
	ValidatePassword      = service.ValidatePassword
	ValidateEmailPassword = service.ValidateEmailPassword
)

// --- handler layer re-exports ---

type (
	Handler       = handler.Handler
	ErrorResponse = handler.ErrorResponse

	AuthRequest         = handler.AuthRequest
	RefreshTokenRequest = handler.RefreshTokenRequest

	UpdateProfileRequest = handler.UpdateProfileRequest
	UserProfileResponse  = handler.UserProfileResponse

	PasswordResetRequest        = handler.PasswordResetRequest
	PasswordResetVerifyRequest  = handler.PasswordResetVerifyRequest
	PasswordResetConfirmRequest = handler.PasswordResetConfirmRequest
	PasswordResetVerifyResponse = handler.PasswordResetVerifyResponse

	PasswordChangeVerifyRequest  = handler.PasswordChangeVerifyRequest
	PasswordChangeConfirmRequest = handler.PasswordChangeConfirmRequest
	PasswordChangeVerifyResponse = handler.PasswordChangeVerifyResponse
)

var NewHandler = handler.NewHandler

// --- middleware re-exports ---

var (
	AuthMiddleware         = middleware.AuthMiddleware
	OptionalAuthMiddleware = middleware.OptionalAuthMiddleware
	AdminTokenMiddleware   = middleware.AdminTokenMiddleware
	UserIDFromContext      = middleware.UserIDFromContext
)
