package service

import (
	"context"
)

// ServiceAPI describes auth operations used by HTTP handlers.
//
//go:generate mockery --name ServiceAPI --dir . --output ../mocks --outpkg mocks --filename service_api.go
type ServiceAPI interface {
	Register(ctx context.Context, email, pw string) (*TokenPair, error)
	Login(ctx context.Context, email, pw string) (*TokenPair, error)
	Refresh(ctx context.Context, token string) (*TokenPair, error)

	RequestPasswordReset(ctx context.Context, email string) error
	VerifyPasswordResetCode(ctx context.Context, email, code string) (string, error)
	ConfirmPasswordReset(ctx context.Context, resetToken, password, passwordConfirm string) error

	UpdateProfile(ctx context.Context, userID int64, in ProfileUpdate) (UserProfile, error)
	GetProfile(ctx context.Context, userID int64) (UserProfile, error)

	RequestPasswordChange(ctx context.Context, userID int64) error
	VerifyPasswordChangeCode(ctx context.Context, userID int64, code string) (string, error)
	ConfirmPasswordChange(ctx context.Context, changeToken, password, passwordConfirm string) error
}
