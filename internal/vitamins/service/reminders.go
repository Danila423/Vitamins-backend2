package service

import (
	"context"
	"errors"
	"strings"

	"vitamins-backend_2/internal/db"

	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"
)

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

	var built ReminderResponse
	if err := s.tx.InTx(ctx, func(txRepo ReminderRepository) error {
		uv, err := txRepo.CreateUserVitamin(ctx, db.CreateUserVitaminParams{
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
			return err
		}

		course, err := txRepo.CreateVitaminCourse(ctx, db.CreateVitaminCourseParams{
			UserVitaminID: uv.ID,
			StartDate:     startDate,
			EndDate:       endDate,
			Timezone:      strings.TrimSpace(req.Course.Timezone),
		})
		if err != nil {
			return err
		}

		schedule, err := txRepo.CreateIntakeSchedule(ctx, db.CreateIntakeScheduleParams{
			CourseID: course.ID,
			Type:     scheduleTypeFromMask(mask),
			DaysMask: mask,
		})
		if err != nil {
			return err
		}
		for i, t := range times {
			if _, err := txRepo.CreateIntakeTime(ctx, db.CreateIntakeTimeParams{
				ScheduleID: schedule.ID,
				TimeOfDay:  t,
				SortOrder:  int32(i),
			}); err != nil {
				return err
			}
		}

		prefs, overrides := resolveNotificationDefaults(req.NotificationPreferences, req.ContentOverrides)
		if _, err := txRepo.CreateNotificationPreferences(ctx, prefs.ToCreateParams(uv.ID)); err != nil {
			return err
		}
		if _, err := txRepo.UpsertNotificationOverrides(ctx, overrides.WithUserVitaminID(uv.ID)); err != nil {
			return err
		}
		resp, err := s.buildReminderWith(ctx, txRepo, uv)
		if err != nil {
			return err
		}
		built = resp
		return nil
	}); err != nil {
		return ReminderResponse{}, err
	}
	return built, nil
}

func (s *Service) ListReminders(ctx context.Context, userID int64) ([]ReminderResponse, error) {
	items, err := s.repo.ListUserVitamins(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []ReminderResponse{}, nil
	}

	result := make([]ReminderResponse, len(items))
	g, gctx := errgroup.WithContext(ctx)
	limit := s.listParallelism
	if limit <= 0 {
		limit = defaultListParallelism
	}
	g.SetLimit(limit)

	for i := range items {
		i := i
		item := items[i]
		g.Go(func() error {
			resp, err := s.buildReminder(gctx, item)
			if err != nil {
				return err
			}
			result[i] = resp
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) GetReminder(ctx context.Context, userID, id int64) (ReminderResponse, error) {
	uv, err := s.repo.GetUserVitaminByID(ctx, db.GetUserVitaminByIDParams{
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

	var built ReminderResponse
	if err := s.tx.InTx(ctx, func(txRepo ReminderRepository) error {
		uv, err := txRepo.GetUserVitaminByID(ctx, db.GetUserVitaminByIDParams{ID: id, UserID: userID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrReminderNotFound
			}
			return err
		}

		name := uv.Name
		if req.Name != nil {
			name = strings.TrimSpace(*req.Name)
			if name == "" {
				return ErrNameRequired
			}
		}
		form := uv.DosageForm
		if req.Form != nil {
			if strings.TrimSpace(*req.Form) == "" {
				return ErrInvalidForm
			}
			f, err := parseForm(req.Form)
			if err != nil {
				return err
			}
			form = f
		}
		condition := uv.Condition
		if req.Condition != nil {
			c, err := parseCondition(req.Condition)
			if err != nil {
				return err
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
		if _, err := txRepo.UpdateUserVitaminCore(ctx, db.UpdateUserVitaminCoreParams{
			Name:       name,
			DosageForm: form,
			DoseValue:  dose,
			DoseUnit:   doseUnit,
			Condition:  condition,
			Note:       note,
			ID:         id,
			UserID:     userID,
		}); err != nil {
			return err
		}

		if req.Course != nil {
			course, err := txRepo.GetVitaminCourseByUserVitaminID(ctx, id)
			if err != nil {
				return err
			}
			startDate := course.StartDate
			if strings.TrimSpace(req.Course.StartDate) != "" {
				startDate, err = parseDate(req.Course.StartDate)
				if err != nil {
					return err
				}
			}
			endDate := course.EndDate
			if req.Course.EndDate != nil || req.Course.DurationDays != nil {
				endDate, err = parseEndDate(startDate, req.Course.EndDate, req.Course.DurationDays)
				if err != nil {
					return err
				}
			}
			timezone := course.Timezone
			if strings.TrimSpace(req.Course.Timezone) != "" {
				timezone = strings.TrimSpace(req.Course.Timezone)
			}
			if strings.TrimSpace(timezone) == "" {
				return ErrTimezoneRequired
			}
			if _, err := txRepo.UpdateVitaminCourse(ctx, db.UpdateVitaminCourseParams{
				StartDate:     startDate,
				EndDate:       endDate,
				Timezone:      timezone,
				UserVitaminID: id,
			}); err != nil {
				return err
			}
		}

		if req.Schedule != nil {
			mask, err := parseDaysMask(req.Schedule.Days)
			if err != nil {
				return err
			}
			times, err := validateTimes(req.Schedule.Times)
			if err != nil {
				return err
			}
			course, err := txRepo.GetVitaminCourseByUserVitaminID(ctx, id)
			if err != nil {
				return err
			}
			if _, err := txRepo.UpdateIntakeSchedule(ctx, db.UpdateIntakeScheduleParams{
				Type:     scheduleTypeFromMask(mask),
				DaysMask: mask,
				CourseID: course.ID,
			}); err != nil {
				return err
			}
			schedule, err := txRepo.GetIntakeScheduleByCourseID(ctx, course.ID)
			if err != nil {
				return err
			}
			if err := txRepo.DeleteIntakeTimesByScheduleID(ctx, schedule.ID); err != nil {
				return err
			}
			for i, t := range times {
				if _, err := txRepo.CreateIntakeTime(ctx, db.CreateIntakeTimeParams{
					ScheduleID: schedule.ID,
					TimeOfDay:  t,
					SortOrder:  int32(i),
				}); err != nil {
					return err
				}
			}
		}

		if req.NotificationPreferences != nil || req.ContentOverrides != nil {
			currentPrefs, err := txRepo.GetNotificationPreferencesByUserVitaminID(ctx, id)
			if err != nil {
				return err
			}
			nextPrefs := mergeNotificationPrefs(req.NotificationPreferences, currentPrefs)
			if _, err := txRepo.UpdateNotificationPreferences(ctx, nextPrefs.ToUpdateParams(id)); err != nil {
				return err
			}
			currentOverrides, err := txRepo.GetNotificationOverridesByUserVitaminID(ctx, id)
			if err != nil {
				return err
			}
			nextOverrides := mergeNotificationOverrides(req.ContentOverrides, currentOverrides)
			if _, err := txRepo.UpsertNotificationOverrides(ctx, nextOverrides.WithUserVitaminID(id)); err != nil {
				return err
			}
		}
		uvFresh, err := txRepo.GetUserVitaminByID(ctx, db.GetUserVitaminByIDParams{ID: id, UserID: userID})
		if err != nil {
			return err
		}
		resp, err := s.buildReminderWith(ctx, txRepo, uvFresh)
		if err != nil {
			return err
		}
		built = resp
		return nil
	}); err != nil {
		return ReminderResponse{}, err
	}
	return built, nil
}

func (s *Service) SetReminderActive(ctx context.Context, userID, id int64, active bool) (ReminderResponse, error) {
	_, err := s.repo.UpdateUserVitaminActive(ctx, db.UpdateUserVitaminActiveParams{
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
