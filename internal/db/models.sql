CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    first_name TEXT NOT NULL DEFAULT '',
    last_name TEXT NOT NULL DEFAULT '',
    analytics_consent BOOLEAN NOT NULL DEFAULT false,
    analytics_consent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vitamin_catalog (
    id BIGSERIAL PRIMARY KEY,
    code TEXT UNIQUE,
    display_name TEXT NOT NULL,
    default_unit TEXT,
    interaction_text TEXT,
    compatibility_text TEXT,
    contraindications_text TEXT,
    default_condition SMALLINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_vitamins (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    catalog_id BIGINT REFERENCES vitamin_catalog(id),
    name TEXT NOT NULL DEFAULT '',
    dosage_form SMALLINT NOT NULL DEFAULT 0,
    dose_value TEXT NOT NULL DEFAULT '',
    dose_unit TEXT NOT NULL DEFAULT '',
    condition SMALLINT NOT NULL DEFAULT 0,
    note TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (name <> '')
);

CREATE INDEX IF NOT EXISTS idx_user_vitamins_user_active ON user_vitamins (user_id, is_active);
CREATE INDEX IF NOT EXISTS idx_user_vitamins_user_catalog ON user_vitamins (user_id, catalog_id);

CREATE TABLE IF NOT EXISTS vitamin_courses (
    id BIGSERIAL PRIMARY KEY,
    user_vitamin_id BIGINT NOT NULL REFERENCES user_vitamins(id) ON DELETE CASCADE,
    start_date DATE NOT NULL,
    end_date DATE,
    timezone TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_vitamin_id)
);

CREATE TABLE IF NOT EXISTS intake_schedules (
    id BIGSERIAL PRIMARY KEY,
    course_id BIGINT NOT NULL REFERENCES vitamin_courses(id) ON DELETE CASCADE,
    type SMALLINT NOT NULL,
    days_mask SMALLINT NOT NULL DEFAULT 127,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (course_id)
);

CREATE INDEX IF NOT EXISTS idx_intake_schedules_course ON intake_schedules (course_id);

CREATE TABLE IF NOT EXISTS intake_times (
    id BIGSERIAL PRIMARY KEY,
    schedule_id BIGINT NOT NULL REFERENCES intake_schedules(id) ON DELETE CASCADE,
    time_of_day TIME NOT NULL,
    sort_order INT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_intake_times_unique ON intake_times (schedule_id, time_of_day);

CREATE TABLE IF NOT EXISTS notification_preferences (
    id BIGSERIAL PRIMARY KEY,
    user_vitamin_id BIGINT NOT NULL REFERENCES user_vitamins(id) ON DELETE CASCADE,
    include_dose BOOLEAN NOT NULL DEFAULT true,
    include_frequency BOOLEAN NOT NULL DEFAULT true,
    include_interaction BOOLEAN NOT NULL DEFAULT true,
    include_compatibility BOOLEAN NOT NULL DEFAULT true,
    include_condition BOOLEAN NOT NULL DEFAULT true,
    include_contraindications BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_vitamin_id)
);

CREATE TABLE IF NOT EXISTS notification_text_overrides (
    id BIGSERIAL PRIMARY KEY,
    user_vitamin_id BIGINT NOT NULL REFERENCES user_vitamins(id) ON DELETE CASCADE,
    interaction_text_override TEXT,
    compatibility_text_override TEXT,
    contraindications_text_override TEXT,
    UNIQUE (user_vitamin_id)
);

CREATE TABLE IF NOT EXISTS analytics_events (
    event_id UUID PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    anonymous_id UUID,
    session_id UUID NOT NULL,
    event_name TEXT NOT NULL,
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id TEXT,
    app_version TEXT,
    platform TEXT,
    CHECK (user_id IS NOT NULL OR anonymous_id IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_analytics_events_name_time ON analytics_events (event_name, occurred_at);
CREATE INDEX IF NOT EXISTS idx_analytics_events_user_time ON analytics_events (user_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_analytics_events_anon_time ON analytics_events (anonymous_id, occurred_at);
