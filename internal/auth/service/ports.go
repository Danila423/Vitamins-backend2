package service

import (
	"context"
	"errors"
	"time"
)

// ErrEmailConflict is returned by UserRepository when an e-mail uniqueness
// constraint is violated. The service layer maps it to ErrEmailAlreadyExists.
var ErrEmailConflict = errors.New("EMAIL_CONFLICT")

// User is the domain representation of an authentication user. Persistence
// details (sqlc/pgx types) do not leak through this boundary.
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	FirstName    string
	LastName     string
}

// UserProfile is the DTO returned to the HTTP layer for profile endpoints.
type UserProfile struct {
	ID        int64
	Email     string
	FirstName string
	LastName  string
}

// UserRepository is the auth data port.
// Service/use-case depends on this interface instead of sqlc directly.
// Implementations must map pgx.ErrNoRows to ErrUserNotFound and unique
// violations on email to ErrEmailConflict so the service layer stays free of
// driver-specific details.
type UserRepository interface {
	CreateUser(ctx context.Context, email, passwordHash string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByID(ctx context.Context, userID int64) (User, error)
	UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error
	UpdateUserProfile(ctx context.Context, userID int64, email, firstName, lastName string) (User, error)
}

// TokenProvider is the auth token port.
type TokenProvider interface {
	GenerateTokenPair(userID int64) (*TokenPair, error)
	Parse(token string) (*Claims, error)
}

// Mailer delivers one-time numeric codes to users over e-mail (or another
// configured transport). Subject is passed explicitly so different flows
// (password reset/change, future 2FA) can reuse the same method.
type Mailer interface {
	SendOneTimeCode(ctx context.Context, toEmail, subject, code string) error
}

type RedisStore interface {
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) (int64, error)
}
