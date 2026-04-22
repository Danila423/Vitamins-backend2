package service

type CreateReminderRequest struct {
	CatalogID               *int64                       `json:"catalog_id"`
	Name                    *string                      `json:"name"`
	Form                    *string                      `json:"form"`
	Dose                    *string                      `json:"dose"`
	Condition               *string                      `json:"condition"`
	Note                    *string                      `json:"note"`
	Course                  CourseInput                  `json:"course"`
	Schedule                ScheduleInput                `json:"schedule"`
	NotificationPreferences NotificationPreferencesInput `json:"notification_preferences"`
	ContentOverrides        ContentOverridesInput        `json:"content_overrides"`
}

type UpdateReminderRequest struct {
	Name                    *string                       `json:"name"`
	Form                    *string                       `json:"form"`
	Dose                    *string                       `json:"dose"`
	Condition               *string                       `json:"condition"`
	Note                    *string                       `json:"note"`
	Course                  *CourseInput                  `json:"course"`
	Schedule                *ScheduleInput                `json:"schedule"`
	NotificationPreferences *NotificationPreferencesInput `json:"notification_preferences"`
	ContentOverrides        *ContentOverridesInput        `json:"content_overrides"`
}

type CourseInput struct {
	StartDate    string  `json:"start_date"`
	EndDate      *string `json:"end_date"`
	DurationDays *int    `json:"duration_days"`
	Timezone     string  `json:"timezone"`
}

type ScheduleInput struct {
	Days  []string `json:"days"`
	Times []string `json:"times"`
}

type NotificationPreferencesInput struct {
	IncludeDose              *bool `json:"include_dose"`
	IncludeFrequency         *bool `json:"include_frequency"`
	IncludeInteraction       *bool `json:"include_interaction"`
	IncludeCompatibility     *bool `json:"include_compatibility"`
	IncludeCondition         *bool `json:"include_condition"`
	IncludeContraindications *bool `json:"include_contraindications"`
}

type ContentOverridesInput struct {
	InteractionTextOverride       *string `json:"interaction_text_override"`
	CompatibilityTextOverride     *string `json:"compatibility_text_override"`
	ContraindicationsTextOverride *string `json:"contraindications_text_override"`
}

type CatalogItem struct {
	ID                    int64   `json:"id"`
	Code                  *string `json:"code,omitempty"`
	DisplayName           string  `json:"display_name"`
	DefaultUnit           *string `json:"default_unit,omitempty"`
	InteractionText       *string `json:"interaction_text,omitempty"`
	CompatibilityText     *string `json:"compatibility_text,omitempty"`
	ContraindicationsText *string `json:"contraindications_text,omitempty"`
	DefaultCondition      *string `json:"default_condition,omitempty"`
}

type CourseResponse struct {
	StartDate string  `json:"start_date"`
	EndDate   *string `json:"end_date,omitempty"`
	Timezone  string  `json:"timezone"`
}

type ScheduleResponse struct {
	Type  string   `json:"type"`
	Days  []string `json:"days"`
	Times []string `json:"times"`
}

type NotificationPreferencesResponse struct {
	IncludeDose              bool `json:"include_dose"`
	IncludeFrequency         bool `json:"include_frequency"`
	IncludeInteraction       bool `json:"include_interaction"`
	IncludeCompatibility     bool `json:"include_compatibility"`
	IncludeCondition         bool `json:"include_condition"`
	IncludeContraindications bool `json:"include_contraindications"`
}

type ContentOverridesResponse struct {
	InteractionTextOverride       *string `json:"interaction_text_override,omitempty"`
	CompatibilityTextOverride     *string `json:"compatibility_text_override,omitempty"`
	ContraindicationsTextOverride *string `json:"contraindications_text_override,omitempty"`
}

type ReminderResponse struct {
	ID                      int64                           `json:"id"`
	CatalogID               *int64                          `json:"catalog_id,omitempty"`
	Name                    string                          `json:"name"`
	Form                    string                          `json:"form"`
	Dose                    string                          `json:"dose"`
	Condition               string                          `json:"condition"`
	Note                    string                          `json:"note"`
	IsActive                bool                            `json:"is_active"`
	Catalog                 *CatalogItem                    `json:"catalog,omitempty"`
	Course                  CourseResponse                  `json:"course"`
	Schedule                ScheduleResponse                `json:"schedule"`
	NotificationPreferences NotificationPreferencesResponse `json:"notification_preferences"`
	ContentOverrides        ContentOverridesResponse        `json:"content_overrides"`
}
