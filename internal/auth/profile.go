package auth

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"vitamins-backend_2/internal/db"
)

type ProfileUpdate struct {
	FirstName *string
	LastName  *string
	Email     *string
}

func (s *Service) GetProfile(ctx context.Context, userID int64) (db.User, error) {
	if userID == 0 {
		return db.User{}, ErrUserNotFound
	}
	u, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.User{}, ErrUserNotFound
		}
		return db.User{}, err
	}
	return u, nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID int64, in ProfileUpdate) (db.User, error) {
	if in.FirstName == nil && in.LastName == nil && in.Email == nil {
		return db.User{}, ErrNoFieldsToUpdate
	}
	if userID == 0 {
		return db.User{}, ErrUserNotFound
	}

	u, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.User{}, ErrUserNotFound
		}
		return db.User{}, err
	}

	nextEmail := u.Email
	if in.Email != nil {
		normEmail := strings.ToLower(strings.TrimSpace(*in.Email))
		if err := ValidateEmail(normEmail); err != nil {
			return db.User{}, err
		}
		nextEmail = normEmail
		if strings.ToLower(u.Email) != normEmail {
			existing, err := s.q.GetUserByEmail(ctx, normEmail)
			if err == nil && existing.ID != userID {
				return db.User{}, ErrEmailAlreadyExists
			}
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return db.User{}, err
			}
		}
	}

	nextFirstName := u.FirstName
	if in.FirstName != nil {
		nextFirstName = strings.TrimSpace(*in.FirstName)
	}
	nextLastName := u.LastName
	if in.LastName != nil {
		nextLastName = strings.TrimSpace(*in.LastName)
	}

	updated, err := s.q.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		Email:     nextEmail,
		FirstName: nextFirstName,
		LastName:  nextLastName,
		ID:        userID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return db.User{}, ErrEmailAlreadyExists
		}
		return db.User{}, err
	}
	return updated, nil
}

func (s *Service) RequestPasswordChange(ctx context.Context, userID int64) error {
	if s.mailer == nil {
		return ErrMailerNotConfigured
	}
	if s.redis == nil {
		return ErrRedisNotConfigured
	}
	if userID == 0 {
		return ErrUserNotFound
	}

	u, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}

	normEmail := strings.ToLower(strings.TrimSpace(u.Email))
	limited, err := s.redis.SetNX(ctx, changeRateLimitKey(userID), "1", s.resetCfg.RateLimit)
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
	if err := s.redis.Set(ctx, codeKey, codeHash, s.resetCfg.CodeTTL); err != nil {
		return err
	}
	_ = s.redis.Set(ctx, changeCodeAttemptsKey(userID), "0", s.resetCfg.CodeTTL)
	if err := s.mailer.SendPasswordResetCode(ctx, normEmail, code); err != nil {
		_, _ = s.redis.Del(ctx, codeKey, changeCodeAttemptsKey(userID))
		return err
	}
	return nil
}

func (s *Service) VerifyPasswordChangeCode(ctx context.Context, userID int64, code string) (string, error) {
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
	if attempts >= s.resetCfg.MaxAttempts {
		_, _ = s.redis.Del(ctx, codeKey, attemptsKey)
		return "", ErrChangeCodeAttempts
	}
	if !tokensEqual(hashToken(code), codeHash) {
		attempts++
		if err := s.redis.Set(ctx, attemptsKey, strconv.Itoa(attempts), s.resetCfg.CodeTTL); err != nil {
			return "", err
		}
		if attempts >= s.resetCfg.MaxAttempts {
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
	if err := s.redis.Set(ctx, changeTokenKey(token), strconv.FormatInt(userID, 10), s.resetCfg.SessionTTL); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) ConfirmPasswordChange(ctx context.Context, changeToken, password, passwordConfirm string) error {
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
	if err := s.q.UpdateUserPassword(ctx, userID, hashed); err != nil {
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
