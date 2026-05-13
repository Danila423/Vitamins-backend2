package service

import (
	"context"
	"errors"
	"time"
)

var ErrEmailConflict = errors.New("EMAIL_CONFLICT")

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	FirstName    string
	LastName     string
}

type UserProfile struct {
	ID        int64
	Email     string
	FirstName string
	LastName  string
}

type UserRepository interface {
	CreateUser(ctx context.Context, email, passwordHash string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByID(ctx context.Context, userID int64) (User, error)
	UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error
	UpdateUserProfile(ctx context.Context, userID int64, email, firstName, lastName string) (User, error)
	DeleteUser(ctx context.Context, userID int64) error
}

type TokenProvider interface {
	GenerateTokenPair(userID int64) (*TokenPair, error)
	Parse(token string) (*Claims, error)
}

type Mailer interface {
	SendOneTimeCode(ctx context.Context, toEmail, subject, code string) error
}

type RedisStore interface {
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) (int64, error)
}
