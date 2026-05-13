# Схема базы данных Vitamins Backend

Источник истины: `pkg/db/models.sql`.

Проект использует одну PostgreSQL-базу с девятью таблицами. Схема применяется во время запуска из `pkg/db/models.sql` через `pkg/db/migrate.go`; файлы `migrations/0001_init.sql` и `pkg/db/migrations/0001_init.sql` дублируют ту же начальную схему.

## Файлы

- `database-schema.drawio` - редактируемая ER-диаграмма для diagrams.net / draw.io.
- `database-schema.dbml` - схема для импорта в dbdiagram.io.
- `database-schema.md` - текстовое описание схемы.

## Домены

- Аутентификация и профиль: `users`.
- Справочник витаминов: `vitamin_catalog`, наполняется seed-файлом `pkg/db/seed_vitamin_catalog.sql`.
- Пользовательские напоминания: `user_vitamins`, `vitamin_courses`, `intake_schedules`, `intake_times`.
- Контент уведомлений: `notification_preferences`, `notification_text_overrides`.
- Аналитика: `analytics_events`, а также поля consent в `users`.

## Основные связи

- `users.id` -> `user_vitamins.user_id`: один пользователь имеет много напоминаний; при удалении пользователя его напоминания удаляются каскадно.
- `vitamin_catalog.id` -> `user_vitamins.catalog_id`: напоминание может ссылаться на элемент справочника, но поле nullable.
- `user_vitamins.id` -> `vitamin_courses.user_vitamin_id`: одно напоминание имеет один курс; FK уникальный.
- `vitamin_courses.id` -> `intake_schedules.course_id`: один курс имеет одно расписание; FK уникальный.
- `intake_schedules.id` -> `intake_times.schedule_id`: одно расписание имеет много времен приема.
- `user_vitamins.id` -> `notification_preferences.user_vitamin_id`: одно напоминание имеет одну строку настроек уведомлений; FK уникальный.
- `user_vitamins.id` -> `notification_text_overrides.user_vitamin_id`: одно напоминание может иметь одну строку переопределений текста; FK уникальный.
- `users.id` -> `analytics_events.user_id`: один пользователь может иметь много событий аналитики; при удалении пользователя `analytics_events.user_id` становится `NULL`.

## Кодируемые поля

`condition` и `default_condition`:

- `0` = `any`
- `1` = `before_meal`
- `2` = `after_meal`
- `3` = `during_meal`

`dosage_form`:

- `0` = `tablet`
- `1` = `capsule`
- `2` = `powder`
- `3` = `liquid`
- `4` = `injection`
- `5` = `other`
- `6` = `drops`
- `7` = `chewable_tablet`
- `8` = `ampoule`
- `9` = `spray`

`intake_schedules.type`:

- `0` = `everyday`
- `1` = `custom`

`intake_schedules.days_mask`:

- `mon` = `1`
- `tue` = `2`
- `wed` = `4`
- `thu` = `8`
- `fri` = `16`
- `sat` = `32`
- `sun` = `64`
- `127` = все дни недели

## Правила целостности

- `user_vitamins.name` не может быть пустым.
- `analytics_events` должен содержать либо `user_id`, либо `anonymous_id`.
- `vitamin_courses.user_vitamin_id`, `intake_schedules.course_id`, `notification_preferences.user_vitamin_id` и `notification_text_overrides.user_vitamin_id` уникальны, поэтому эти таблицы работают как one-to-one расширения.
- У `intake_times` есть уникальный индекс на `(schedule_id, time_of_day)`, который запрещает дублировать одно и то же время в рамках одного расписания.

## Уникальные ограничения

- `users.email`
- `vitamin_catalog.code`
- `vitamin_courses.user_vitamin_id`
- `intake_schedules.course_id`
- `notification_preferences.user_vitamin_id`
- `notification_text_overrides.user_vitamin_id`
- `intake_times(schedule_id, time_of_day)`

## Политики удаления внешних ключей

- `user_vitamins.user_id` -> `users.id`: `ON DELETE CASCADE`
- `user_vitamins.catalog_id` -> `vitamin_catalog.id`: стандартное поведение PostgreSQL, `NO ACTION`
- `vitamin_courses.user_vitamin_id` -> `user_vitamins.id`: `ON DELETE CASCADE`
- `intake_schedules.course_id` -> `vitamin_courses.id`: `ON DELETE CASCADE`
- `intake_times.schedule_id` -> `intake_schedules.id`: `ON DELETE CASCADE`
- `notification_preferences.user_vitamin_id` -> `user_vitamins.id`: `ON DELETE CASCADE`
- `notification_text_overrides.user_vitamin_id` -> `user_vitamins.id`: `ON DELETE CASCADE`
- `analytics_events.user_id` -> `users.id`: `ON DELETE SET NULL`

## Важные индексы

- `idx_user_vitamins_user_active` на `(user_id, is_active)` - для выборки активных напоминаний пользователя.
- `idx_user_vitamins_user_catalog` на `(user_id, catalog_id)` - для напоминаний пользователя, связанных со справочником.
- `idx_intake_schedules_course` на `(course_id)` - для поиска расписания по курсу.
- `idx_analytics_events_name_time` на `(event_name, occurred_at)` - для фильтрации и экспорта событий.
- `idx_analytics_events_user_time` на `(user_id, occurred_at)` - для пользовательской аналитики.
- `idx_analytics_events_anon_time` на `(anonymous_id, occurred_at)` - для анонимной аналитики.
