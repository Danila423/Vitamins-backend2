package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	ConditionAny    int16 = 0
	ConditionBefore int16 = 1
	ConditionAfter  int16 = 2
	ConditionDuring int16 = 3
)

const (
	FormTablet    int16 = 0
	FormCapsule   int16 = 1
	FormPowder    int16 = 2
	FormLiquid    int16 = 3
	FormInjection int16 = 4
	FormOther     int16 = 5
	FormDrops     int16 = 6
	FormChewable  int16 = 7
	FormAmpoule   int16 = 8
	FormSpray     int16 = 9
)

const (
	ScheduleEveryday int16 = 0
	ScheduleCustom   int16 = 1
)

var dayBits = map[string]int16{
	"mon": 1 << 0,
	"tue": 1 << 1,
	"wed": 1 << 2,
	"thu": 1 << 3,
	"fri": 1 << 4,
	"sat": 1 << 5,
	"sun": 1 << 6,
}

var dayOrder = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

func parseCondition(raw *string) (int16, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return ConditionAny, nil
	}
	switch strings.ToLower(strings.TrimSpace(*raw)) {
	case "any":
		return ConditionAny, nil
	case "before", "before_meal":
		return ConditionBefore, nil
	case "after", "after_meal":
		return ConditionAfter, nil
	case "during", "during_meal", "with_meal":
		return ConditionDuring, nil
	default:
		return 0, ErrInvalidCondition
	}
}

func conditionToString(v int16) string {
	switch v {
	case ConditionBefore:
		return "before_meal"
	case ConditionAfter:
		return "after_meal"
	case ConditionDuring:
		return "during_meal"
	default:
		return "any"
	}
}

func parseForm(raw *string) (int16, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return FormTablet, nil
	}
	switch strings.ToLower(strings.TrimSpace(*raw)) {
	case "tablet":
		return FormTablet, nil
	case "capsule":
		return FormCapsule, nil
	case "powder":
		return FormPowder, nil
	case "liquid":
		return FormLiquid, nil
	case "injection":
		return FormInjection, nil
	case "drops", "drop":
		return FormDrops, nil
	case "chewable", "chewable_tablet":
		return FormChewable, nil
	case "ampoule", "ampule":
		return FormAmpoule, nil
	case "spray":
		return FormSpray, nil
	case "other":
		return FormOther, nil
	default:
		return 0, ErrInvalidForm
	}
}

func formToString(v int16) string {
	switch v {
	case FormCapsule:
		return "capsule"
	case FormPowder:
		return "powder"
	case FormLiquid:
		return "liquid"
	case FormInjection:
		return "injection"
	case FormDrops:
		return "drops"
	case FormChewable:
		return "chewable_tablet"
	case FormAmpoule:
		return "ampoule"
	case FormSpray:
		return "spray"
	case FormOther:
		return "other"
	default:
		return "tablet"
	}
}

func defaultUnitForForm(v int16) string {
	switch v {
	case FormDrops:
		return "капли"
	case FormPowder:
		return "г"
	case FormLiquid:
		return "мл"
	case FormSpray:
		return "нажатия"
	default:
		return "шт"
	}
}

func parseDaysMask(days []string) (int16, error) {
	if len(days) == 0 {
		return 127, nil
	}
	var mask int16
	for _, d := range days {
		key := strings.ToLower(strings.TrimSpace(d))
		bit, ok := dayBits[key]
		if !ok {
			return 0, ErrInvalidDays
		}
		mask |= bit
	}
	if mask == 0 {
		return 0, ErrInvalidDays
	}
	return mask, nil
}

func daysFromMask(mask int16) []string {
	var days []string
	for _, d := range dayOrder {
		if mask&dayBits[d] != 0 {
			days = append(days, d)
		}
	}
	return days
}

func scheduleTypeFromMask(mask int16) int16 {
	if mask == 127 {
		return ScheduleEveryday
	}
	return ScheduleCustom
}

func scheduleTypeToString(t int16) string {
	if t == ScheduleEveryday {
		return "everyday"
	}
	return "custom"
}

func parseTimeOfDay(raw string) (pgtype.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return pgtype.Time{}, ErrInvalidTimes
	}
	layout := "15:04"
	if strings.Count(value, ":") == 2 {
		layout = "15:04:05"
	}
	parsed, err := time.Parse(layout, value)
	if err != nil {
		return pgtype.Time{}, ErrInvalidTimes
	}
	seconds := parsed.Hour()*3600 + parsed.Minute()*60 + parsed.Second()
	micro := int64(seconds) * int64(time.Second/time.Microsecond)
	return pgtype.Time{Microseconds: micro, Valid: true}, nil
}

func formatTimeOfDay(t pgtype.Time) string {
	if !t.Valid {
		return ""
	}
	totalSeconds := t.Microseconds / int64(time.Second/time.Microsecond)
	h := totalSeconds / 3600
	m := (totalSeconds % 3600) / 60
	s := totalSeconds % 60
	if s == 0 {
		return fmt.Sprintf("%02d:%02d", h, m)
	}
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func parseDate(raw string) (pgtype.Date, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return pgtype.Date{}, ErrStartDateRequired
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return pgtype.Date{}, ErrInvalidDate
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func parseEndDate(start pgtype.Date, end *string, durationDays *int) (pgtype.Date, error) {
	if end != nil && durationDays != nil {
		return pgtype.Date{}, ErrInvalidCourseDuration
	}
	if end != nil {
		value := strings.TrimSpace(*end)
		if value == "" {
			return pgtype.Date{}, nil
		}
		t, err := time.Parse("2006-01-02", value)
		if err != nil {
			return pgtype.Date{}, ErrInvalidDate
		}
		return pgtype.Date{Time: t, Valid: true}, nil
	}
	if durationDays != nil {
		if *durationDays <= 0 {
			return pgtype.Date{}, ErrInvalidCourseDuration
		}
		startTime := start.Time
		endTime := startTime.AddDate(0, 0, *durationDays-1)
		return pgtype.Date{Time: endTime, Valid: true}, nil
	}
	return pgtype.Date{}, nil
}

func dateToString(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

func textToPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func boolOrDefault(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func validateTimes(times []string) ([]pgtype.Time, error) {
	if len(times) == 0 {
		return nil, ErrInvalidTimes
	}
	seen := make(map[int64]struct{}, len(times))
	parsed := make([]pgtype.Time, 0, len(times))
	for _, t := range times {
		value, err := parseTimeOfDay(t)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[value.Microseconds]; ok {
			return nil, ErrInvalidTimes
		}
		seen[value.Microseconds] = struct{}{}
		parsed = append(parsed, value)
	}
	return parsed, nil
}

var (
	ErrInvalidCondition      = errors.New("INVALID_CONDITION")
	ErrInvalidForm           = errors.New("INVALID_FORM")
	ErrInvalidDays           = errors.New("INVALID_DAYS")
	ErrInvalidTimes          = errors.New("INVALID_TIMES")
	ErrStartDateRequired     = errors.New("START_DATE_REQUIRED")
	ErrInvalidDate           = errors.New("INVALID_DATE")
	ErrInvalidCourseDuration = errors.New("INVALID_COURSE_DURATION")
)
