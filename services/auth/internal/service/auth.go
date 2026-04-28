package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	appLogger "vitamins-backend_2/pkg/logger"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	users  UserRepository
	tokens *JWTManager
	redis  RedisStore
}

func refreshAllowKey(jti string) string {
	return "auth:refresh:jti:" + jti
}

func hash(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func check(h, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(h), []byte(pw)) == nil
}

func (s *AuthService) Register(ctx context.Context, email, pw string) (*TokenPair, error) {
	if err := ValidateEmailPassword(email, pw); err != nil {
		return nil, err
	}

	h, err := hash(pw)
	if err != nil {
		return nil, err
	}

	user, err := s.users.CreateUser(ctx, email, h)
	if err != nil {
		if errors.Is(err, ErrEmailConflict) {
			return nil, ErrEmailAlreadyExists
		}
		return nil, err
	}

	return s.issueTokens(ctx, user.ID)
}

func (s *AuthService) Login(ctx context.Context, email, pw string) (*TokenPair, error) {
	if err := ValidateEmailPassword(email, pw); err != nil {
		return nil, err
	}

	u, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !check(u.PasswordHash, pw) {
		return nil, ErrInvalidCredentials
	}

	return s.issueTokens(ctx, u.ID)
}

func (s *AuthService) Refresh(ctx context.Context, token string) (*TokenPair, error) {
	c, err := s.tokens.ParseWithType(token, TokenTypeRefresh)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if s.redis != nil && c.ID != "" {
		jtiKey := refreshAllowKey(c.ID)
		exists, getErr := s.redis.Get(ctx, jtiKey)
		if getErr != nil || exists == "" {
			return nil, ErrInvalidCredentials
		}
		if _, err := s.redis.Del(ctx, jtiKey); err != nil {
			return nil, ErrInvalidCredentials
		}
	}

	return s.issueTokens(ctx, c.UserID)
}

func (s *AuthService) issueTokens(ctx context.Context, userID int64) (*TokenPair, error) {
	jti, err := newJTI()
	if err != nil {
		return nil, err
	}
	pair, err := s.tokens.GenerateTokenPairWithJTI(userID, jti)
	if err != nil {
		return nil, err
	}
	if s.redis != nil {
		if err := s.redis.Set(ctx, refreshAllowKey(jti), strconv.FormatInt(userID, 10), s.tokens.RefreshTTL()); err != nil {
			appLogger.WithContext(slog.Default(), ctx).With(
				"channel", "app",
				"operation", "auth.refresh.allowlist_write",
			).Warn("refresh allowlist write failed", "error", err.Error())
		}
	}
	return pair, nil
}
