package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"strings"
	"time"

	appLogger "vitamins-backend_2/internal/logger"
)

// PasswordResetService handles the two password flows (reset + change). It
// requires a mailer and a redis store; without them the flows return the
// corresponding NOT_CONFIGURED error.
type PasswordResetService struct {
	users  UserRepository
	mailer Mailer
	redis  RedisStore
	cfg    PasswordResetConfig
	now    func() time.Time
}

func (s *PasswordResetService) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	if err := ValidateEmail(email); err != nil {
		return err
	}
	if s.mailer == nil {
		return ErrMailerNotConfigured
	}
	if s.redis == nil {
		return ErrRedisNotConfigured
	}
	normEmail := strings.ToLower(email)
	log := appLogger.WithContext(slog.Default(), ctx).With(
		"channel", "app",
		"operation", "auth.password_reset.request",
		"user.email_masked", appLogger.MaskEmail(normEmail),
	)
	limited, err := s.redis.SetNX(ctx, rateLimitKey(normEmail), "1", s.cfg.RateLimit)
	if err != nil {
		log.Error("password reset rate limit set failed", "error", err.Error())
		return err
	}
	if !limited {
		log.Warn("password reset rate limit hit")
		return ErrTooManyRequests
	}
	if _, err := s.users.GetUserByEmail(ctx, normEmail); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			log.Warn("password reset user not found")
			return ErrUserNotFound
		}
		log.Error("password reset db lookup failed", "error", err.Error())
		return err
	}
	code, err := generateNumericCode(6)
	if err != nil {
		log.Error("password reset code generation failed", "error", err.Error())
		return err
	}
	codeHash := hashToken(code)
	codeKey := resetCodeKey(normEmail)
	if err := s.redis.Set(ctx, codeKey, codeHash, s.cfg.CodeTTL); err != nil {
		log.Error("password reset cache write failed", "error", err.Error())
		return err
	}
	_ = s.redis.Set(ctx, resetCodeAttemptsKey(normEmail), "0", s.cfg.CodeTTL)
	if err := s.mailer.SendOneTimeCode(ctx, normEmail, "Password reset code", code); err != nil {
		log.Error("password reset mail send failed", "error", err.Error())
		_, _ = s.redis.Del(ctx, codeKey, resetCodeAttemptsKey(normEmail))
		return err
	}
	return nil
}

func (s *PasswordResetService) VerifyPasswordResetCode(ctx context.Context, email, code string) (string, error) {
	email = strings.TrimSpace(email)
	if err := ValidateEmail(email); err != nil {
		return "", err
	}
	if code == "" {
		return "", ErrResetCodeRequired
	}
	if s.redis == nil {
		return "", ErrRedisNotConfigured
	}
	normEmail := strings.ToLower(email)
	u, err := s.users.GetUserByEmail(ctx, normEmail)
	if err != nil {
		return "", ErrResetCodeInvalid
	}
	codeKey := resetCodeKey(normEmail)
	codeHash, err := s.redis.Get(ctx, codeKey)
	if err != nil || codeHash == "" {
		return "", ErrResetCodeInvalid
	}
	attemptsKey := resetCodeAttemptsKey(normEmail)
	attemptsStr, _ := s.redis.Get(ctx, attemptsKey)
	attempts := parseAttempts(attemptsStr)
	if attempts >= s.cfg.MaxAttempts {
		_, _ = s.redis.Del(ctx, codeKey, attemptsKey)
		return "", ErrResetCodeAttempts
	}
	if !tokensEqual(hashToken(code), codeHash) {
		attempts++
		if err := s.redis.Set(ctx, attemptsKey, strconv.Itoa(attempts), s.cfg.CodeTTL); err != nil {
			return "", err
		}
		if attempts >= s.cfg.MaxAttempts {
			_, _ = s.redis.Del(ctx, codeKey, attemptsKey)
			return "", ErrResetCodeAttempts
		}
		return "", ErrResetCodeInvalid
	}
	_, _ = s.redis.Del(ctx, codeKey, attemptsKey)
	token, err := generateToken(32)
	if err != nil {
		return "", err
	}
	if err := s.redis.Set(ctx, resetTokenKey(token), strconv.FormatInt(u.ID, 10), s.cfg.SessionTTL); err != nil {
		return "", err
	}
	return token, nil
}

func (s *PasswordResetService) ConfirmPasswordReset(ctx context.Context, resetToken, password, passwordConfirm string) error {
	if resetToken == "" {
		return ErrResetTokenRequired
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
	userIDStr, err := s.redis.Get(ctx, resetTokenKey(resetToken))
	if err != nil || userIDStr == "" {
		return ErrResetSessionInvalid
	}
	_, _ = s.redis.Del(ctx, resetTokenKey(resetToken))
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return ErrResetSessionInvalid
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

// --- Helpers shared with the password change flow ---

func generateNumericCode(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("invalid length")
	}
	upper := big.NewInt(1)
	for i := 0; i < length; i++ {
		upper.Mul(upper, big.NewInt(10))
	}
	n, err := rand.Int(rand.Reader, upper)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", length, n.Int64()), nil
}

func generateToken(bytesLen int) (string, error) {
	if bytesLen <= 0 {
		return "", errors.New("invalid token length")
	}
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func tokensEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

func resetCodeKey(email string) string {
	return "auth:pwdreset:code:" + email
}

func resetCodeAttemptsKey(email string) string {
	return "auth:pwdreset:code:attempts:" + email
}

func resetTokenKey(token string) string {
	return "auth:pwdreset:token:" + token
}

func rateLimitKey(email string) string {
	return "auth:pwdreset:rate:" + email
}

func parseAttempts(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}
