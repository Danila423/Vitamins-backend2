// Package handler hosts the HTTP layer for the analytics feature. It
// translates gin contexts into calls on service.ServiceAPI and maps domain
// errors back to the exact HTTP+JSON contract the frontend already depends
// on.
package handler

import (
	"encoding/csv"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"vitamins-backend_2/internal/analytics/service"
	"vitamins-backend_2/internal/auth"
	appLogger "vitamins-backend_2/internal/logger"

	"github.com/gin-gonic/gin"
)

// ErrorResponse is the public JSON shape returned for any analytics error. It
// is exported so the swagger/docs generators and external callers can depend
// on its schema.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Handler struct {
	s service.ServiceAPI
}

// NewHandler creates a new analytics handler.
// JWT parsing for optional auth is now performed by auth.OptionalAuthMiddleware.
// Admin-token protection for Export is enforced by auth.AdminTokenMiddleware.
func NewHandler(s service.ServiceAPI) *Handler {
	return &Handler{s: s}
}

func send(c *gin.Context, code int, k, m string) {
	c.JSON(code, ErrorResponse{Code: k, Message: m})
}

func logWithContext(c *gin.Context, channel string) *slog.Logger {
	return appLogger.WithContext(slog.Default(), c.Request.Context()).With("channel", channel)
}

// IngestEvents godoc
// @Summary      Прием аналитических событий
// @Description  Принимает батч аналитических событий
// @Tags         analytics
// @Accept       json
// @Produce      json
// @Param        request body service.BatchRequest true "Batch событий"
// @Success      200 {object} service.IngestResponse
// @Failure      400 {object} ErrorResponse "EMPTY_BATCH, BATCH_TOO_LARGE, INVALID_EVENT_ID, INVALID_OCCURRED_AT, INVALID_EVENT_NAME, INVALID_SESSION_ID, INVALID_ANONYMOUS_ID, ANONYMOUS_ID_REQUIRED"
// @Failure      401 {object} ErrorResponse "INVALID_TOKEN"
// @Failure      403 {object} ErrorResponse "CONSENT_REQUIRED"
// @Failure      500 {object} ErrorResponse
// @Router       /analytics/events [post]
func (h *Handler) IngestEvents(c *gin.Context) {
	var req service.BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		send(c, http.StatusBadRequest, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	var userID *int64
	if id, ok := auth.UserIDFromContext(c); ok {
		userID = &id
	}
	resp, err := h.s.Ingest(c.Request.Context(), userID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	logWithContext(c, "app").InfoContext(c.Request.Context(), "analytics events ingested",
		"operation", "analytics.ingest",
		"accepted", resp.Accepted,
		"deduplicated", resp.Deduplicated,
	)
	c.JSON(http.StatusOK, resp)
}

// SetConsent godoc
// @Summary      Установить согласие на аналитику
// @Tags         analytics
// @Accept       json
// @Produce      json
// @Param        request body service.ConsentRequest true "Согласие"
// @Security     BearerAuth
// @Success      200 {object} service.ConsentResponse
// @Failure      400 {object} ErrorResponse
// @Failure      401 {object} ErrorResponse "AUTH_REQUIRED, INVALID_TOKEN"
// @Router       /analytics/consent [post]
func (h *Handler) SetConsent(c *gin.Context) {
	var req service.ConsentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		send(c, http.StatusBadRequest, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		send(c, http.StatusUnauthorized, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	if err := h.s.SetConsent(c.Request.Context(), userID, req.Consent); err != nil {
		logWithContext(c, "app").ErrorContext(c.Request.Context(), "analytics consent update failed",
			"operation", "analytics.consent.set",
			"user_id", userID,
			"error", err.Error(),
		)
		send(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	logWithContext(c, "audit").InfoContext(c.Request.Context(), "analytics consent updated",
		"operation", "analytics.consent.set",
		"user_id", userID,
		"consent", req.Consent,
	)
	c.JSON(http.StatusOK, service.ConsentResponse(req))
}

// GetConsent godoc
// @Summary      Получить согласие на аналитику
// @Tags         analytics
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} service.ConsentResponse
// @Failure      401 {object} ErrorResponse "AUTH_REQUIRED, INVALID_TOKEN"
// @Failure      500 {object} ErrorResponse
// @Router       /analytics/consent [get]
func (h *Handler) GetConsent(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		send(c, http.StatusUnauthorized, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	consent, err := h.s.GetConsent(c.Request.Context(), userID)
	if err != nil {
		logWithContext(c, "app").ErrorContext(c.Request.Context(), "analytics consent read failed",
			"operation", "analytics.consent.get",
			"user_id", userID,
			"error", err.Error(),
		)
		send(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	c.JSON(http.StatusOK, service.ConsentResponse{Consent: consent})
}

// Export godoc
// @Summary      Экспорт аналитики
// @Tags         admin
// @Produce      text/csv,application/json
// @Param        from query string false "RFC3339 from"
// @Param        to query string false "RFC3339 to"
// @Param        event query string false "event_name"
// @Param        user_id query int false "user_id"
// @Param        format query string false "csv|jsonl"
// @Param        limit query int false "limit"
// @Param        offset query int false "offset"
// @Success      200 {string} string "CSV/JSONL"
// @Failure      401 {object} ErrorResponse "ADMIN_REQUIRED"
// @Failure      500 {object} ErrorResponse
// @Router       /admin/analytics/export [get]
func (h *Handler) Export(c *gin.Context) {
	filter := service.ExportFilter{
		From:      strPtr(c.Query("from")),
		To:        strPtr(c.Query("to")),
		EventName: strPtr(c.Query("event")),
		UserID:    parseInt64Ptr(c.Query("user_id")),
		Limit:     clampInt(parseIntDefault(c.Query("limit"), 10000), 1, 100000),
		Offset:    maxInt(parseIntDefault(c.Query("offset"), 0), 0),
	}
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "jsonl")))

	rows, err := h.s.Export(c.Request.Context(), filter)
	if err != nil {
		logWithContext(c, "app").ErrorContext(c.Request.Context(), "analytics export failed",
			"operation", "analytics.export",
			"error", err.Error(),
		)
		send(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	logWithContext(c, "audit").InfoContext(c.Request.Context(), "analytics exported",
		"operation", "analytics.export",
		"rows", len(rows),
		"format", format,
	)

	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv; charset=utf-8")
		writer := csv.NewWriter(c.Writer)
		_ = writer.Write([]string{
			"event_id", "occurred_at", "received_at", "user_id", "anonymous_id",
			"session_id", "event_name", "properties", "request_id", "app_version", "platform",
		})
		for _, row := range rows {
			userID := ""
			if row.UserID != nil {
				userID = strconv.FormatInt(*row.UserID, 10)
			}
			anonID := ""
			if row.AnonymousID != nil {
				anonID = *row.AnonymousID
			}
			requestID := ""
			if row.RequestID != nil {
				requestID = *row.RequestID
			}
			appVersion := ""
			if row.AppVersion != nil {
				appVersion = *row.AppVersion
			}
			platform := ""
			if row.Platform != nil {
				platform = *row.Platform
			}
			_ = writer.Write([]string{
				row.EventID,
				row.OccurredAt,
				row.ReceivedAt,
				userID,
				anonID,
				row.SessionID,
				row.EventName,
				row.Properties,
				requestID,
				appVersion,
				platform,
			})
		}
		writer.Flush()
		return
	default:
		c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
		for _, row := range rows {
			_, _ = c.Writer.WriteString(service.SerializeJSONL(row))
		}
	}
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrEmptyBatch):
		send(c, http.StatusBadRequest, "EMPTY_BATCH", "Пустой батч событий")
	case errors.Is(err, service.ErrBatchTooLarge):
		send(c, http.StatusBadRequest, "BATCH_TOO_LARGE", "Слишком много событий в батче")
	case errors.Is(err, service.ErrInvalidEventID):
		send(c, http.StatusBadRequest, "INVALID_EVENT_ID", "Неверный event_id")
	case errors.Is(err, service.ErrInvalidOccurredAt):
		send(c, http.StatusBadRequest, "INVALID_OCCURRED_AT", "Неверное время события")
	case errors.Is(err, service.ErrInvalidEventName):
		send(c, http.StatusBadRequest, "INVALID_EVENT_NAME", "Неверное имя события")
	case errors.Is(err, service.ErrInvalidSessionID):
		send(c, http.StatusBadRequest, "INVALID_SESSION_ID", "Неверный session_id")
	case errors.Is(err, service.ErrInvalidAnonymousID):
		send(c, http.StatusBadRequest, "INVALID_ANONYMOUS_ID", "Неверный anonymous_id")
	case errors.Is(err, service.ErrAnonymousRequired):
		send(c, http.StatusBadRequest, "ANONYMOUS_ID_REQUIRED", "Нужен anonymous_id без consent")
	case errors.Is(err, service.ErrConsentRequired):
		send(c, http.StatusForbidden, "CONSENT_REQUIRED", "Нужно согласие пользователя")
	case errors.Is(err, service.ErrUserNotFound):
		send(c, http.StatusNotFound, "USER_NOT_FOUND", "Пользователь не найден")
	default:
		logWithContext(c, "app").ErrorContext(c.Request.Context(), "analytics handler failed",
			"operation", "analytics.handler.error",
			"error", err.Error(),
		)
		send(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Что-то пошло не так.")
	}
}

func parseIntDefault(raw string, def int) int {
	if strings.TrimSpace(raw) == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

func parseInt64Ptr(raw string) *int64 {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(v, lo int) int {
	if v < lo {
		return lo
	}
	return v
}

func strPtr(v string) *string {
	value := strings.TrimSpace(v)
	if value == "" {
		return nil
	}
	return &value
}
