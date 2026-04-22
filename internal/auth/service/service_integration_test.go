//go:build integration

package service

import (
	"context"
	"testing"
	"time"

	"vitamins-backend_2/internal/db"
	"vitamins-backend_2/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_RegisterLoginRefresh_Integration(t *testing.T) {
	pool := testutil.NewTestPool(t)
	testutil.ResetTables(t, pool)

	q := db.New(pool)
	jwt := NewJWTManager("integration-secret", 2*time.Minute, 10*time.Minute)
	svc := NewService(q, jwt, nil, nil, PasswordResetConfig{})

	ctx := context.Background()
	registerTokens, err := svc.Register(ctx, "user1@test.local", "Passw0rd1")
	require.NoError(t, err)
	require.NotEmpty(t, registerTokens.AccessToken)
	require.NotEmpty(t, registerTokens.RefreshToken)

	loginTokens, err := svc.Login(ctx, "user1@test.local", "Passw0rd1")
	require.NoError(t, err)
	require.NotEmpty(t, loginTokens.AccessToken)
	require.NotEmpty(t, loginTokens.RefreshToken)

	refreshed, err := svc.Refresh(ctx, loginTokens.RefreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, refreshed.AccessToken)
	require.NotEmpty(t, refreshed.RefreshToken)

	loginClaims, err := jwt.Parse(loginTokens.AccessToken)
	require.NoError(t, err)
	refreshClaims, err := jwt.Parse(refreshed.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, loginClaims.UserID, refreshClaims.UserID)
}

func TestService_UpdateProfileEmailUniqueness_Integration(t *testing.T) {
	pool := testutil.NewTestPool(t)
	testutil.ResetTables(t, pool)

	q := db.New(pool)
	jwt := NewJWTManager("integration-secret", time.Minute, time.Minute)
	svc := NewService(q, jwt, nil, nil, PasswordResetConfig{})
	ctx := context.Background()

	userA, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        "a@test.local",
		PasswordHash: "hash",
	})
	require.NoError(t, err)
	_, err = q.CreateUser(ctx, db.CreateUserParams{
		Email:        "b@test.local",
		PasswordHash: "hash",
	})
	require.NoError(t, err)

	newEmail := "b@test.local"
	_, err = svc.UpdateProfile(ctx, userA.ID, ProfileUpdate{Email: &newEmail})
	assert.ErrorIs(t, err, ErrEmailAlreadyExists)
}
