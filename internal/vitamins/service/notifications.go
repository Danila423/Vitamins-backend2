package service

import (
	"strings"

	"vitamins-backend_2/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// notificationPrefsInput / notificationOverridesInput are internal value
// objects used by the service layer to build sqlc parameters for both create
// and update flows. They are intentionally not exported — handlers consume the
// JSON-side `NotificationPreferencesInput` / `ContentOverridesInput` types
// from types.go.
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
