//go:build integration

package service

import (
	"context"
	"testing"
	"time"

	"vitamins-backend_2/pkg/db"
	"vitamins-backend_2/pkg/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_CreateUpdateListReminder_Integration(t *testing.T) {
	pool := testutil.NewTestPool(t)
	testutil.ResetTables(t, pool)

	ctx := context.Background()
	q := db.New(pool)
	svc := NewService(q, pool)

	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        "vitamins-user@test.local",
		PasswordHash: "hash",
	})
	require.NoError(t, err)

	var catalogID int64
	err = pool.QueryRow(ctx, `
INSERT INTO vitamin_catalog(code, display_name, default_unit, default_condition)
VALUES ('VITAMIN_D', 'Vitamin D', 'IU', 2)
RETURNING id
`).Scan(&catalogID)
	require.NoError(t, err)

	name := "Vitamin D3"
	form := "capsule"
	condition := "after_meal"
	dose := "1000"
	note := "daily"

	created, err := svc.CreateReminder(ctx, user.ID, CreateReminderRequest{
		CatalogID: &catalogID,
		Name:      &name,
		Form:      &form,
		Condition: &condition,
		Dose:      &dose,
		Note:      &note,
		Course:    CourseInput{StartDate: "2026-04-01", DurationDays: intPtr(10), Timezone: "Europe/Moscow"},
		Schedule:  ScheduleInput{Days: []string{"mon", "wed", "fri"}, Times: []string{"08:00", "20:30"}},
		NotificationPreferences: NotificationPreferencesInput{
			IncludeInteraction: boolPtr(false),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Name)
	assert.Equal(t, "capsule", created.Form)
	require.NotNil(t, created.CatalogID)
	assert.Equal(t, catalogID, *created.CatalogID)
	assert.Equal(t, []string{"08:00", "20:30"}, created.Schedule.Times)

	reminders, err := svc.ListReminders(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, reminders, 1)
	assert.Equal(t, created.ID, reminders[0].ID)

	newForm := "liquid"
	newTimes := []string{"09:15"}
	updated, err := svc.UpdateReminder(ctx, user.ID, created.ID, UpdateReminderRequest{
		Form: &newForm,
		Schedule: &ScheduleInput{
			Days:  []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"},
			Times: newTimes,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "liquid", updated.Form)
	assert.Equal(t, newTimes, updated.Schedule.Times)

	stored, err := q.GetUserVitaminByID(ctx, db.GetUserVitaminByIDParams{ID: created.ID, UserID: user.ID})
	require.NoError(t, err)
	assert.Equal(t, "мл", stored.DoseUnit)
}

func TestService_SetReminderActive_Integration(t *testing.T) {
	pool := testutil.NewTestPool(t)
	testutil.ResetTables(t, pool)

	ctx := context.Background()
	q := db.New(pool)
	svc := NewService(q, pool)

	user, err := q.CreateUser(ctx, db.CreateUserParams{Email: "active@test.local", PasswordHash: "hash"})
	require.NoError(t, err)

	name := "Magnesium"
	form := "tablet"
	condition := "after_meal"
	created, err := svc.CreateReminder(ctx, user.ID, CreateReminderRequest{
		Name:      &name,
		Form:      &form,
		Condition: &condition,
		Course:    CourseInput{StartDate: time.Now().UTC().Format("2006-01-02"), DurationDays: intPtr(7), Timezone: "Europe/Moscow"},
		Schedule:  ScheduleInput{Days: []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}, Times: []string{"10:00"}},
	})
	require.NoError(t, err)

	disabled, err := svc.SetReminderActive(ctx, user.ID, created.ID, false)
	require.NoError(t, err)
	assert.False(t, disabled.IsActive)

	_, err = svc.SetReminderActive(ctx, user.ID, 999999, true)
	require.ErrorIs(t, err, ErrReminderNotFound)
}

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }
