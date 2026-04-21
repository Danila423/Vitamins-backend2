package auth

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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
	"vitamins-backend_2/internal/db"
	appLogger "vitamins-backend_2/internal/logger"
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

type Service struct {
	q        *db.Queries
	jwt      *JWTManager
	mailer   Mailer
	redis    RedisStore
	resetCfg PasswordResetConfig
	now      func() time.Time
}

type Mailer interface {
	SendPasswordResetCode(ctx context.Context, toEmail, code string) error
}

type PasswordResetConfig struct {
	CodeTTL     time.Duration
	SessionTTL  time.Duration
	MaxAttempts int
	RateLimit   time.Duration
}

type RedisStore interface {
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) (int64, error)
}

func NewService(q *db.Queries, j *JWTManager, m Mailer, r RedisStore, cfg PasswordResetConfig) *Service {
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
	return &Service{q: q, jwt: j, mailer: m, redis: r, resetCfg: cfg, now: time.Now}
}

func hash(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func check(h, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(h), []byte(pw)) == nil
}

func (s *Service) Register(ctx context.Context, email, pw string) (*TokenPair, error) {
	if err := ValidateEmailPassword(email, pw); err != nil {
		return nil, err
	}

	h, err := hash(pw)
	if err != nil {
		return nil, err
	}

	user, err := s.q.CreateUser(ctx, db.CreateUserParams{Email: email, PasswordHash: h})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailAlreadyExists
		}
		return nil, err
	}

	return s.jwt.GenerateTokenPair(user.ID)
}

func (s *Service) Login(ctx context.Context, email, pw string) (*TokenPair, error) {
	if err := ValidateEmailPassword(email, pw); err != nil {
		return nil, err
	}

	u, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !check(u.PasswordHash, pw) {
		return nil, ErrInvalidCredentials
	}

	return s.jwt.GenerateTokenPair(u.ID)
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
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
	limited, err := s.redis.SetNX(ctx, rateLimitKey(normEmail), "1", s.resetCfg.RateLimit)
	if err != nil {
		log.Error("password reset rate limit set failed", "error", err.Error())
		return err
	}
	if !limited {
		log.Warn("password reset rate limit hit")
		return ErrTooManyRequests
	}
	if _, err := s.q.GetUserByEmail(ctx, normEmail); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
	if err := s.redis.Set(ctx, codeKey, codeHash, s.resetCfg.CodeTTL); err != nil {
		log.Error("password reset cache write failed", "error", err.Error())
		return err
	}
	_ = s.redis.Set(ctx, resetCodeAttemptsKey(normEmail), "0", s.resetCfg.CodeTTL)
	if err := s.mailer.SendPasswordResetCode(ctx, normEmail, code); err != nil {
		log.Error("password reset mail send failed", "error", err.Error())
		_, _ = s.redis.Del(ctx, codeKey, resetCodeAttemptsKey(normEmail))
		return err
	}
	return nil
}

func (s *Service) VerifyPasswordResetCode(ctx context.Context, email, code string) (string, error) {
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
	u, err := s.q.GetUserByEmail(ctx, normEmail)
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
	if attempts >= s.resetCfg.MaxAttempts {
		_, _ = s.redis.Del(ctx, codeKey, attemptsKey)
		return "", ErrResetCodeAttempts
	}
	if !tokensEqual(hashToken(code), codeHash) {
		attempts++
		if err := s.redis.Set(ctx, attemptsKey, strconv.Itoa(attempts), s.resetCfg.CodeTTL); err != nil {
			return "", err
		}
		if attempts >= s.resetCfg.MaxAttempts {
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
	if err := s.redis.Set(ctx, resetTokenKey(token), strconv.FormatInt(u.ID, 10), s.resetCfg.SessionTTL); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, resetToken, password, passwordConfirm string) error {
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
	if err := s.q.UpdateUserPassword(ctx, userID, hashed); err != nil {
		return err
	}
	return nil
}

func generateNumericCode(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("invalid length")
	}
	max := big.NewInt(1)
	for i := 0; i < length; i++ {
		max.Mul(max, big.NewInt(10))
	}
	n, err := rand.Int(rand.Reader, max)
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

func (s *Service) Refresh(ctx context.Context, token string) (*TokenPair, error) {
	c, err := s.jwt.Parse(token)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	return s.jwt.GenerateTokenPair(c.UserID)
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
