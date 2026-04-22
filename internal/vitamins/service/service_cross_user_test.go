//go:build integration

package service

import (
	"context"
	"testing"

	"vitamins-backend_2/internal/db"
	"vitamins-backend_2/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestService_UserCannotAccessOthersReminders_Integration ensures that the
// authorization scope of the vitamins use-case is by user_id: user A creates
// a reminder, and user B is not allowed to read, modify or toggle it. We rely
// on the repository's WHERE user_id = $1 clause; this test would catch a
// regression if that clause was ever dropped.
func TestService_UserCannotAccessOthersReminders_Integration(t *testing.T) {
	pool := testutil.NewTestPool(t)
	testutil.ResetTables(t, pool)

	ctx := context.Background()
	q := db.New(pool)
	svc := NewService(q, pool)

	owner, err := q.CreateUser(ctx, db.CreateUserParams{Email: "owner@test.local", PasswordHash: "hash"})
	require.NoError(t, err)
	intruder, err := q.CreateUser(ctx, db.CreateUserParams{Email: "intruder@test.local", PasswordHash: "hash"})
	require.NoError(t, err)

	name := "Vitamin C"
	form := "tablet"
	condition := "after_meal"
	created, err := svc.CreateReminder(ctx, owner.ID, CreateReminderRequest{
		Name:      &name,
		Form:      &form,
		Condition: &condition,
		Course:    CourseInput{StartDate: "2026-04-01", DurationDays: intPtr(5), Timezone: "Europe/Moscow"},
		Schedule:  ScheduleInput{Days: []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}, Times: []string{"08:00"}},
	})
	require.NoError(t, err)

	_, err = svc.GetReminder(ctx, intruder.ID, created.ID)
	assert.ErrorIs(t, err, ErrReminderNotFound, "intruder must not read someone else's reminder")

	newName := "Hijacked"
	_, err = svc.UpdateReminder(ctx, intruder.ID, created.ID, UpdateReminderRequest{Name: &newName})
	assert.Error(t, err, "intruder must not update someone else's reminder")

	_, err = svc.SetReminderActive(ctx, intruder.ID, created.ID, false)
	assert.ErrorIs(t, err, ErrReminderNotFound, "intruder must not toggle someone else's reminder")

	got, err := svc.GetReminder(ctx, owner.ID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, name, got.Name, "owner's reminder must be intact and unchanged")

	intruderList, err := svc.ListReminders(ctx, intruder.ID)
	require.NoError(t, err)
	assert.Empty(t, intruderList, "intruder's list must be empty")
}
