-- name: CreateUser :one
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
RETURNING id, email, password_hash, first_name, last_name, created_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, first_name, last_name, created_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, password_hash, first_name, last_name, created_at
FROM users
WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $1
WHERE id = $2;

-- name: UpdateUserProfile :one
UPDATE users
SET email = $1,
    first_name = $2,
    last_name = $3
WHERE id = $4
RETURNING id, email, password_hash, first_name, last_name, created_at;

-- name: ListVitaminCatalog :many
SELECT id, code, display_name, default_unit, interaction_text, compatibility_text,
       contraindications_text, default_condition, created_at, updated_at
FROM vitamin_catalog
ORDER BY display_name;

-- name: GetVitaminCatalogByID :one
SELECT id, code, display_name, default_unit, interaction_text, compatibility_text,
       contraindications_text, default_condition, created_at, updated_at
FROM vitamin_catalog
WHERE id = $1;

-- name: CreateUserVitamin :one
INSERT INTO user_vitamins (
    user_id, catalog_id, name, dosage_form, dose_value, dose_unit, condition, note, is_active
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, user_id, catalog_id, name, dosage_form, dose_value, dose_unit, condition, note,
          is_active, created_at, updated_at;

-- name: GetUserVitaminByID :one
SELECT id, user_id, catalog_id, name, dosage_form, dose_value, dose_unit, condition, note,
       is_active, created_at, updated_at
FROM user_vitamins
WHERE id = $1 AND user_id = $2;

-- name: ListUserVitamins :many
SELECT id, user_id, catalog_id, name, dosage_form, dose_value, dose_unit, condition, note,
       is_active, created_at, updated_at
FROM user_vitamins
WHERE user_id = $1
ORDER BY id;

-- name: UpdateUserVitaminCore :one
UPDATE user_vitamins
SET name = $1,
    dosage_form = $2,
    dose_value = $3,
    dose_unit = $4,
    condition = $5,
    note = $6,
    updated_at = now()
WHERE id = $7 AND user_id = $8
RETURNING id, user_id, catalog_id, name, dosage_form, dose_value, dose_unit, condition, note,
          is_active, created_at, updated_at;

-- name: UpdateUserVitaminActive :one
UPDATE user_vitamins
SET is_active = $1,
    updated_at = now()
WHERE id = $2 AND user_id = $3
RETURNING id, user_id, catalog_id, name, dosage_form, dose_value, dose_unit, condition, note,
          is_active, created_at, updated_at;

-- name: CreateVitaminCourse :one
INSERT INTO vitamin_courses (user_vitamin_id, start_date, end_date, timezone)
VALUES ($1, $2, $3, $4)
RETURNING id, user_vitamin_id, start_date, end_date, timezone, created_at;

-- name: GetVitaminCourseByUserVitaminID :one
SELECT id, user_vitamin_id, start_date, end_date, timezone, created_at
FROM vitamin_courses
WHERE user_vitamin_id = $1;

-- name: UpdateVitaminCourse :one
UPDATE vitamin_courses
SET start_date = $1,
    end_date = $2,
    timezone = $3
WHERE user_vitamin_id = $4
RETURNING id, user_vitamin_id, start_date, end_date, timezone, created_at;

-- name: CreateIntakeSchedule :one
INSERT INTO intake_schedules (course_id, type, days_mask)
VALUES ($1, $2, $3)
RETURNING id, course_id, type, days_mask, created_at;

-- name: GetIntakeScheduleByCourseID :one
SELECT id, course_id, type, days_mask, created_at
FROM intake_schedules
WHERE course_id = $1;

-- name: UpdateIntakeSchedule :one
UPDATE intake_schedules
SET type = $1,
    days_mask = $2
WHERE course_id = $3
RETURNING id, course_id, type, days_mask, created_at;

-- name: CreateIntakeTime :one
INSERT INTO intake_times (schedule_id, time_of_day, sort_order)
VALUES ($1, $2, $3)
RETURNING id, schedule_id, time_of_day, sort_order;

-- name: ListIntakeTimesByScheduleID :many
SELECT id, schedule_id, time_of_day, sort_order
FROM intake_times
WHERE schedule_id = $1
ORDER BY sort_order, time_of_day;

-- name: DeleteIntakeTimesByScheduleID :exec
DELETE FROM intake_times
WHERE schedule_id = $1;

-- name: CreateNotificationPreferences :one
INSERT INTO notification_preferences (
    user_vitamin_id, include_dose, include_frequency, include_interaction,
    include_compatibility, include_condition, include_contraindications
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_vitamin_id, include_dose, include_frequency, include_interaction,
          include_compatibility, include_condition, include_contraindications,
          created_at, updated_at;

-- name: GetNotificationPreferencesByUserVitaminID :one
SELECT id, user_vitamin_id, include_dose, include_frequency, include_interaction,
       include_compatibility, include_condition, include_contraindications,
       created_at, updated_at
FROM notification_preferences
WHERE user_vitamin_id = $1;

-- name: UpdateNotificationPreferences :one
UPDATE notification_preferences
SET include_dose = $1,
    include_frequency = $2,
    include_interaction = $3,
    include_compatibility = $4,
    include_condition = $5,
    include_contraindications = $6,
    updated_at = now()
WHERE user_vitamin_id = $7
RETURNING id, user_vitamin_id, include_dose, include_frequency, include_interaction,
          include_compatibility, include_condition, include_contraindications,
          created_at, updated_at;

-- name: UpsertNotificationOverrides :one
INSERT INTO notification_text_overrides (
    user_vitamin_id, interaction_text_override, compatibility_text_override,
    contraindications_text_override
)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_vitamin_id)
DO UPDATE SET interaction_text_override = EXCLUDED.interaction_text_override,
              compatibility_text_override = EXCLUDED.compatibility_text_override,
              contraindications_text_override = EXCLUDED.contraindications_text_override
RETURNING id, user_vitamin_id, interaction_text_override, compatibility_text_override,
          contraindications_text_override;

-- name: GetNotificationOverridesByUserVitaminID :one
SELECT id, user_vitamin_id, interaction_text_override, compatibility_text_override,
       contraindications_text_override
FROM notification_text_overrides
WHERE user_vitamin_id = $1;
