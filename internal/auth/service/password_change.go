package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

func (s *PasswordResetService) RequestPasswordChange(ctx context.Context, userID int64) error {
	if s.mailer == nil {
		return ErrMailerNotConfigured
	}
	if s.redis == nil {
		return ErrRedisNotConfigured
	}
	if userID == 0 {
		return ErrUserNotFound
	}

	u, err := s.users.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	normEmail := strings.ToLower(strings.TrimSpace(u.Email))
	limited, err := s.redis.SetNX(ctx, changeRateLimitKey(userID), "1", s.cfg.RateLimit)
	if err != nil {
		return err
	}
	if !limited {
		return ErrTooManyRequests
	}

	code, err := generateNumericCode(6)
	if err != nil {
		return err
	}
	codeHash := hashToken(code)
	codeKey := changeCodeKey(userID)
	if err := s.redis.Set(ctx, codeKey, codeHash, s.cfg.CodeTTL); err != nil {
		return err
	}
	_ = s.redis.Set(ctx, changeCodeAttemptsKey(userID), "0", s.cfg.CodeTTL)
	if err := s.mailer.SendOneTimeCode(ctx, normEmail, "Password change code", code); err != nil {
		_, _ = s.redis.Del(ctx, codeKey, changeCodeAttemptsKey(userID))
		return err
	}
	return nil
}

func (s *PasswordResetService) VerifyPasswordChangeCode(ctx context.Context, userID int64, code string) (string, error) {
	if code == "" {
		return "", ErrChangeCodeRequired
	}
	if s.redis == nil {
		return "", ErrRedisNotConfigured
	}
	if userID == 0 {
		return "", ErrUserNotFound
	}

	codeKey := changeCodeKey(userID)
	codeHash, err := s.redis.Get(ctx, codeKey)
	if err != nil || codeHash == "" {
		return "", ErrChangeCodeInvalid
	}
	attemptsKey := changeCodeAttemptsKey(userID)
	attemptsStr, _ := s.redis.Get(ctx, attemptsKey)
	attempts := parseAttempts(attemptsStr)
	if attempts >= s.cfg.MaxAttempts {
		_, _ = s.redis.Del(ctx, codeKey, attemptsKey)
		return "", ErrChangeCodeAttempts
	}
	if !tokensEqual(hashToken(code), codeHash) {
		attempts++
		if err := s.redis.Set(ctx, attemptsKey, strconv.Itoa(attempts), s.cfg.CodeTTL); err != nil {
			return "", err
		}
		if attempts >= s.cfg.MaxAttempts {
			_, _ = s.redis.Del(ctx, codeKey, attemptsKey)
			return "", ErrChangeCodeAttempts
		}
		return "", ErrChangeCodeInvalid
	}

	_, _ = s.redis.Del(ctx, codeKey, attemptsKey)
	token, err := generateToken(32)
	if err != nil {
		return "", err
	}
	if err := s.redis.Set(ctx, changeTokenKey(token), strconv.FormatInt(userID, 10), s.cfg.SessionTTL); err != nil {
		return "", err
	}
	return token, nil
}

func (s *PasswordResetService) ConfirmPasswordChange(ctx context.Context, changeToken, password, passwordConfirm string) error {
	if changeToken == "" {
		return ErrChangeTokenRequired
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	if password != passwordConfirm {
		return ErrPasswordMismatch
	}
	if s.redis == nil {
		return ErrRedisNotConfigured
	}

	userIDStr, err := s.redis.Get(ctx, changeTokenKey(changeToken))
	if err != nil || userIDStr == "" {
		return ErrChangeSessionInvalid
	}
	_, _ = s.redis.Del(ctx, changeTokenKey(changeToken))
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return ErrChangeSessionInvalid
	}
	hashed, err := hash(password)
	if err != nil {
		return err
	}
	if err := s.users.UpdateUserPassword(ctx, userID, hashed); err != nil {
		return err
	}
	return nil
}

func changeCodeKey(userID int64) string {
	return "auth:pwdchange:code:" + strconv.FormatInt(userID, 10)
}

func changeCodeAttemptsKey(userID int64) string {
	return "auth:pwdchange:code:attempts:" + strconv.FormatInt(userID, 10)
}

func changeTokenKey(token string) string {
	return "auth:pwdchange:token:" + token
}

func changeRateLimitKey(userID int64) string {
	return "auth:pwdchange:rate:" + strconv.FormatInt(userID, 10)
}
