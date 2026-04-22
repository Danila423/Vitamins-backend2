// Package service implements the vitamin reminders use-case (catalog,
// per-user reminders with their dosing schedules and notification settings).
//
// The service layer is split for readability into a few files inside this
// same package:
//
//   - service.go         — `Service` type, constructors, the catalog
//     listing and the response-building helpers shared by all reminder
//     methods. This is the entry point.
//   - reminders.go       — Create/List/Get/Update/SetActive operations.
//   - notifications.go   — internal types and helpers for merging
//     notification preferences and content overrides.
//
// The HTTP-layer JSON shapes (see types.go) are part of the public API used
// by the frontend; do not rename fields without coordinating a migration.
package service

import (
	"context"
	"errors"
	"strings"

	"vitamins-backend_2/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrCatalogNotFound  = errors.New("CATALOG_NOT_FOUND")
	ErrNameRequired     = errors.New("NAME_REQUIRED")
	ErrReminderNotFound = errors.New("REMINDER_NOT_FOUND")
	ErrNoFieldsToUpdate = errors.New("NO_FIELDS_TO_UPDATE")
	ErrTimezoneRequired = errors.New("TIMEZONE_REQUIRED")
)

// ServiceConfig groups tunable options for the vitamins use-case service.
type ServiceConfig struct { //nolint:revive // renaming would break the public API
	// ListParallelism caps the number of concurrent reminder builds inside
	// ListReminders. Zero or negative falls back to defaultListParallelism.
	ListParallelism int
}

const defaultListParallelism = 8

type Service struct {
	repo            ReminderRepository
	tx              TxManager
	listParallelism int
}

func NewService(q *db.Queries, pool *pgxpool.Pool) *Service {
	return NewServiceWithConfig(q, pool, ServiceConfig{})
}

// NewServiceWithConfig wires the default sqlc-backed repository with explicit
// configuration. Currently only ListParallelism is honored; more knobs may be
// added without changing the constructor signature.
func NewServiceWithConfig(q *db.Queries, pool *pgxpool.Pool, cfg ServiceConfig) *Service {
	repo := NewRepository(q, pool)
	s := NewServiceWithDeps(repo, repo)
	if cfg.ListParallelism > 0 {
		s.listParallelism = cfg.ListParallelism
	}
	return s
}

func NewServiceWithDeps(repo ReminderRepository, tx TxManager) *Service {
	return &Service{repo: repo, tx: tx, listParallelism: defaultListParallelism}
}

func (s *Service) ListCatalog(ctx context.Context) ([]CatalogItem, error) {
	items, err := s.repo.ListVitaminCatalog(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]CatalogItem, 0, len(items))
	for _, item := range items {
		result = append(result, CatalogItem{
			ID:                    item.ID,
			Code:                  textToPtr(item.Code),
			DisplayName:           item.DisplayName,
			DefaultUnit:           textToPtr(item.DefaultUnit),
			InteractionText:       textToPtr(item.InteractionText),
			CompatibilityText:     textToPtr(item.CompatibilityText),
			ContraindicationsText: textToPtr(item.ContraindicationsText),
			DefaultCondition:      conditionPtr(item.DefaultCondition),
		})
	}
	return result, nil
}

// resolveCatalog turns the user-provided (catalogID, name) pair into a
// concrete (catalogID pgtype.Int8, name string) ready for sqlc.
func (s *Service) resolveCatalog(ctx context.Context, catalogID *int64, name *string) (pgtype.Int8, string, error) {
	if catalogID != nil {
		item, err := s.repo.GetVitaminCatalogByID(ctx, *catalogID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return pgtype.Int8{}, "", ErrCatalogNotFound
			}
			return pgtype.Int8{}, "", err
		}
		finalName := ""
		if name != nil {
			finalName = strings.TrimSpace(*name)
		}
		if finalName == "" {
			finalName = item.DisplayName
		}
		if finalName == "" {
			return pgtype.Int8{}, "", ErrNameRequired
		}
		return pgtype.Int8{Int64: *catalogID, Valid: true}, finalName, nil
	}
	finalName := ""
	if name != nil {
		finalName = strings.TrimSpace(*name)
	}
	if finalName == "" {
		return pgtype.Int8{}, "", ErrNameRequired
	}
	return pgtype.Int8{}, finalName, nil
}

// buildReminder is the convenience wrapper that builds a response from the
// non-transactional repository.
func (s *Service) buildReminder(ctx context.Context, uv db.UserVitamin) (ReminderResponse, error) {
	return s.buildReminderWith(ctx, s.repo, uv)
}

// buildReminderWith assembles the response using the provided repository, which
// may be a transactional one. This lets Create/Update return data committed
// inside the same transaction without an extra round-trip read after commit.
func (s *Service) buildReminderWith(ctx context.Context, repo ReminderRepository, uv db.UserVitamin) (ReminderResponse, error) {
	course, err := repo.GetVitaminCourseByUserVitaminID(ctx, uv.ID)
	if err != nil {
		return ReminderResponse{}, err
	}
	schedule, err := repo.GetIntakeScheduleByCourseID(ctx, course.ID)
	if err != nil {
		return ReminderResponse{}, err
	}
	times, err := repo.ListIntakeTimesByScheduleID(ctx, schedule.ID)
	if err != nil {
		return ReminderResponse{}, err
	}
	prefs, err := repo.GetNotificationPreferencesByUserVitaminID(ctx, uv.ID)
	if err != nil {
		return ReminderResponse{}, err
	}
	overrides, err := repo.GetNotificationOverridesByUserVitaminID(ctx, uv.ID)
	if err != nil {
		return ReminderResponse{}, err
	}

	var catalog *CatalogItem
	var catalogID *int64
	if uv.CatalogID.Valid {
		catalogID = &uv.CatalogID.Int64
		item, err := repo.GetVitaminCatalogByID(ctx, uv.CatalogID.Int64)
		if err != nil {
			return ReminderResponse{}, err
		}
		catalog = &CatalogItem{
			ID:                    item.ID,
			Code:                  textToPtr(item.Code),
			DisplayName:           item.DisplayName,
			DefaultUnit:           textToPtr(item.DefaultUnit),
			InteractionText:       textToPtr(item.InteractionText),
			CompatibilityText:     textToPtr(item.CompatibilityText),
			ContraindicationsText: textToPtr(item.ContraindicationsText),
			DefaultCondition:      conditionPtr(item.DefaultCondition),
		}
	}

	timeStrings := make([]string, 0, len(times))
	for _, t := range times {
		timeStrings = append(timeStrings, formatTimeOfDay(t.TimeOfDay))
	}

	return ReminderResponse{
		ID:        uv.ID,
		CatalogID: catalogID,
		Name:      uv.Name,
		Form:      formToString(uv.DosageForm),
		Dose:      uv.DoseValue,
		Condition: conditionToString(uv.Condition),
		Note:      uv.Note,
		IsActive:  uv.IsActive,
		Catalog:   catalog,
		Course: CourseResponse{
			StartDate: course.StartDate.Time.Format("2006-01-02"),
			EndDate:   dateToString(course.EndDate),
			Timezone:  course.Timezone,
		},
		Schedule: ScheduleResponse{
			Type:  scheduleTypeToString(schedule.Type),
			Days:  daysFromMask(schedule.DaysMask),
			Times: timeStrings,
		},
		NotificationPreferences: NotificationPreferencesResponse{
			IncludeDose:              prefs.IncludeDose,
			IncludeFrequency:         prefs.IncludeFrequency,
			IncludeInteraction:       prefs.IncludeInteraction,
			IncludeCompatibility:     prefs.IncludeCompatibility,
			IncludeCondition:         prefs.IncludeCondition,
			IncludeContraindications: prefs.IncludeContraindications,
		},
		ContentOverrides: ContentOverridesResponse{
			InteractionTextOverride:       textToPtr(overrides.InteractionTextOverride),
			CompatibilityTextOverride:     textToPtr(overrides.CompatibilityTextOverride),
			ContraindicationsTextOverride: textToPtr(overrides.ContraindicationsTextOverride),
		},
	}, nil
}

func conditionPtr(v pgtype.Int2) *string {
	if !v.Valid {
		return nil
	}
	s := conditionToString(v.Int16)
	return &s
}
