package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUserRepo struct {
	createUserFn         func(ctx context.Context, email, passwordHash string) (User, error)
	getUserByEmailFn     func(ctx context.Context, email string) (User, error)
	getUserByIDFn        func(ctx context.Context, userID int64) (User, error)
	updateUserPasswordFn func(ctx context.Context, userID int64, passwordHash string) error
	updateUserProfileFn  func(ctx context.Context, userID int64, email, firstName, lastName string) (User, error)
}

func (f *fakeUserRepo) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	if f.createUserFn == nil {
		return User{}, errors.New("create user not stubbed")
	}
	return f.createUserFn(ctx, email, passwordHash)
}

func (f *fakeUserRepo) GetUserByEmail(ctx context.Context, email string) (User, error) {
	if f.getUserByEmailFn == nil {
		return User{}, ErrUserNotFound
	}
	return f.getUserByEmailFn(ctx, email)
}

func (f *fakeUserRepo) GetUserByID(ctx context.Context, userID int64) (User, error) {
	if f.getUserByIDFn == nil {
		return User{}, ErrUserNotFound
	}
	return f.getUserByIDFn(ctx, userID)
}

func (f *fakeUserRepo) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	if f.updateUserPasswordFn == nil {
		return nil
	}
	return f.updateUserPasswordFn(ctx, userID, passwordHash)
}

func (f *fakeUserRepo) UpdateUserProfile(ctx context.Context, userID int64, email, firstName, lastName string) (User, error) {
	if f.updateUserProfileFn == nil {
		return User{}, errors.New("update profile not stubbed")
	}
	return f.updateUserProfileFn(ctx, userID, email, firstName, lastName)
}

type fakeRedis struct {
	data       map[string]string
	setnxTaken map[string]bool
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{
		data:       make(map[string]string),
		setnxTaken: make(map[string]bool),
	}
}

func (f *fakeRedis) SetNX(_ context.Context, key, value string, _ time.Duration) (bool, error) {
	if f.setnxTaken[key] || f.data[key] != "" {
		return false, nil
	}
	f.setnxTaken[key] = true
	f.data[key] = value
	return true, nil
}

func (f *fakeRedis) Set(_ context.Context, key, value string, _ time.Duration) error {
	f.data[key] = value
	return nil
}

func (f *fakeRedis) Get(_ context.Context, key string) (string, error) {
	v, ok := f.data[key]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func (f *fakeRedis) Del(_ context.Context, keys ...string) (int64, error) {
	var n int64
	for _, key := range keys {
		if _, ok := f.data[key]; ok {
			delete(f.data, key)
			n++
		}
	}
	return n, nil
}

func testJWT() *JWTManager {
	return NewJWTManager("unit-test-secret", 5*time.Minute, 30*time.Minute)
}

func TestService_Register(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		repo := &fakeUserRepo{
			createUserFn: func(_ context.Context, email, hash string) (User, error) {
				require.Equal(t, "user@test.local", email)
				require.NotEmpty(t, hash)
				require.NotEqual(t, "Passw0rd1", hash)
				return User{ID: 42, Email: email}, nil
			},
		}
		svc := NewServiceWithDeps(repo, testJWT(), nil, nil, PasswordResetConfig{})

		got, err := svc.Register(context.Background(), "user@test.local", "Passw0rd1")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.NotEmpty(t, got.AccessToken)
		assert.NotEmpty(t, got.RefreshToken)
	})

	t.Run("duplicate email maps to domain error", func(t *testing.T) {
		t.Parallel()

		repo := &fakeUserRepo{
			createUserFn: func(_ context.Context, _, _ string) (User, error) {
				return User{}, ErrEmailConflict
			},
		}
		svc := NewServiceWithDeps(repo, testJWT(), nil, nil, PasswordResetConfig{})

		_, err := svc.Register(context.Background(), "dup@test.local", "Passw0rd1")
		require.ErrorIs(t, err, ErrEmailAlreadyExists)
	})
}

func TestService_Login(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		passHash, err := hash("Passw0rd1")
		require.NoError(t, err)

		repo := &fakeUserRepo{
			getUserByEmailFn: func(_ context.Context, email string) (User, error) {
				require.Equal(t, "user@test.local", email)
				return User{ID: 7, Email: email, PasswordHash: passHash}, nil
			},
		}
		svc := NewServiceWithDeps(repo, testJWT(), nil, nil, PasswordResetConfig{})

		got, err := svc.Login(context.Background(), "user@test.local", "Passw0rd1")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.NotEmpty(t, got.AccessToken)
	})

	t.Run("unknown user returns invalid credentials", func(t *testing.T) {
		t.Parallel()

		repo := &fakeUserRepo{
			getUserByEmailFn: func(_ context.Context, _ string) (User, error) {
				return User{}, ErrUserNotFound
			},
		}
		svc := NewServiceWithDeps(repo, testJWT(), nil, nil, PasswordResetConfig{})

		_, err := svc.Login(context.Background(), "missing@test.local", "Passw0rd1")
		require.ErrorIs(t, err, ErrInvalidCredentials)
	})
}

func TestService_Refresh(t *testing.T) {
	t.Parallel()

	t.Run("success rotates pair", func(t *testing.T) {
		t.Parallel()

		jwt := testJWT()
		svc := NewServiceWithDeps(&fakeUserRepo{}, jwt, nil, nil, PasswordResetConfig{})

		initial, err := jwt.GenerateTokenPair(11)
		require.NoError(t, err)

		got, err := svc.Refresh(context.Background(), initial.RefreshToken)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.NotEmpty(t, got.AccessToken)
		assert.NotEmpty(t, got.RefreshToken)
	})

	t.Run("rejects access token as refresh", func(t *testing.T) {
		t.Parallel()

		jwt := testJWT()
		svc := NewServiceWithDeps(&fakeUserRepo{}, jwt, nil, nil, PasswordResetConfig{})

		pair, err := jwt.GenerateTokenPair(11)
		require.NoError(t, err)
		_, err = svc.Refresh(context.Background(), pair.AccessToken)
		require.ErrorIs(t, err, ErrInvalidCredentials)
	})

	t.Run("invalid token returns invalid credentials", func(t *testing.T) {
		t.Parallel()

		svc := NewServiceWithDeps(&fakeUserRepo{}, testJWT(), nil, nil, PasswordResetConfig{})
		_, err := svc.Refresh(context.Background(), "bad")
		require.ErrorIs(t, err, ErrInvalidCredentials)
	})

	t.Run("reused refresh token rejected when redis allowlist configured", func(t *testing.T) {
		t.Parallel()

		jwt := testJWT()
		redis := newFakeRedis()
		svc := NewServiceWithDeps(&fakeUserRepo{}, jwt, nil, redis, PasswordResetConfig{})

		first, err := svc.Register(context.Background(), "reuse@test.local", "Passw0rd1")
		// Register in this setup needs a repo that creates the user
		_ = first
		_ = err

		// Directly test rotation by issuing a pair and reusing refresh.
		redis.data = map[string]string{}
		redis.setnxTaken = map[string]bool{}
		// Issue and rotate
		pair, err := jwt.GenerateTokenPair(55)
		require.NoError(t, err)
		// Pretend the pair is active
		parsed, err := jwt.ParseWithType(pair.RefreshToken, TokenTypeRefresh)
		require.NoError(t, err)
		require.NoError(t, redis.Set(context.Background(), refreshAllowKey(parsed.ID), "55", time.Minute))

		rotated, err := svc.Refresh(context.Background(), pair.RefreshToken)
		require.NoError(t, err)
		require.NotNil(t, rotated)

		// Second use of the same (now revoked) refresh token must fail.
		_, err = svc.Refresh(context.Background(), pair.RefreshToken)
		require.ErrorIs(t, err, ErrInvalidCredentials)
	})
}

func TestService_GetAndUpdateProfile(t *testing.T) {
	t.Parallel()

	t.Run("get profile user not found", func(t *testing.T) {
		t.Parallel()

		repo := &fakeUserRepo{
			getUserByIDFn: func(_ context.Context, _ int64) (User, error) {
				return User{}, ErrUserNotFound
			},
		}
		svc := NewServiceWithDeps(repo, testJWT(), nil, nil, PasswordResetConfig{})
		_, err := svc.GetProfile(context.Background(), 10)
		require.ErrorIs(t, err, ErrUserNotFound)
	})

	t.Run("update profile success normalizes email and trims names", func(t *testing.T) {
		t.Parallel()

		repo := &fakeUserRepo{
			getUserByIDFn: func(_ context.Context, userID int64) (User, error) {
				require.Equal(t, int64(5), userID)
				return User{
					ID:        5,
					Email:     "old@test.local",
					FirstName: "",
					LastName:  "",
				}, nil
			},
			getUserByEmailFn: func(_ context.Context, _ string) (User, error) {
				return User{}, ErrUserNotFound
			},
			updateUserProfileFn: func(_ context.Context, userID int64, email, firstName, lastName string) (User, error) {
				assert.Equal(t, int64(5), userID)
				assert.Equal(t, "new@test.local", email)
				assert.Equal(t, "Dan", firstName)
				assert.Equal(t, "Nest", lastName)
				return User{
					ID:        userID,
					Email:     email,
					FirstName: firstName,
					LastName:  lastName,
				}, nil
			},
		}
		svc := NewServiceWithDeps(repo, testJWT(), nil, nil, PasswordResetConfig{})

		first := "  Dan  "
		last := " Nest "
		email := "  NEW@test.local "
		got, err := svc.UpdateProfile(context.Background(), 5, ProfileUpdate{
			FirstName: &first,
			LastName:  &last,
			Email:     &email,
		})
		require.NoError(t, err)
		assert.Equal(t, "new@test.local", got.Email)
		assert.Equal(t, "Dan", got.FirstName)
		assert.Equal(t, "Nest", got.LastName)
	})
}

func TestService_VerifyAndConfirmPasswordReset_NonMailScenarios(t *testing.T) {
	t.Parallel()

	t.Run("verify reset wrong code increments attempts", func(t *testing.T) {
		t.Parallel()

		repo := &fakeUserRepo{
			getUserByEmailFn: func(_ context.Context, _ string) (User, error) {
				return User{ID: 1, Email: "u@test.local"}, nil
			},
		}
		redis := newFakeRedis()
		redis.data[resetCodeKey("u@test.local")] = hashToken("123456")
		redis.data[resetCodeAttemptsKey("u@test.local")] = "0"
		svc := NewServiceWithDeps(repo, testJWT(), nil, redis, PasswordResetConfig{
			CodeTTL:     time.Minute,
			SessionTTL:  time.Minute,
			MaxAttempts: 5,
			RateLimit:   time.Minute,
		})

		_, err := svc.VerifyPasswordResetCode(context.Background(), "u@test.local", "000000")
		require.ErrorIs(t, err, ErrResetCodeInvalid)
		assert.Equal(t, "1", redis.data[resetCodeAttemptsKey("u@test.local")])
	})

	t.Run("verify reset success issues token and removes code keys", func(t *testing.T) {
		t.Parallel()

		repo := &fakeUserRepo{
			getUserByEmailFn: func(_ context.Context, _ string) (User, error) {
				return User{ID: 33, Email: "u@test.local"}, nil
			},
		}
		redis := newFakeRedis()
		redis.data[resetCodeKey("u@test.local")] = hashToken("123456")
		redis.data[resetCodeAttemptsKey("u@test.local")] = "1"
		svc := NewServiceWithDeps(repo, testJWT(), nil, redis, PasswordResetConfig{
			CodeTTL:     time.Minute,
			SessionTTL:  time.Minute,
			MaxAttempts: 5,
			RateLimit:   time.Minute,
		})

		token, err := svc.VerifyPasswordResetCode(context.Background(), "u@test.local", "123456")
		require.NoError(t, err)
		require.NotEmpty(t, token)
		_, hasCode := redis.data[resetCodeKey("u@test.local")]
		_, hasAttempts := redis.data[resetCodeAttemptsKey("u@test.local")]
		assert.False(t, hasCode)
		assert.False(t, hasAttempts)
		assert.Equal(t, "33", redis.data[resetTokenKey(token)])
	})

	t.Run("confirm reset updates password by token", func(t *testing.T) {
		t.Parallel()

		redis := newFakeRedis()
		redis.data[resetTokenKey("token-1")] = "9"

		var updatedUserID int64
		var updatedHash string
		repo := &fakeUserRepo{
			updateUserPasswordFn: func(_ context.Context, userID int64, passwordHash string) error {
				updatedUserID = userID
				updatedHash = passwordHash
				return nil
			},
		}
		svc := NewServiceWithDeps(repo, testJWT(), nil, redis, PasswordResetConfig{})

		err := svc.ConfirmPasswordReset(context.Background(), "token-1", "Newpass1", "Newpass1")
		require.NoError(t, err)
		assert.Equal(t, int64(9), updatedUserID)
		assert.NotEmpty(t, updatedHash)
		assert.True(t, check(updatedHash, "Newpass1"))
		_, stillExists := redis.data[resetTokenKey("token-1")]
		assert.False(t, stillExists)
	})
}

func TestService_VerifyAndConfirmPasswordChange_NonMailScenarios(t *testing.T) {
	t.Parallel()

	t.Run("verify change success issues change token", func(t *testing.T) {
		t.Parallel()

		redis := newFakeRedis()
		redis.data[changeCodeKey(4)] = hashToken("222222")
		redis.data[changeCodeAttemptsKey(4)] = "0"
		svc := NewServiceWithDeps(&fakeUserRepo{}, testJWT(), nil, redis, PasswordResetConfig{
			CodeTTL:     time.Minute,
			SessionTTL:  time.Minute,
			MaxAttempts: 5,
			RateLimit:   time.Minute,
		})

		token, err := svc.VerifyPasswordChangeCode(context.Background(), 4, "222222")
		require.NoError(t, err)
		require.NotEmpty(t, token)
		assert.Equal(t, strconv.FormatInt(4, 10), redis.data[changeTokenKey(token)])
	})

	t.Run("confirm change validates token and updates password", func(t *testing.T) {
		t.Parallel()

		redis := newFakeRedis()
		redis.data[changeTokenKey("ct-1")] = "12"
		repo := &fakeUserRepo{
			updateUserPasswordFn: func(_ context.Context, userID int64, passwordHash string) error {
				assert.Equal(t, int64(12), userID)
				assert.True(t, strings.HasPrefix(passwordHash, "$2"))
				return nil
			},
		}
		svc := NewServiceWithDeps(repo, testJWT(), nil, redis, PasswordResetConfig{})

		err := svc.ConfirmPasswordChange(context.Background(), "ct-1", "Change123", "Change123")
		require.NoError(t, err)
		_, exists := redis.data[changeTokenKey("ct-1")]
		assert.False(t, exists)
	})
}

func TestService_RequestPasswordFlowGuards_NonMail(t *testing.T) {
	t.Parallel()

	t.Run("request password reset fails if mailer missing", func(t *testing.T) {
		t.Parallel()
		svc := NewServiceWithDeps(&fakeUserRepo{}, testJWT(), nil, newFakeRedis(), PasswordResetConfig{})
		err := svc.RequestPasswordReset(context.Background(), "user@test.local")
		require.ErrorIs(t, err, ErrMailerNotConfigured)
	})

	t.Run("request password reset fails if redis missing", func(t *testing.T) {
		t.Parallel()
		svc := NewServiceWithDeps(&fakeUserRepo{}, testJWT(), noopMailer{}, nil, PasswordResetConfig{})
		err := svc.RequestPasswordReset(context.Background(), "user@test.local")
		require.ErrorIs(t, err, ErrRedisNotConfigured)
	})

	t.Run("request password change fails if mailer missing", func(t *testing.T) {
		t.Parallel()
		svc := NewServiceWithDeps(&fakeUserRepo{}, testJWT(), nil, newFakeRedis(), PasswordResetConfig{})
		err := svc.RequestPasswordChange(context.Background(), 1)
		require.ErrorIs(t, err, ErrMailerNotConfigured)
	})

	t.Run("request password change fails if redis missing", func(t *testing.T) {
		t.Parallel()
		svc := NewServiceWithDeps(&fakeUserRepo{}, testJWT(), noopMailer{}, nil, PasswordResetConfig{})
		err := svc.RequestPasswordChange(context.Background(), 1)
		require.ErrorIs(t, err, ErrRedisNotConfigured)
	})
}

type noopMailer struct{}

func (noopMailer) SendOneTimeCode(_ context.Context, _, _, _ string) error { return nil }
