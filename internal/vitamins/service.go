package vitamins

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"vitamins-backend_2/internal/db"
)

var (
	ErrCatalogNotFound  = errors.New("CATALOG_NOT_FOUND")
	ErrNameRequired     = errors.New("NAME_REQUIRED")
	ErrReminderNotFound = errors.New("REMINDER_NOT_FOUND")
	ErrNoFieldsToUpdate = errors.New("NO_FIELDS_TO_UPDATE")
	ErrTimezoneRequired = errors.New("TIMEZONE_REQUIRED")
)

type Service struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewService(q *db.Queries, pool *pgxpool.Pool) *Service {
	return &Service{q: q, pool: pool}
}

func (s *Service) ListCatalog(ctx context.Context) ([]CatalogItem, error) {
	items, err := s.q.ListVitaminCatalog(ctx)
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

func (s *Service) CreateReminder(ctx context.Context, userID int64, req CreateReminderRequest) (ReminderResponse, error) {
	if userID == 0 {
		return ReminderResponse{}, ErrReminderNotFound
	}
	condition, err := parseCondition(req.Condition)
	if err != nil {
		return ReminderResponse{}, err
	}
	form, err := parseForm(req.Form)
	if err != nil {
		return ReminderResponse{}, err
	}
	if strings.TrimSpace(req.Course.Timezone) == "" {
		return ReminderResponse{}, ErrTimezoneRequired
	}
	startDate, err := parseDate(req.Course.StartDate)
	if err != nil {
		return ReminderResponse{}, err
	}
	endDate, err := parseEndDate(startDate, req.Course.EndDate, req.Course.DurationDays)
	if err != nil {
		return ReminderResponse{}, err
	}
	mask, err := parseDaysMask(req.Schedule.Days)
	if err != nil {
		return ReminderResponse{}, err
	}
	times, err := validateTimes(req.Schedule.Times)
	if err != nil {
		return ReminderResponse{}, err
	}

	catalogID, name, err := s.resolveCatalog(ctx, req.CatalogID, req.Name)
	if err != nil {
		return ReminderResponse{}, err
	}

	dose := ""
	if req.Dose != nil {
		dose = strings.TrimSpace(*req.Dose)
	}
	note := ""
	if req.Note != nil {
		note = strings.TrimSpace(*req.Note)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ReminderResponse{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	uv, err := qtx.CreateUserVitamin(ctx, db.CreateUserVitaminParams{
		UserID:     userID,
		CatalogID:  catalogID,
		Name:       name,
		DosageForm: form,
		DoseValue:  dose,
		DoseUnit:   defaultUnitForForm(form),
		Condition:  condition,
		Note:       note,
		IsActive:   true,
	})
	if err != nil {
		return ReminderResponse{}, err
	}

	course, err := qtx.CreateVitaminCourse(ctx, db.CreateVitaminCourseParams{
		UserVitaminID: uv.ID,
		StartDate:     startDate,
		EndDate:       endDate,
		Timezone:      strings.TrimSpace(req.Course.Timezone),
	})
	if err != nil {
		return ReminderResponse{}, err
	}

	schedule, err := qtx.CreateIntakeSchedule(ctx, db.CreateIntakeScheduleParams{
		CourseID: course.ID,
		Type:     scheduleTypeFromMask(mask),
		DaysMask: mask,
	})
	if err != nil {
		return ReminderResponse{}, err
	}
	for i, t := range times {
		if _, err := qtx.CreateIntakeTime(ctx, db.CreateIntakeTimeParams{
			ScheduleID: schedule.ID,
			TimeOfDay:  t,
			SortOrder:  int32(i),
		}); err != nil {
			return ReminderResponse{}, err
		}
	}

	prefs, overrides := resolveNotificationDefaults(req.NotificationPreferences, req.ContentOverrides)
	if _, err := qtx.CreateNotificationPreferences(ctx, prefs.ToCreateParams(uv.ID)); err != nil {
		return ReminderResponse{}, err
	}
	if _, err := qtx.UpsertNotificationOverrides(ctx, overrides.WithUserVitaminID(uv.ID)); err != nil {
		return ReminderResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ReminderResponse{}, err
	}
	return s.GetReminder(ctx, userID, uv.ID)
}

func (s *Service) ListReminders(ctx context.Context, userID int64) ([]ReminderResponse, error) {
	items, err := s.q.ListUserVitamins(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]ReminderResponse, 0, len(items))
	for _, item := range items {
		resp, err := s.buildReminder(ctx, item)
		if err != nil {
			return nil, err
		}
		result = append(result, resp)
	}
	return result, nil
}

func (s *Service) GetReminder(ctx context.Context, userID, id int64) (ReminderResponse, error) {
	uv, err := s.q.GetUserVitaminByID(ctx, db.GetUserVitaminByIDParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ReminderResponse{}, ErrReminderNotFound
		}
		return ReminderResponse{}, err
	}
	return s.buildReminder(ctx, uv)
}

func (s *Service) UpdateReminder(ctx context.Context, userID, id int64, req UpdateReminderRequest) (ReminderResponse, error) {
	if req.Name == nil && req.Form == nil && req.Dose == nil && req.Condition == nil && req.Note == nil &&
		req.Course == nil && req.Schedule == nil && req.NotificationPreferences == nil && req.ContentOverrides == nil {
		return ReminderResponse{}, ErrNoFieldsToUpdate
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ReminderResponse{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	uv, err := qtx.GetUserVitaminByID(ctx, db.GetUserVitaminByIDParams{ID: id, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ReminderResponse{}, ErrReminderNotFound
		}
		return ReminderResponse{}, err
	}

	name := uv.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			return ReminderResponse{}, ErrNameRequired
		}
	}
	form := uv.DosageForm
	if req.Form != nil {
		if strings.TrimSpace(*req.Form) == "" {
			return ReminderResponse{}, ErrInvalidForm
		}
		f, err := parseForm(req.Form)
		if err != nil {
			return ReminderResponse{}, err
		}
		form = f
	}
	condition := uv.Condition
	if req.Condition != nil {
		c, err := parseCondition(req.Condition)
		if err != nil {
			return ReminderResponse{}, err
		}
		condition = c
	}
	dose := uv.DoseValue
	if req.Dose != nil {
		dose = strings.TrimSpace(*req.Dose)
	}
	doseUnit := uv.DoseUnit
	note := uv.Note
	if req.Note != nil {
		note = strings.TrimSpace(*req.Note)
	}
	if req.Form != nil {
		doseUnit = defaultUnitForForm(form)
	}
	if _, err := qtx.UpdateUserVitaminCore(ctx, db.UpdateUserVitaminCoreParams{
		Name:       name,
		DosageForm: form,
		DoseValue:  dose,
		DoseUnit:   doseUnit,
		Condition:  condition,
		Note:       note,
		ID:         id,
		UserID:     userID,
	}); err != nil {
		return ReminderResponse{}, err
	}

	if req.Course != nil {
		course, err := qtx.GetVitaminCourseByUserVitaminID(ctx, id)
		if err != nil {
			return ReminderResponse{}, err
		}
		startDate := course.StartDate
		if strings.TrimSpace(req.Course.StartDate) != "" {
			startDate, err = parseDate(req.Course.StartDate)
			if err != nil {
				return ReminderResponse{}, err
			}
		}
		endDate := course.EndDate
		if req.Course.EndDate != nil || req.Course.DurationDays != nil {
			endDate, err = parseEndDate(startDate, req.Course.EndDate, req.Course.DurationDays)
			if err != nil {
				return ReminderResponse{}, err
			}
		}
		timezone := course.Timezone
		if strings.TrimSpace(req.Course.Timezone) != "" {
			timezone = strings.TrimSpace(req.Course.Timezone)
		}
		if strings.TrimSpace(timezone) == "" {
			return ReminderResponse{}, ErrTimezoneRequired
		}
		if _, err := qtx.UpdateVitaminCourse(ctx, db.UpdateVitaminCourseParams{
			StartDate:     startDate,
			EndDate:       endDate,
			Timezone:      timezone,
			UserVitaminID: id,
		}); err != nil {
			return ReminderResponse{}, err
		}
	}

	if req.Schedule != nil {
		mask, err := parseDaysMask(req.Schedule.Days)
		if err != nil {
			return ReminderResponse{}, err
		}
		times, err := validateTimes(req.Schedule.Times)
		if err != nil {
			return ReminderResponse{}, err
		}
		course, err := qtx.GetVitaminCourseByUserVitaminID(ctx, id)
		if err != nil {
			return ReminderResponse{}, err
		}
		if _, err := qtx.UpdateIntakeSchedule(ctx, db.UpdateIntakeScheduleParams{
			Type:     scheduleTypeFromMask(mask),
			DaysMask: mask,
			CourseID: course.ID,
		}); err != nil {
			return ReminderResponse{}, err
		}
		schedule, err := qtx.GetIntakeScheduleByCourseID(ctx, course.ID)
		if err != nil {
			return ReminderResponse{}, err
		}
		if err := qtx.DeleteIntakeTimesByScheduleID(ctx, schedule.ID); err != nil {
			return ReminderResponse{}, err
		}
		for i, t := range times {
			if _, err := qtx.CreateIntakeTime(ctx, db.CreateIntakeTimeParams{
				ScheduleID: schedule.ID,
				TimeOfDay:  t,
				SortOrder:  int32(i),
			}); err != nil {
				return ReminderResponse{}, err
			}
		}
	}

	if req.NotificationPreferences != nil || req.ContentOverrides != nil {
		currentPrefs, err := qtx.GetNotificationPreferencesByUserVitaminID(ctx, id)
		if err != nil {
			return ReminderResponse{}, err
		}
		nextPrefs := mergeNotificationPrefs(req.NotificationPreferences, currentPrefs)
		if _, err := qtx.UpdateNotificationPreferences(ctx, nextPrefs.ToUpdateParams(id)); err != nil {
			return ReminderResponse{}, err
		}
		currentOverrides, err := qtx.GetNotificationOverridesByUserVitaminID(ctx, id)
		if err != nil {
			return ReminderResponse{}, err
		}
		nextOverrides := mergeNotificationOverrides(req.ContentOverrides, currentOverrides)
		if _, err := qtx.UpsertNotificationOverrides(ctx, nextOverrides.WithUserVitaminID(id)); err != nil {
			return ReminderResponse{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return ReminderResponse{}, err
	}
	return s.GetReminder(ctx, userID, id)
}

func (s *Service) SetReminderActive(ctx context.Context, userID, id int64, active bool) (ReminderResponse, error) {
	_, err := s.q.UpdateUserVitaminActive(ctx, db.UpdateUserVitaminActiveParams{
		IsActive: active,
		ID:       id,
		UserID:   userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ReminderResponse{}, ErrReminderNotFound
		}
		return ReminderResponse{}, err
	}
	return s.GetReminder(ctx, userID, id)
}

func (s *Service) resolveCatalog(ctx context.Context, catalogID *int64, name *string) (pgtype.Int8, string, error) {
	if catalogID != nil {
		item, err := s.q.GetVitaminCatalogByID(ctx, *catalogID)
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

func (s *Service) buildReminder(ctx context.Context, uv db.UserVitamin) (ReminderResponse, error) {
	course, err := s.q.GetVitaminCourseByUserVitaminID(ctx, uv.ID)
	if err != nil {
		return ReminderResponse{}, err
	}
	schedule, err := s.q.GetIntakeScheduleByCourseID(ctx, course.ID)
	if err != nil {
		return ReminderResponse{}, err
	}
	times, err := s.q.ListIntakeTimesByScheduleID(ctx, schedule.ID)
	if err != nil {
		return ReminderResponse{}, err
	}
	prefs, err := s.q.GetNotificationPreferencesByUserVitaminID(ctx, uv.ID)
	if err != nil {
		return ReminderResponse{}, err
	}
	overrides, err := s.q.GetNotificationOverridesByUserVitaminID(ctx, uv.ID)
	if err != nil {
		return ReminderResponse{}, err
	}

	var catalog *CatalogItem
	var catalogID *int64
	if uv.CatalogID.Valid {
		catalogID = &uv.CatalogID.Int64
		item, err := s.q.GetVitaminCatalogByID(ctx, uv.CatalogID.Int64)
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

type notificationPrefsInput struct {
	IncludeDose              bool
	IncludeFrequency         bool
	IncludeInteraction       bool
	IncludeCompatibility     bool
	IncludeCondition         bool
	IncludeContraindications bool
}

func (p notificationPrefsInput) ToCreateParams(id int64) db.CreateNotificationPreferencesParams {
	return db.CreateNotificationPreferencesParams{
		UserVitaminID:            id,
		IncludeDose:              p.IncludeDose,
		IncludeFrequency:         p.IncludeFrequency,
		IncludeInteraction:       p.IncludeInteraction,
		IncludeCompatibility:     p.IncludeCompatibility,
		IncludeCondition:         p.IncludeCondition,
		IncludeContraindications: p.IncludeContraindications,
	}
}

func (p notificationPrefsInput) ToUpdateParams(id int64) db.UpdateNotificationPreferencesParams {
	return db.UpdateNotificationPreferencesParams{
		IncludeDose:              p.IncludeDose,
		IncludeFrequency:         p.IncludeFrequency,
		IncludeInteraction:       p.IncludeInteraction,
		IncludeCompatibility:     p.IncludeCompatibility,
		IncludeCondition:         p.IncludeCondition,
		IncludeContraindications: p.IncludeContraindications,
		UserVitaminID:            id,
	}
}

type notificationOverridesInput struct {
	InteractionTextOverride       pgtype.Text
	CompatibilityTextOverride     pgtype.Text
	ContraindicationsTextOverride pgtype.Text
}

func (o notificationOverridesInput) WithUserVitaminID(id int64) db.UpsertNotificationOverridesParams {
	return db.UpsertNotificationOverridesParams{
		UserVitaminID:                 id,
		InteractionTextOverride:       o.InteractionTextOverride,
		CompatibilityTextOverride:     o.CompatibilityTextOverride,
		ContraindicationsTextOverride: o.ContraindicationsTextOverride,
	}
}

func resolveNotificationDefaults(prefs NotificationPreferencesInput, overrides ContentOverridesInput) (notificationPrefsInput, notificationOverridesInput) {
	prefsInput := notificationPrefsInput{
		IncludeDose:              boolOrDefault(prefs.IncludeDose, true),
		IncludeFrequency:         boolOrDefault(prefs.IncludeFrequency, true),
		IncludeInteraction:       boolOrDefault(prefs.IncludeInteraction, true),
		IncludeCompatibility:     boolOrDefault(prefs.IncludeCompatibility, true),
		IncludeCondition:         boolOrDefault(prefs.IncludeCondition, true),
		IncludeContraindications: boolOrDefault(prefs.IncludeContraindications, true),
	}
	overrideValues := notificationOverridesInput{
		InteractionTextOverride:       overrideFromPtr(overrides.InteractionTextOverride),
		CompatibilityTextOverride:     overrideFromPtr(overrides.CompatibilityTextOverride),
		ContraindicationsTextOverride: overrideFromPtr(overrides.ContraindicationsTextOverride),
	}
	return prefsInput, overrideValues
}

func mergeNotificationPrefs(input *NotificationPreferencesInput, current db.NotificationPreference) notificationPrefsInput {
	if input == nil {
		return notificationPrefsInput{
			IncludeDose:              current.IncludeDose,
			IncludeFrequency:         current.IncludeFrequency,
			IncludeInteraction:       current.IncludeInteraction,
			IncludeCompatibility:     current.IncludeCompatibility,
			IncludeCondition:         current.IncludeCondition,
			IncludeContraindications: current.IncludeContraindications,
		}
	}
	return notificationPrefsInput{
		IncludeDose:              boolOrDefault(input.IncludeDose, current.IncludeDose),
		IncludeFrequency:         boolOrDefault(input.IncludeFrequency, current.IncludeFrequency),
		IncludeInteraction:       boolOrDefault(input.IncludeInteraction, current.IncludeInteraction),
		IncludeCompatibility:     boolOrDefault(input.IncludeCompatibility, current.IncludeCompatibility),
		IncludeCondition:         boolOrDefault(input.IncludeCondition, current.IncludeCondition),
		IncludeContraindications: boolOrDefault(input.IncludeContraindications, current.IncludeContraindications),
	}
}

func mergeNotificationOverrides(input *ContentOverridesInput, current db.NotificationTextOverride) notificationOverridesInput {
	if input == nil {
		return notificationOverridesInput{
			InteractionTextOverride:       current.InteractionTextOverride,
			CompatibilityTextOverride:     current.CompatibilityTextOverride,
			ContraindicationsTextOverride: current.ContraindicationsTextOverride,
		}
	}
	return notificationOverridesInput{
		InteractionTextOverride:       mergeOverride(input.InteractionTextOverride, current.InteractionTextOverride),
		CompatibilityTextOverride:     mergeOverride(input.CompatibilityTextOverride, current.CompatibilityTextOverride),
		ContraindicationsTextOverride: mergeOverride(input.ContraindicationsTextOverride, current.ContraindicationsTextOverride),
	}
}

func conditionPtr(v pgtype.Int2) *string {
	if !v.Valid {
		return nil
	}
	s := conditionToString(v.Int16)
	return &s
}

func overrideFromPtr(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: trimmed, Valid: true}
}

func mergeOverride(newValue *string, fallback pgtype.Text) pgtype.Text {
	if newValue == nil {
		return fallback
	}
	return overrideFromPtr(newValue)
}
