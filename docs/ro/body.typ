= НАЗНАЧЕНИЕ ПРОГРАММЫ
== Функциональное назначение

#h(2em) Программа представляет собой серверную часть программного комплекса «Мобильное приложение для напоминаний о приёме витаминов» (англ. «Mobile Application to Track Supplement Intake») и предназначена для развёртывания на вычислительной технике оператора (сервере или рабочей станции разработчика) и обеспечения клиентским приложениям следующих функций по протоколу HTTP (REST API). Исходный код и актуальное описание эндпоинтов -- в репозитории `https://github.com/Danila423/Vitamins-backend.git` (Swagger: `/swagger/index.html`).

\- регистрация, аутентификация и обновление сеансовых токенов (JWT);\
\- управление профилем пользователя и смена пароля с подтверждением по электронной почте;\
\- сброс пароля по коду из письма;\
\- выдача каталога витаминов и ведение напоминаний о приёме (создание, изменение, удаление, включение и отключение);\
\- приём и хранение аналитических событий, управление согласием на аналитику;\
\- административный экспорт накопленной аналитики;\
\- предоставление машиночитаемых метрик для системы мониторинга Prometheus;\
\- интерактивная спецификация API (Swagger UI).

== Эксплуатационное назначение

#h(2em) Программа предназначена для постоянной или периодической эксплуатации в составе контейнеризованной инфраструктуры (Docker / Docker Compose). Оператором в смысле настоящего руководства является лицо, ответственное за развёртывание, настройку, запуск, остановку и сопровождение серверной части (системный администратор, инженер эксплуатации, разработчик бэкенда).

#h(2em) Клиентские приложения (мобильные или веб) взаимодействуют с программой по сети; требования к конечным пользователям сервиса и к рабочим местам оператора различаются и приведены в разделе «Условия выполнения программы».

= УСЛОВИЯ ВЫПОЛНЕНИЯ ПРОГРАММЫ
== Минимальный состав аппаратных средств

#h(2em) Для устойчивой работы в составе стенда разработки или тестового контура рекомендуется следующий состав средств:

1. ПЭВМ или сервер с 64-разрядным процессором, не менее 4 ГБ оперативной памяти (для полного профиля с мониторингом и стеком ELK — не менее 8 ГБ).
2. Свободное место на диске не менее 10 ГБ под образы контейнеров и тома данных (база данных, кэш, опционально — Elasticsearch, Prometheus).
3. Сетевое подключение для приёма запросов клиентов и (при необходимости) отправки электронной почты через внешний SMTP.

== Минимальный состав программных средств

#h(2em) На компьютере оператора должны быть установлены:

\- 64-разрядная ОС Linux, macOS или Windows с поддержкой Docker Desktop / Docker Engine и плагина Docker Compose v2;\
\- при сборке из исходных текстов — среда Go не ниже версии, указанной в файле `go.mod` репозитория, и утилита `make` (опционально);\
\- для ручной проверки API — веб-браузер или клиент командной строки (`curl`).

#h(2em) В инфраструктуре контейнеров используются образы: PostgreSQL 16, Redis 7, а также при полном профиле — компоненты Prometheus, Grafana, Alertmanager, стек ELK (Elasticsearch, Logstash, Kibana) в составе, описанном в файле `docker-compose.yml` репозитория.

== Требования к персоналу (оператору)

#h(2em) Оператор должен владеть базовыми навыками работы в командной строке, понимать назначение переменных окружения и портов контейнеров, уметь просматривать журналы (`docker compose logs`) и при необходимости обращаться к документации Docker и PostgreSQL.

#h(2em) Для первичной настройки секретов (`JWT_SECRET`, `ADMIN_TOKEN`, параметры SMTP) требуется соблюдение правил информационной безопасности: не публиковать реальные значения в открытых репозиториях, хранить актуальный файл `.env` вне системы контроля версий.

= ВЫПОЛНЕНИЕ ПРОГРАММЫ
== Получение исходного кода

#h(2em) Исходный код размещается в системе контроля версий (Git). Оператор клонирует репозиторий проекта на локальный компьютер или развёртывает копию на сервере заказчика в соответствии с принятой политикой доступа.

== Подготовка конфигурации

#h(2em) В корне проекта создаётся файл `.env` (на основе внутренней инструкции команды разработки), в котором задаются как минимум:

\- `DATABASE_URL` — строка подключения к PostgreSQL;\
\- `JWT_SECRET` — секрет для подписи JWT;\
\- `REDIS_ADDR` (и при необходимости `REDIS_PASSWORD`, `REDIS_DB`) — параметры Redis;\
\- параметры SMTP (`SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`) для отправки писем с кодами;\
\- `ADMIN_TOKEN` — токен администратора для доступа к экспорту аналитики.

#h(2em) Файл `.env` подключается к сервису `api` через директиву `env_file` в `docker-compose.yml`. После изменения переменных окружения контейнер приложения следует пересоздать (`docker compose up -d --force-recreate api`), а не ограничиваться перезапуском без пересоздания.

== Сборка и запуск

#h(2em) *Минимальный контур* (приложение, СУБД, Redis) запускается из корня репозитория командой вида:

#h(2em) `docker compose up -d --build db redis api`

#h(2em) При первом старте база данных инициализируется скриптами из каталога `internal/db`, выполняется загрузка каталога витаминов.

#h(2em) *Полный контур* с мониторингом и журналированием — в соответствии с профилем сервисов в `docker-compose.yml` (Prometheus, Grafana, ELK и др.) с учётом доступных ресурсов. Пример: `docker compose up -d --build`. Ниже приведены примеры: фрагмент вывода сборки образа приложения и итоговое состояние запуска контейнеров (успешный запуск всех сервисов).

#figure(
  image("images/docker-compose-build.png", width: 100%),
  caption: [Фрагмент вывода сборки и развёртывания (`docker compose up -d --build`)],
)

#figure(
  image("images/docker-compose-running.png", width: 100%),
  caption: [Успешный запуск контейнеров (все заданные сервисы в состоянии Running / Healthy / Started)],
)

#h(2em) Проверка готовности HTTP-сервиса: обращение к базовому URL (по умолчанию порт `8080` согласно настройкам). Интерактивная документация API доступна по пути `/swagger/index.html` (см. рисунок ниже).

#figure(
  image("images/swagger-ui.png", width: 100%),
  caption: [Доступ к интерактивной спецификации API (Swagger UI)],
)

== Типовые операции эксплуатации

=== Просмотр состояния контейнеров и журналов

#h(2em) Для диагностики используются команды `docker compose ps` и `docker compose logs -f api` (или имя соответствующего сервиса). Структурированные записи при настроенном драйвере GELF индексируются в стеке ELK; в Kibana можно открыть представление с данным потоком (см. рисунок ниже).

#figure(
  image("images/kibana-discover.png", width: 100%),
  caption: [Просмотр логов приложения в Kibana (например, data view «Vitamins logs»)],
)

=== Мониторинг (Grafana)

#h(2em) Графану при полном профиле удобно открывать в браузере на порту, проброшенном в `docker-compose` (по умолчанию `3000`). Предустановленные панели сгруппированы в папке (см. рисунок ниже).

#figure(
  image("images/grafana-vitamins.png", width: 100%),
  caption: [Список дашбордов мониторинга (папка «Vitamins» в Grafana)],
)

=== Метрики (Prometheus)

#h(2em) Эндпоинт `GET /metrics` отдаёт метрики в формате Prometheus; при развёрнутом Prometheus они снимаются по расписанию. Пример фрагмента ответа приведён на рисунке ниже.

#figure(
  image("images/metrics-prometheus.png", width: 100%),
  caption: [Фрагмент выдачи метрик в формате Prometheus (эндпоинт `GET /metrics`)],
)

=== Резервное копирование и обновление

#h(2em) Данные PostgreSQL хранятся в именованном томе Docker; резервное копирование выполняется штатными средствами СУБД (`pg_dump`) согласно регламенту заказчика. Обновление версии приложения — путём пересборки образа и повторного запуска compose с теми же томами.

== Завершение работы

#h(2em) Остановка сервисов: `docker compose down` в каталоге проекта. При необходимости сохранения данных тома не удаляются без явного указания опций очистки томов.

= СООБЩЕНИЯ ОПЕРАТОРУ
== Сообщения и коды ошибок API

#h(2em) Для маршрутов JSON API (префикс `/api/v1/…`) тело ответа об ошибке имеет вид `{"code": "<код>", "message": "<текст>"}`. Успешные ответы — с кодом HTTP 2xx; конкретная структура тела зависит от маршрута (см. OpenAPI/ Swagger).

#h(2em) *Общие случаи.* Необработанная паника или иная внутренняя ошибка: HTTP 500, `INTERNAL_ERROR`, сообщение «Что-то пошло не так.» Недоступность СУБД или кэша может проявляться как HTTP 500/503 в зависимости от стека. Эндпоинты `GET /metrics` и `GET /swagger/…` отдают не JSON, а, соответственно, текст метрик / HTML-страницу; коды HTML-ошибок — по правилам веб-сервера.

#h(2em) *Авторизация bearer JWT (маршруты, где токен обязателен).* См. `Authorization: Bearer <access token>`. Отсутствие заголовка: HTTP 401, `AUTH_REQUIRED` («Требуется авторизация»). Неверный формат заголовка, пустой токен, невалидный или просроченный access-токен: HTTP 401, `INVALID_TOKEN` («Неверный токен»).

#h(2em) *Опциональный JWT* (только `POST /api/v1/analytics/events`): токен не передаётся — запрос обрабатывается без пользователя. Если заголовок `Authorization` передан, но невалиден — HTTP 401, `INVALID_TOKEN`.

#h(2em) *Админ-токен* (`X-Admin-Token` на `GET /api/v1/admin/analytics/export`). Переменная `ADMIN_TOKEN` на сервере пуста: HTTP 401, `ADMIN_REQUIRED` («Админ токен не настроен»). Токен не совпал: HTTP 401, `ADMIN_REQUIRED` («Требуется админ токен»).

#h(2em) Ниже — *23 маршрута* REST API с префиксом `/api/v1` (как в `cmd/api/main.go`). Служебные `GET /metrics` и `GET /swagger/…` без префикса `/api/v1` описаны в общем вводном абзаце.

=== `POST /api/v1/auth/register`

#h(2em) *Успех:* HTTP 200, тело — пара токенов `accessToken`, `refreshToken` и сопутствующие поля. *Ошибки:*\

\- 400, `BAD_REQUEST` — неверный JSON;\

\- 400, `EMAIL_AND_PASSWORD_REQUIRED` — не заданы email/пароль;\

\- 400, `EMAIL_REQUIRED` / `INVALID_EMAIL_FORMAT` / `PASSWORD_REQUIRED` / `INVALID_PASSWORD_FORMAT`;\

\- 409, `EMAIL_ALREADY_EXISTS` — email уже зарегистрирован;\

\- 500, `INTERNAL_ERROR`.

=== `POST /api/v1/auth/login`

#h(2em) *Успех:* HTTP 200, пара токенов. *Ошибки:*\

\- 400, `BAD_REQUEST`, `EMAIL_AND_PASSWORD_REQUIRED`, `EMAIL_REQUIRED`, `INVALID_EMAIL_FORMAT`, `PASSWORD_REQUIRED`, `INVALID_PASSWORD_FORMAT`;\

\- 401, `INVALID_CREDENTIALS` — неверные email или пароль;\

\- 500, `INTERNAL_ERROR`.

=== `POST /api/v1/auth/refresh`

#h(2em) *Успех:* HTTP 200, новая пара токенов. *Ошибки:*\

\- 400, `BAD_REQUEST` — неверный JSON;\

\- 401, `INVALID_REFRESH_TOKEN` — невалидный или истёкший refresh token;\

\- 500, `INTERNAL_ERROR`.

=== `POST /api/v1/auth/password/reset/request`

#h(2em) *Успех:* HTTP 200. *Ошибки:*\

\- 400, `BAD_REQUEST` / `EMAIL_REQUIRED` / `INVALID_EMAIL_FORMAT`;\

\- 404, `USER_NOT_FOUND`;\

\- 429, `TOO_MANY_REQUESTS`;\

\- 500, `MAILER_NOT_CONFIGURED` / `REDIS_NOT_CONFIGURED` / `INTERNAL_ERROR`.

=== `POST /api/v1/auth/password/reset/verify`

#h(2em) *Успех:* HTTP 200, тело с `resetToken`. *Ошибки:*\

\- 400, `BAD_REQUEST`, `EMAIL_REQUIRED`, `INVALID_EMAIL_FORMAT`, `RESET_CODE_REQUIRED`;\

\- 401, `RESET_CODE_INVALID`, `RESET_CODE_EXPIRED`, `RESET_CODE_TOO_MANY_ATTEMPTS`;\

\- 500, `REDIS_NOT_CONFIGURED`, `INTERNAL_ERROR`.

=== `POST /api/v1/auth/password/reset/confirm`

#h(2em) *Успех:* HTTP 200. *Ошибки:*\

\- 400, `BAD_REQUEST`, `RESET_TOKEN_REQUIRED`, `PASSWORD_REQUIRED`, `INVALID_PASSWORD_FORMAT`, `PASSWORD_CONFIRMATION_MISMATCH`;\

\- 401, `RESET_SESSION_INVALID`, `RESET_SESSION_EXPIRED`;\

\- 500, `REDIS_NOT_CONFIGURED`, `INTERNAL_ERROR`.

=== `GET /api/v1/users/me`

#h(2em) *Успех:* HTTP 200, профиль. *Помимо ошибок middleware JWT* (`AUTH_REQUIRED`, `INVALID_TOKEN`):\

\- 404, `USER_NOT_FOUND`;\

\- 500, `INTERNAL_ERROR`.

=== `PATCH /api/v1/users/me`

#h(2em) *Успех:* HTTP 200, обновлённый профиль. *Помимо middleware JWT:*\

\- 400, `BAD_REQUEST` — неверный JSON;\

\- 400, `NO_FIELDS_TO_UPDATE`;\

\- 400, `EMAIL_REQUIRED` / `INVALID_EMAIL_FORMAT`;\

\- 409, `EMAIL_ALREADY_EXISTS`;\

\- 404, `USER_NOT_FOUND`;\

\- 500, `INTERNAL_ERROR`.

=== `POST /api/v1/users/me/password/change/request`

#h(2em) *Успех:* HTTP 200. *Помимо middleware JWT:*\

\- 400, `BAD_REQUEST` — неверный JSON;\

\- 429, `TOO_MANY_REQUESTS`;\

\- 404, `USER_NOT_FOUND`;\

\- 500, `MAILER_NOT_CONFIGURED`, `REDIS_NOT_CONFIGURED`, `INTERNAL_ERROR`.

=== `POST /api/v1/users/me/password/change/verify`

#h(2em) *Успех:* HTTP 200, тело с `changeToken`. *Помимо middleware JWT:*\

\- 400, `BAD_REQUEST`, `CHANGE_CODE_REQUIRED`;\

\- 401, `CHANGE_CODE_INVALID`, `CHANGE_CODE_TOO_MANY_ATTEMPTS`;\

\- 404, `USER_NOT_FOUND`;\

\- 500, `REDIS_NOT_CONFIGURED`, `INTERNAL_ERROR`.

=== `POST /api/v1/users/me/password/change/confirm`

#h(2em) *Успех:* HTTP 200. *Помимо middleware JWT:*\

\- 400, `BAD_REQUEST`, `CHANGE_TOKEN_REQUIRED`, `PASSWORD_REQUIRED`, `INVALID_PASSWORD_FORMAT`, `PASSWORD_CONFIRMATION_MISMATCH`;\

\- 401, `CHANGE_SESSION_INVALID`, `CHANGE_SESSION_EXPIRED`;\

\- 500, `REDIS_NOT_CONFIGURED`, `INTERNAL_ERROR`.

=== `GET /api/v1/vitamins/catalog`

#h(2em) *Успех:* HTTP 200, массив каталога (без JWT). *Ошибки:*\

\- 500, `INTERNAL_ERROR`.

=== `POST /api/v1/vitamins/reminders`

#h(2em) *Успех:* HTTP 200, созданное напоминание. *Помимо middleware JWT:*\

\- 400, `BAD_REQUEST` — неверный JSON тела;\

\- 400 — `NAME_REQUIRED`, `INVALID_FORM`, `INVALID_CONDITION`, `INVALID_DAYS`, `INVALID_TIMES`, `START_DATE_REQUIRED`, `INVALID_DATE_FORMAT`, `INVALID_COURSE_DURATION`, `TIMEZONE_REQUIRED`;\

\- 404, `CATALOG_NOT_FOUND`;\

\- 500, `INTERNAL_ERROR`.

=== `GET /api/v1/vitamins/reminders`

#h(2em) *Успех:* HTTP 200, массив. *Помимо middleware JWT:*\

\- 500, `INTERNAL_ERROR`.

=== `GET /api/v1/vitamins/reminders/:id`

#h(2em) *Успех:* HTTP 200. *Помимо middleware JWT:*\

\- 400, `INVALID_ID` — неверный `id` в пути;\

\- 404, `REMINDER_NOT_FOUND`;\

\- 500, `INTERNAL_ERROR`.

=== `PATCH /api/v1/vitamins/reminders/:id`

#h(2em) *Успех:* HTTP 200. *Помимо middleware JWT:*\

\- 400, `BAD_REQUEST` — неверный JSON;\

\- 400, `INVALID_ID`;\

\- 400, `NO_FIELDS_TO_UPDATE`;\

\- 400/404 — прочие коды валидации из общего `handleError` (в т.ч. `CATALOG_NOT_FOUND` при смене `catalog_id`, поля витамина и курса);\

\- 404, `REMINDER_NOT_FOUND`;\

\- 500, `INTERNAL_ERROR`.

=== `DELETE /api/v1/vitamins/reminders/:id`

#h(2em) *Успех:* HTTP 200, тело с данными напоминания. *Помимо middleware JWT:*\

\- 400, `INVALID_ID`;\

\- 404, `REMINDER_NOT_FOUND`;\

\- 500, `INTERNAL_ERROR`.

=== `POST /api/v1/vitamins/reminders/:id/enable`

#h(2em) *Успех:* HTTP 200. *Помимо middleware JWT:*\

\- 400, `INVALID_ID`;\

\- 404, `REMINDER_NOT_FOUND`;\

\- 500, `INTERNAL_ERROR`.

=== `POST /api/v1/vitamins/reminders/:id/disable`

#h(2em) *Успех:* HTTP 200. *Помимо middleware JWT:*\

\- 400, `INVALID_ID`;\

\- 404, `REMINDER_NOT_FOUND`;\

\- 500, `INTERNAL_ERROR`.

=== `POST /api/v1/analytics/events`

#h(2em) *Успех:* HTTP 200, `accepted`, `deduplicated`. *Ошибки* (часть — без обязательного JWT; невалидный Bearer, если заголовок есть, даёт 401 от опционального middleware):\

\- 400, `BAD_REQUEST` — неверный JSON;\

\- 400, `EMPTY_BATCH`, `BATCH_TOO_LARGE`;\

\- 400, `INVALID_EVENT_ID`, `INVALID_OCCURRED_AT`, `INVALID_EVENT_NAME`, `INVALID_SESSION_ID`, `INVALID_ANONYMOUS_ID`;\

\- 400, `ANONYMOUS_ID_REQUIRED`;\

\- 401, `INVALID_TOKEN` — неверный или истёкший access-токен, если `Authorization` передан;\

\- 403, `CONSENT_REQUIRED`;\

\- 404, `USER_NOT_FOUND`;\

\- 500, `INTERNAL_ERROR`.

=== `GET /api/v1/analytics/consent`

#h(2em) *Успех:* HTTP 200, `consent: bool`. *Помимо middleware JWT:*\

\- 401, `AUTH_REQUIRED` — нет `Authorization: Bearer`;\

\- 401, `INVALID_TOKEN` — неверный access-токен;\

\- 500, `INTERNAL_ERROR`.

=== `POST /api/v1/analytics/consent`

#h(2em) *Успех:* HTTP 200, `consent: bool`. *Помимо middleware JWT:*\

\- 400, `BAD_REQUEST` — неверный JSON тела;\

\- 401, `AUTH_REQUIRED` / `INVALID_TOKEN`;\

\- 500, `INTERNAL_ERROR`.

=== `GET /api/v1/admin/analytics/export`

#h(2em) *Успех:* HTTP 200, поток CSV или JSONL. *Вместо JWT — проверка* `X-Admin-Token` (см. выше; коды `ADMIN_REQUIRED`). *Дополнительно:*\

\- 500, `INTERNAL_ERROR` — сбой выгрузки.

== Действия оператора в исключительных ситуациях

#h(2em) При недоступности API оператор проверяет: запущены ли контейнеры `api`, `db`, `redis`; корректность `DATABASE_URL` и сетевых имён внутри compose; не исчерпаны ли ресурсы хоста. При сбоях миграций — журнал старта контейнера `api` и целостность тома СУБД.

#h(2em) При подозрении на компрометацию секретов — смена `JWT_SECRET` и `ADMIN_TOKEN` с принудительным выходом пользователей из сеансов (аннулирование старых refresh-токенов по политике заказчика) и ротация учётных данных SMTP.

#set heading(numbering: none)

= СПИСОК ИСПОЛЬЗОВАННОЙ ЛИТЕРАТУРЫ

1. ГОСТ 19.101-77: Виды программ и программных документов. \// Единая система программной документации. – М.: ИПК Издательство стандартов, 2001.
2. ГОСТ 19.102-77: Стадии разработки. \// Единая система программной документации. – М.: ИПК Издательство стандартов, 2001.
3. ГОСТ 19.103-77: Обозначения программ и программных документов. \// Единая система программной документации. – М.: ИПК Издательство стандартов, 2001.
4. ГОСТ 19.104-78: Основные надписи. \// Единая система программной документации. – М.: ИПК Издательство стандартов, 2001.
5. ГОСТ 19.105-78: Общие требования к программным документам. \// Единая система программной документации. – М.: ИПК Издательство стандартов, 2001.
6. ГОСТ 19.106-78: Требования к программным документам, выполненным печатным способом. \// Единая система программной документации. – М.: ИПК Издательство стандартов, 2001.
7. ГОСТ 19.505-79: Руководство оператора. Требования к содержанию и оформлению. \// Единая система программной документации. – М.: ИПК Издательство стандартов, 2001.
8. Официальная документация Docker. URL: `https://docs.docker.com/`
9. Официальная документация PostgreSQL. URL: `https://www.postgresql.org/docs/`
10. Нестеркин Д. Г. Техническое задание на программу «Мобильное приложение для напоминаний о приёме витаминов». Обозначение `RU.17701729.12.20-01 ТЗ 01-1`. (локальный комплект программной документации по курсовому проекту.)
11. Репозиторий исходного кода серверной части. Электронный ресурс. URL: `https://github.com/Danila423/Vitamins-backend.git` (дата обращения 22.04.2026)
