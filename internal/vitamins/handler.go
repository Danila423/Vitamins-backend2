package vitamins

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"vitamins-backend_2/internal/auth"
	appLogger "vitamins-backend_2/internal/logger"
)

type Handler struct {
	s *Service
}

func NewHandler(s *Service) *Handler {
	return &Handler{s: s}
}

func send(c *gin.Context, code int, k, m string) {
	c.JSON(code, ErrorResponse{Code: k, Message: m})
}

func logWithContext(c *gin.Context, channel string) *slog.Logger {
	return appLogger.WithContext(slog.Default(), c.Request.Context()).With("channel", channel)
}

// ListCatalog godoc
// @Summary      Каталог витаминов
// @Description  Возвращает список витаминов из справочника
// @Tags         vitamins
// @Accept       json
// @Produce      json
// @Success      200 {array} CatalogItem
// @Failure      500 {object} ErrorResponse
// @Router       /vitamins/catalog [get]
func (h *Handler) ListCatalog(c *gin.Context) {
	ctx := c.Request.Context()
	items, err := h.s.ListCatalog(ctx)
	if err != nil {
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	c.JSON(200, items)
}

// CreateReminder godoc
// @Summary      Создать напоминание о витамине
// @Description  Создает пользовательский витамин с курсом, расписанием и настройками уведомления
// @Tags         vitamins
// @Accept       json
// @Produce      json
// @Param        request body CreateReminderRequest true "Данные напоминания"
// @Security     BearerAuth
// @Success      200 {object} ReminderResponse
// @Failure      400 {object} ErrorResponse "NAME_REQUIRED, INVALID_FORM, INVALID_CONDITION, INVALID_DAYS, INVALID_TIMES, START_DATE_REQUIRED, INVALID_DATE_FORMAT, INVALID_COURSE_DURATION, TIMEZONE_REQUIRED"
// @Failure      401 {object} ErrorResponse "AUTH_REQUIRED, INVALID_TOKEN"
// @Failure      404 {object} ErrorResponse "CATALOG_NOT_FOUND"
// @Failure      500 {object} ErrorResponse
// @Router       /vitamins/reminders [post]
func (h *Handler) CreateReminder(c *gin.Context) {
	var r CreateReminderRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	ctx := c.Request.Context()
	resp, err := h.s.CreateReminder(ctx, userID, r)
	if err != nil {
		h.handleError(c, err)
		return
	}
	logWithContext(c, "audit").InfoContext(ctx, "reminder created",
		"operation", "vitamins.reminder.create",
		"user_id", userID,
		"reminder_id", resp.ID,
	)
	c.JSON(200, resp)
}

// ListReminders godoc
// @Summary      Список напоминаний
// @Description  Возвращает список напоминаний пользователя
// @Tags         vitamins
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} ReminderResponse
// @Failure      401 {object} ErrorResponse "AUTH_REQUIRED, INVALID_TOKEN"
// @Failure      500 {object} ErrorResponse
// @Router       /vitamins/reminders [get]
func (h *Handler) ListReminders(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	ctx := c.Request.Context()
	resp, err := h.s.ListReminders(ctx, userID)
	if err != nil {
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	c.JSON(200, resp)
}

// GetReminder godoc
// @Summary      Детали напоминания
// @Description  Возвращает подробную информацию о напоминании
// @Tags         vitamins
// @Accept       json
// @Produce      json
// @Param        id path int true "ID напоминания"
// @Security     BearerAuth
// @Success      200 {object} ReminderResponse
// @Failure      400 {object} ErrorResponse "INVALID_ID"
// @Failure      401 {object} ErrorResponse "AUTH_REQUIRED, INVALID_TOKEN"
// @Failure      404 {object} ErrorResponse "REMINDER_NOT_FOUND"
// @Failure      500 {object} ErrorResponse
// @Router       /vitamins/reminders/{id} [get]
func (h *Handler) GetReminder(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	resp, err := h.s.GetReminder(ctx, userID, id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(200, resp)
}

// UpdateReminder godoc
// @Summary      Обновить напоминание
// @Description  Обновляет дозу, примечание, условие, расписание, курс и настройки уведомлений
// @Tags         vitamins
// @Accept       json
// @Produce      json
// @Param        id path int true "ID напоминания"
// @Param        request body UpdateReminderRequest true "Данные для обновления"
// @Security     BearerAuth
// @Success      200 {object} ReminderResponse
// @Failure      400 {object} ErrorResponse "INVALID_ID, NO_FIELDS_TO_UPDATE, NAME_REQUIRED, INVALID_FORM, INVALID_CONDITION, INVALID_DAYS, INVALID_TIMES, START_DATE_REQUIRED, INVALID_DATE_FORMAT, INVALID_COURSE_DURATION, TIMEZONE_REQUIRED"
// @Failure      401 {object} ErrorResponse "AUTH_REQUIRED, INVALID_TOKEN"
// @Failure      404 {object} ErrorResponse "REMINDER_NOT_FOUND"
// @Failure      500 {object} ErrorResponse
// @Router       /vitamins/reminders/{id} [patch]
func (h *Handler) UpdateReminder(c *gin.Context) {
	var r UpdateReminderRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	resp, err := h.s.UpdateReminder(ctx, userID, id, r)
	if err != nil {
		h.handleError(c, err)
		return
	}
	logWithContext(c, "audit").InfoContext(ctx, "reminder updated",
		"operation", "vitamins.reminder.update",
		"user_id", userID,
		"reminder_id", resp.ID,
	)
	c.JSON(200, resp)
}

// DeleteReminder godoc
// @Summary      Удалить напоминание
// @Description  Мягко удаляет напоминание (is_active=false)
// @Tags         vitamins
// @Accept       json
// @Produce      json
// @Param        id path int true "ID напоминания"
// @Security     BearerAuth
// @Success      200 {object} ReminderResponse
// @Failure      400 {object} ErrorResponse "INVALID_ID"
// @Failure      401 {object} ErrorResponse "AUTH_REQUIRED, INVALID_TOKEN"
// @Failure      404 {object} ErrorResponse "REMINDER_NOT_FOUND"
// @Failure      500 {object} ErrorResponse
// @Router       /vitamins/reminders/{id} [delete]
func (h *Handler) DeleteReminder(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	resp, err := h.s.SetReminderActive(ctx, userID, id, false)
	if err != nil {
		h.handleError(c, err)
		return
	}
	logWithContext(c, "audit").InfoContext(ctx, "reminder deleted",
		"operation", "vitamins.reminder.delete",
		"user_id", userID,
		"reminder_id", resp.ID,
	)
	c.JSON(200, resp)
}

// EnableReminder godoc
// @Summary      Включить напоминание
// @Description  Устанавливает is_active=true
// @Tags         vitamins
// @Accept       json
// @Produce      json
// @Param        id path int true "ID напоминания"
// @Security     BearerAuth
// @Success      200 {object} ReminderResponse
// @Failure      400 {object} ErrorResponse "INVALID_ID"
// @Failure      401 {object} ErrorResponse "AUTH_REQUIRED, INVALID_TOKEN"
// @Failure      404 {object} ErrorResponse "REMINDER_NOT_FOUND"
// @Failure      500 {object} ErrorResponse
// @Router       /vitamins/reminders/{id}/enable [post]
func (h *Handler) EnableReminder(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	resp, err := h.s.SetReminderActive(ctx, userID, id, true)
	if err != nil {
		h.handleError(c, err)
		return
	}
	logWithContext(c, "audit").InfoContext(ctx, "reminder enabled",
		"operation", "vitamins.reminder.enable",
		"user_id", userID,
		"reminder_id", resp.ID,
	)
	c.JSON(200, resp)
}

// DisableReminder godoc
// @Summary      Отключить напоминание
// @Description  Устанавливает is_active=false
// @Tags         vitamins
// @Accept       json
// @Produce      json
// @Param        id path int true "ID напоминания"
// @Security     BearerAuth
// @Success      200 {object} ReminderResponse
// @Failure      400 {object} ErrorResponse "INVALID_ID"
// @Failure      401 {object} ErrorResponse "AUTH_REQUIRED, INVALID_TOKEN"
// @Failure      404 {object} ErrorResponse "REMINDER_NOT_FOUND"
// @Failure      500 {object} ErrorResponse
// @Router       /vitamins/reminders/{id}/disable [post]
func (h *Handler) DisableReminder(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	resp, err := h.s.SetReminderActive(ctx, userID, id, false)
	if err != nil {
		h.handleError(c, err)
		return
	}
	logWithContext(c, "audit").InfoContext(ctx, "reminder disabled",
		"operation", "vitamins.reminder.disable",
		"user_id", userID,
		"reminder_id", resp.ID,
	)
	c.JSON(200, resp)
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrCatalogNotFound):
		send(c, http.StatusNotFound, "CATALOG_NOT_FOUND", "Витамин не найден")
	case errors.Is(err, ErrNameRequired):
		send(c, http.StatusBadRequest, "NAME_REQUIRED", "Введите название витамина")
	case errors.Is(err, ErrInvalidForm):
		send(c, http.StatusBadRequest, "INVALID_FORM", "Неверная форма препарата")
	case errors.Is(err, ErrInvalidCondition):
		send(c, http.StatusBadRequest, "INVALID_CONDITION", "Неверное условие приема")
	case errors.Is(err, ErrInvalidDays):
		send(c, http.StatusBadRequest, "INVALID_DAYS", "Неверный список дней")
	case errors.Is(err, ErrInvalidTimes):
		send(c, http.StatusBadRequest, "INVALID_TIMES", "Неверное время приема")
	case errors.Is(err, ErrStartDateRequired):
		send(c, http.StatusBadRequest, "START_DATE_REQUIRED", "Введите дату начала")
	case errors.Is(err, ErrInvalidDate):
		send(c, http.StatusBadRequest, "INVALID_DATE_FORMAT", "Неверный формат даты")
	case errors.Is(err, ErrInvalidCourseDuration):
		send(c, http.StatusBadRequest, "INVALID_COURSE_DURATION", "Неверная длительность курса")
	case errors.Is(err, ErrTimezoneRequired):
		send(c, http.StatusBadRequest, "TIMEZONE_REQUIRED", "Укажите часовой пояс")
	case errors.Is(err, ErrReminderNotFound):
		send(c, http.StatusNotFound, "REMINDER_NOT_FOUND", "Напоминание не найдено")
	case errors.Is(err, ErrNoFieldsToUpdate):
		send(c, http.StatusBadRequest, "NO_FIELDS_TO_UPDATE", "Нечего обновлять")
	default:
		logWithContext(c, "app").ErrorContext(c.Request.Context(), "vitamins handler failed",
			"operation", "vitamins.handler.error",
			"error", err.Error(),
		)
		send(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Что-то пошло не так.")
	}
}

func parseID(c *gin.Context, name string) (int64, bool) {
	raw := c.Param(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		send(c, 400, "INVALID_ID", "Неверный идентификатор")
		return 0, false
	}
	return id, true
}
