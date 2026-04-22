package service

import (
	"context"

	"vitamins-backend_2/internal/db"
)

// ReminderRepository is the data port for vitamins use-cases.
type ReminderRepository interface {
	ListVitaminCatalog(ctx context.Context) ([]db.VitaminCatalog, error)
	GetVitaminCatalogByID(ctx context.Context, id int64) (db.VitaminCatalog, error)

	CreateUserVitamin(ctx context.Context, params db.CreateUserVitaminParams) (db.UserVitamin, error)
	GetUserVitaminByID(ctx context.Context, params db.GetUserVitaminByIDParams) (db.UserVitamin, error)
	ListUserVitamins(ctx context.Context, userID int64) ([]db.UserVitamin, error)
	UpdateUserVitaminCore(ctx context.Context, params db.UpdateUserVitaminCoreParams) (db.UserVitamin, error)
	UpdateUserVitaminActive(ctx context.Context, params db.UpdateUserVitaminActiveParams) (db.UserVitamin, error)

	CreateVitaminCourse(ctx context.Context, params db.CreateVitaminCourseParams) (db.VitaminCourse, error)
	GetVitaminCourseByUserVitaminID(ctx context.Context, userVitaminID int64) (db.VitaminCourse, error)
	UpdateVitaminCourse(ctx context.Context, params db.UpdateVitaminCourseParams) (db.VitaminCourse, error)

	CreateIntakeSchedule(ctx context.Context, params db.CreateIntakeScheduleParams) (db.IntakeSchedule, error)
	GetIntakeScheduleByCourseID(ctx context.Context, courseID int64) (db.IntakeSchedule, error)
	UpdateIntakeSchedule(ctx context.Context, params db.UpdateIntakeScheduleParams) (db.IntakeSchedule, error)

	CreateIntakeTime(ctx context.Context, params db.CreateIntakeTimeParams) (db.IntakeTime, error)
	ListIntakeTimesByScheduleID(ctx context.Context, scheduleID int64) ([]db.IntakeTime, error)
	DeleteIntakeTimesByScheduleID(ctx context.Context, scheduleID int64) error

	CreateNotificationPreferences(ctx context.Context, params db.CreateNotificationPreferencesParams) (db.NotificationPreference, error)
	GetNotificationPreferencesByUserVitaminID(ctx context.Context, userVitaminID int64) (db.NotificationPreference, error)
	UpdateNotificationPreferences(ctx context.Context, params db.UpdateNotificationPreferencesParams) (db.NotificationPreference, error)

	UpsertNotificationOverrides(ctx context.Context, params db.UpsertNotificationOverridesParams) (db.NotificationTextOverride, error)
	GetNotificationOverridesByUserVitaminID(ctx context.Context, userVitaminID int64) (db.NotificationTextOverride, error)
}

// TxManager runs a use-case block in one DB transaction.
type TxManager interface {
	InTx(ctx context.Context, fn func(repo ReminderRepository) error) error
}
