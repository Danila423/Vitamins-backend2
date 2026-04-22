package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"vitamins-backend_2/internal/cache"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// TestPasswordResetVerifyConfirm_WithMiniredis exercises VerifyPasswordResetCode
// and ConfirmPasswordReset using a real Redis-compatible store (miniredis) but
// without going anywhere near SMTP: the one-time code is injected directly into
// the cache, exactly as it would be after RequestPasswordReset succeeds.
//
// This intentionally does not invoke the mailer because per project policy we
// don't want tests to touch the e-mail send/receive path.
func TestPasswordResetVerifyConfirm_WithMiniredis(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := cache.NewRedisStore(client)

	users := &fakeUserRepo{
		getUserByEmailFn: func(_ context.Context, email string) (User, error) {
			return User{ID: 7, Email: email, PasswordHash: "old-hash"}, nil
		},
		updateUserPasswordFn: func(_ context.Context, _ int64, _ string) error { return nil },
	}
	cfg := PasswordResetConfig{CodeTTL: time.Minute, SessionTTL: time.Minute, MaxAttempts: 3, RateLimit: time.Second}
	svc := NewPasswordResetService(users, nil, store, cfg)
	ctx := context.Background()

	const email = "user@example.com"
	const code = "424242"

	sum := sha256.Sum256([]byte(code))
	hashed := hex.EncodeToString(sum[:])
	if err := store.Set(ctx, resetCodeKey(email), hashed, time.Minute); err != nil {
		t.Fatalf("seed code: %v", err)
	}
	if err := store.Set(ctx, resetCodeAttemptsKey(email), "0", time.Minute); err != nil {
		t.Fatalf("seed attempts: %v", err)
	}

	if _, err := svc.VerifyPasswordResetCode(ctx, email, "000000"); err != ErrResetCodeInvalid {
		t.Fatalf("first wrong code: want ErrResetCodeInvalid, got %v", err)
	}
	attempts, _ := store.Get(ctx, resetCodeAttemptsKey(email))
	if attempts != "1" {
		t.Fatalf("attempts should be 1 after wrong code, got %q", attempts)
	}

	resetToken, err := svc.VerifyPasswordResetCode(ctx, email, code)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if resetToken == "" {
		t.Fatalf("expected non-empty reset token")
	}

	if val, _ := store.Get(ctx, resetCodeKey(email)); val != "" {
		t.Fatalf("code key should be gone after success, still: %q", val)
	}

	storedUserID, err := store.Get(ctx, resetTokenKey(resetToken))
	if err != nil {
		t.Fatalf("get reset token: %v", err)
	}
	if storedUserID != strconv.FormatInt(7, 10) {
		t.Fatalf("reset token should map to user 7, got %q", storedUserID)
	}

	if err := svc.ConfirmPasswordReset(ctx, resetToken, "Brand-New-Pass1", "Brand-New-Pass1"); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	if val, _ := store.Get(ctx, resetTokenKey(resetToken)); val != "" {
		t.Fatalf("reset token must be invalidated after confirm, still: %q", val)
	}
	if err := svc.ConfirmPasswordReset(ctx, resetToken, "Brand-New-Pass1", "Brand-New-Pass1"); err != ErrResetSessionInvalid {
		t.Fatalf("reuse confirm: want ErrResetSessionInvalid, got %v", err)
	}
}

func TestPasswordResetVerify_LocksAfterMaxAttempts_WithMiniredis(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := cache.NewRedisStore(client)

	users := &fakeUserRepo{
		getUserByEmailFn: func(_ context.Context, email string) (User, error) {
			return User{ID: 1, Email: email, PasswordHash: "old"}, nil
		},
	}
	cfg := PasswordResetConfig{CodeTTL: time.Minute, SessionTTL: time.Minute, MaxAttempts: 2, RateLimit: time.Second}
	svc := NewPasswordResetService(users, nil, store, cfg)
	ctx := context.Background()

	const email = "lock@example.com"
	sum := sha256.Sum256([]byte("999999"))
	if err := store.Set(ctx, resetCodeKey(email), hex.EncodeToString(sum[:]), time.Minute); err != nil {
		t.Fatalf("seed code: %v", err)
	}
	if err := store.Set(ctx, resetCodeAttemptsKey(email), "0", time.Minute); err != nil {
		t.Fatalf("seed attempts: %v", err)
	}

	if _, err := svc.VerifyPasswordResetCode(ctx, email, "111111"); err != ErrResetCodeInvalid {
		t.Fatalf("first wrong: %v", err)
	}
	if _, err := svc.VerifyPasswordResetCode(ctx, email, "222222"); err != ErrResetCodeAttempts {
		t.Fatalf("second wrong should lock with ErrResetCodeAttempts, got %v", err)
	}
	if val, _ := store.Get(ctx, resetCodeKey(email)); val != "" {
		t.Fatalf("code key must be cleared after lock, still: %q", val)
	}
	if _, err := svc.VerifyPasswordResetCode(ctx, email, "999999"); err != ErrResetCodeInvalid {
		t.Fatalf("after lock, even right code now reads as invalid: %v", err)
	}
}
