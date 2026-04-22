package service

import (
	"context"
	"errors"
	"testing"

	"vitamins-backend_2/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_ListCatalog(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		listVitaminCatalogFn: func(ctx context.Context) ([]db.VitaminCatalog, error) {
			return []db.VitaminCatalog{{
				ID:                1,
				DisplayName:       "Vitamin D",
				Code:              pgtype.Text{String: "VITAMIN_D", Valid: true},
				DefaultUnit:       pgtype.Text{String: "IU", Valid: true},
				DefaultCondition:  pgtype.Int2{Int16: 2, Valid: true},
				InteractionText:   pgtype.Text{String: "int", Valid: true},
				CompatibilityText: pgtype.Text{String: "comp", Valid: true},
			}}, nil
		},
	}
	svc := NewServiceWithDeps(repo, noopTx{})

	items, err := svc.ListCatalog(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Vitamin D", items[0].DisplayName)
	require.NotNil(t, items[0].Code)
	assert.Equal(t, "VITAMIN_D", *items[0].Code)
	require.NotNil(t, items[0].DefaultCondition)
	assert.Equal(t, "after_meal", *items[0].DefaultCondition)
}

func TestService_CreateReminder_ValidatesTimezoneBeforeTx(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{}
	tx := &recordTx{}
	svc := NewServiceWithDeps(repo, tx)

	name := "Vitamin D"
	form := "tablet"
	condition := "after_meal"
	_, err := svc.CreateReminder(context.Background(), 10, CreateReminderRequest{
		Name:      &name,
		Form:      &form,
		Condition: &condition,
		Course: CourseInput{
			StartDate: "2026-01-01",
			Timezone:  "",
		},
		Schedule: ScheduleInput{
			Days:  []string{"mon"},
			Times: []string{"08:00"},
		},
	})

	require.ErrorIs(t, err, ErrTimezoneRequired)
	assert.False(t, tx.called)
}

func TestService_UpdateReminder_NoFieldsToUpdate(t *testing.T) {
	t.Parallel()

	svc := NewServiceWithDeps(&stubRepo{}, &recordTx{})
	_, err := svc.UpdateReminder(context.Background(), 1, 2, UpdateReminderRequest{})
	require.ErrorIs(t, err, ErrNoFieldsToUpdate)
}

func TestService_SetReminderActive_NotFound(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		updateUserVitaminActiveFn: func(ctx context.Context, params db.UpdateUserVitaminActiveParams) (db.UserVitamin, error) {
			return db.UserVitamin{}, pgx.ErrNoRows
		},
	}
	svc := NewServiceWithDeps(repo, noopTx{})

	_, err := svc.SetReminderActive(context.Background(), 1, 77, true)
	require.ErrorIs(t, err, ErrReminderNotFound)
}

type recordTx struct {
	called bool
}

func (t *recordTx) InTx(ctx context.Context, fn func(repo ReminderRepository) error) error {
	t.called = true
	return fn(&stubRepo{})
}

type noopTx struct{}

func (noopTx) InTx(ctx context.Context, fn func(repo ReminderRepository) error) error {
	return fn(&stubRepo{})
}

type stubRepo struct {
	listVitaminCatalogFn                        func(ctx context.Context) ([]db.VitaminCatalog, error)
	getVitaminCatalogByIDFn                     func(ctx context.Context, id int64) (db.VitaminCatalog, error)
	createUserVitaminFn                         func(ctx context.Context, params db.CreateUserVitaminParams) (db.UserVitamin, error)
	getUserVitaminByIDFn                        func(ctx context.Context, params db.GetUserVitaminByIDParams) (db.UserVitamin, error)
	listUserVitaminsFn                          func(ctx context.Context, userID int64) ([]db.UserVitamin, error)
	updateUserVitaminCoreFn                     func(ctx context.Context, params db.UpdateUserVitaminCoreParams) (db.UserVitamin, error)
	updateUserVitaminActiveFn                   func(ctx context.Context, params db.UpdateUserVitaminActiveParams) (db.UserVitamin, error)
	createVitaminCourseFn                       func(ctx context.Context, params db.CreateVitaminCourseParams) (db.VitaminCourse, error)
	getVitaminCourseByUserVitaminIDFn           func(ctx context.Context, userVitaminID int64) (db.VitaminCourse, error)
	updateVitaminCourseFn                       func(ctx context.Context, params db.UpdateVitaminCourseParams) (db.VitaminCourse, error)
	createIntakeScheduleFn                      func(ctx context.Context, params db.CreateIntakeScheduleParams) (db.IntakeSchedule, error)
	getIntakeScheduleByCourseIDFn               func(ctx context.Context, courseID int64) (db.IntakeSchedule, error)
	updateIntakeScheduleFn                      func(ctx context.Context, params db.UpdateIntakeScheduleParams) (db.IntakeSchedule, error)
	createIntakeTimeFn                          func(ctx context.Context, params db.CreateIntakeTimeParams) (db.IntakeTime, error)
	listIntakeTimesByScheduleIDFn               func(ctx context.Context, scheduleID int64) ([]db.IntakeTime, error)
	deleteIntakeTimesByScheduleIDFn             func(ctx context.Context, scheduleID int64) error
	createNotificationPreferencesFn             func(ctx context.Context, params db.CreateNotificationPreferencesParams) (db.NotificationPreference, error)
	getNotificationPreferencesByUserVitaminIDFn func(ctx context.Context, userVitaminID int64) (db.NotificationPreference, error)
	updateNotificationPreferencesFn             func(ctx context.Context, params db.UpdateNotificationPreferencesParams) (db.NotificationPreference, error)
	upsertNotificationOverridesFn               func(ctx context.Context, params db.UpsertNotificationOverridesParams) (db.NotificationTextOverride, error)
	getNotificationOverridesByUserVitaminIDFn   func(ctx context.Context, userVitaminID int64) (db.NotificationTextOverride, error)
}

func (s *stubRepo) ListVitaminCatalog(ctx context.Context) ([]db.VitaminCatalog, error) {
	if s.listVitaminCatalogFn != nil {
		return s.listVitaminCatalogFn(ctx)
	}
	return nil, nil
}

func (s *stubRepo) GetVitaminCatalogByID(ctx context.Context, id int64) (db.VitaminCatalog, error) {
	if s.getVitaminCatalogByIDFn != nil {
		return s.getVitaminCatalogByIDFn(ctx, id)
	}
	return db.VitaminCatalog{}, errors.New("unexpected call GetVitaminCatalogByID")
}

func (s *stubRepo) CreateUserVitamin(ctx context.Context, params db.CreateUserVitaminParams) (db.UserVitamin, error) {
	if s.createUserVitaminFn != nil {
		return s.createUserVitaminFn(ctx, params)
	}
	return db.UserVitamin{}, errors.New("unexpected call CreateUserVitamin")
}

func (s *stubRepo) GetUserVitaminByID(ctx context.Context, params db.GetUserVitaminByIDParams) (db.UserVitamin, error) {
	if s.getUserVitaminByIDFn != nil {
		return s.getUserVitaminByIDFn(ctx, params)
	}
	return db.UserVitamin{}, errors.New("unexpected call GetUserVitaminByID")
}

func (s *stubRepo) ListUserVitamins(ctx context.Context, userID int64) ([]db.UserVitamin, error) {
	if s.listUserVitaminsFn != nil {
		return s.listUserVitaminsFn(ctx, userID)
	}
	return nil, nil
}

func (s *stubRepo) UpdateUserVitaminCore(ctx context.Context, params db.UpdateUserVitaminCoreParams) (db.UserVitamin, error) {
	if s.updateUserVitaminCoreFn != nil {
		return s.updateUserVitaminCoreFn(ctx, params)
	}
	return db.UserVitamin{}, errors.New("unexpected call UpdateUserVitaminCore")
}

func (s *stubRepo) UpdateUserVitaminActive(ctx context.Context, params db.UpdateUserVitaminActiveParams) (db.UserVitamin, error) {
	if s.updateUserVitaminActiveFn != nil {
		return s.updateUserVitaminActiveFn(ctx, params)
	}
	return db.UserVitamin{}, errors.New("unexpected call UpdateUserVitaminActive")
}

func (s *stubRepo) CreateVitaminCourse(ctx context.Context, params db.CreateVitaminCourseParams) (db.VitaminCourse, error) {
	if s.createVitaminCourseFn != nil {
		return s.createVitaminCourseFn(ctx, params)
	}
	return db.VitaminCourse{}, errors.New("unexpected call CreateVitaminCourse")
}

func (s *stubRepo) GetVitaminCourseByUserVitaminID(ctx context.Context, userVitaminID int64) (db.VitaminCourse, error) {
	if s.getVitaminCourseByUserVitaminIDFn != nil {
		return s.getVitaminCourseByUserVitaminIDFn(ctx, userVitaminID)
	}
	return db.VitaminCourse{}, errors.New("unexpected call GetVitaminCourseByUserVitaminID")
}

func (s *stubRepo) UpdateVitaminCourse(ctx context.Context, params db.UpdateVitaminCourseParams) (db.VitaminCourse, error) {
	if s.updateVitaminCourseFn != nil {
		return s.updateVitaminCourseFn(ctx, params)
	}
	return db.VitaminCourse{}, errors.New("unexpected call UpdateVitaminCourse")
}

func (s *stubRepo) CreateIntakeSchedule(ctx context.Context, params db.CreateIntakeScheduleParams) (db.IntakeSchedule, error) {
	if s.createIntakeScheduleFn != nil {
		return s.createIntakeScheduleFn(ctx, params)
	}
	return db.IntakeSchedule{}, errors.New("unexpected call CreateIntakeSchedule")
}

func (s *stubRepo) GetIntakeScheduleByCourseID(ctx context.Context, courseID int64) (db.IntakeSchedule, error) {
	if s.getIntakeScheduleByCourseIDFn != nil {
		return s.getIntakeScheduleByCourseIDFn(ctx, courseID)
	}
	return db.IntakeSchedule{}, errors.New("unexpected call GetIntakeScheduleByCourseID")
}

func (s *stubRepo) UpdateIntakeSchedule(ctx context.Context, params db.UpdateIntakeScheduleParams) (db.IntakeSchedule, error) {
	if s.updateIntakeScheduleFn != nil {
		return s.updateIntakeScheduleFn(ctx, params)
	}
	return db.IntakeSchedule{}, errors.New("unexpected call UpdateIntakeSchedule")
}

func (s *stubRepo) CreateIntakeTime(ctx context.Context, params db.CreateIntakeTimeParams) (db.IntakeTime, error) {
	if s.createIntakeTimeFn != nil {
		return s.createIntakeTimeFn(ctx, params)
	}
	return db.IntakeTime{}, errors.New("unexpected call CreateIntakeTime")
}

func (s *stubRepo) ListIntakeTimesByScheduleID(ctx context.Context, scheduleID int64) ([]db.IntakeTime, error) {
	if s.listIntakeTimesByScheduleIDFn != nil {
		return s.listIntakeTimesByScheduleIDFn(ctx, scheduleID)
	}
	return nil, errors.New("unexpected call ListIntakeTimesByScheduleID")
}

func (s *stubRepo) DeleteIntakeTimesByScheduleID(ctx context.Context, scheduleID int64) error {
	if s.deleteIntakeTimesByScheduleIDFn != nil {
		return s.deleteIntakeTimesByScheduleIDFn(ctx, scheduleID)
	}
	return errors.New("unexpected call DeleteIntakeTimesByScheduleID")
}

func (s *stubRepo) CreateNotificationPreferences(ctx context.Context, params db.CreateNotificationPreferencesParams) (db.NotificationPreference, error) {
	if s.createNotificationPreferencesFn != nil {
		return s.createNotificationPreferencesFn(ctx, params)
	}
	return db.NotificationPreference{}, errors.New("unexpected call CreateNotificationPreferences")
}

func (s *stubRepo) GetNotificationPreferencesByUserVitaminID(ctx context.Context, userVitaminID int64) (db.NotificationPreference, error) {
	if s.getNotificationPreferencesByUserVitaminIDFn != nil {
		return s.getNotificationPreferencesByUserVitaminIDFn(ctx, userVitaminID)
	}
	return db.NotificationPreference{}, errors.New("unexpected call GetNotificationPreferencesByUserVitaminID")
}

func (s *stubRepo) UpdateNotificationPreferences(ctx context.Context, params db.UpdateNotificationPreferencesParams) (db.NotificationPreference, error) {
	if s.updateNotificationPreferencesFn != nil {
		return s.updateNotificationPreferencesFn(ctx, params)
	}
	return db.NotificationPreference{}, errors.New("unexpected call UpdateNotificationPreferences")
}

func (s *stubRepo) UpsertNotificationOverrides(ctx context.Context, params db.UpsertNotificationOverridesParams) (db.NotificationTextOverride, error) {
	if s.upsertNotificationOverridesFn != nil {
		return s.upsertNotificationOverridesFn(ctx, params)
	}
	return db.NotificationTextOverride{}, errors.New("unexpected call UpsertNotificationOverrides")
}

func (s *stubRepo) GetNotificationOverridesByUserVitaminID(ctx context.Context, userVitaminID int64) (db.NotificationTextOverride, error) {
	if s.getNotificationOverridesByUserVitaminIDFn != nil {
		return s.getNotificationOverridesByUserVitaminIDFn(ctx, userVitaminID)
	}
	return db.NotificationTextOverride{}, errors.New("unexpected call GetNotificationOverridesByUserVitaminID")
}
