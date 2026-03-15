package analytics

import (
	"encoding/csv"
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"vitamins-backend_2/internal/auth"
)

type Handler struct {
	s          *Service
	jwt        *auth.JWTManager
	adminToken string
}

func NewHandler(s *Service, jwt *auth.JWTManager, adminToken string) *Handler {
	return &Handler{s: s, jwt: jwt, adminToken: adminToken}
}

func send(c *gin.Context, code int, k, m string) {
	c.JSON(code, ErrorResponse{Code: k, Message: m})
}

// IngestEvents godoc
// @Summary      Прием аналитических событий
// @Description  Принимает батч аналитических событий
// @Tags         analytics
// @Accept       json
// @Produce      json
// @Param        request body BatchRequest true "Batch событий"
// @Success      200 {object} IngestResponse
// @Failure      400 {object} ErrorResponse "EMPTY_BATCH, BATCH_TOO_LARGE, INVALID_EVENT_ID, INVALID_OCCURRED_AT, INVALID_EVENT_NAME, INVALID_SESSION_ID, INVALID_ANONYMOUS_ID, ANONYMOUS_ID_REQUIRED"
// @Failure      401 {object} ErrorResponse "INVALID_TOKEN"
// @Failure      403 {object} ErrorResponse "CONSENT_REQUIRED"
// @Failure      500 {object} ErrorResponse
// @Router       /analytics/events [post]
func (h *Handler) IngestEvents(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	userID, err := h.tryUserID(c)
	if err != nil {
		send(c, 401, "INVALID_TOKEN", "Неверный токен")
		return
	}
	resp, err := h.s.Ingest(c.Request.Context(), userID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(200, resp)
}

// SetConsent godoc
// @Summary      Установить согласие на аналитику
// @Tags         analytics
// @Accept       json
// @Produce      json
// @Param        request body ConsentRequest true "Согласие"
// @Security     BearerAuth
// @Success      200 {object} ConsentResponse
// @Failure      400 {object} ErrorResponse
// @Failure      401 {object} ErrorResponse "AUTH_REQUIRED, INVALID_TOKEN"
// @Router       /analytics/consent [post]
func (h *Handler) SetConsent(c *gin.Context) {
	var req ConsentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	if err := h.s.SetConsent(c.Request.Context(), userID, req.Consent); err != nil {
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	c.JSON(200, ConsentResponse{Consent: req.Consent})
}

// GetConsent godoc
// @Summary      Получить согласие на аналитику
// @Tags         analytics
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} ConsentResponse
// @Failure      401 {object} ErrorResponse "AUTH_REQUIRED, INVALID_TOKEN"
// @Failure      500 {object} ErrorResponse
// @Router       /analytics/consent [get]
func (h *Handler) GetConsent(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	consent, err := h.s.GetConsent(c.Request.Context(), userID)
	if err != nil {
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	c.JSON(200, ConsentResponse{Consent: consent})
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
	if strings.TrimSpace(h.adminToken) == "" {
		send(c, 401, "ADMIN_REQUIRED", "Админ токен не настроен")
		return
	}
	if strings.TrimSpace(c.GetHeader("X-Admin-Token")) != h.adminToken {
		send(c, 401, "ADMIN_REQUIRED", "Требуется админ токен")
		return
	}

	filter := ExportFilter{
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
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}

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
			c.Writer.WriteString(serializeJSONL(row))
		}
	}
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrEmptyBatch):
		send(c, 400, "EMPTY_BATCH", "Пустой батч событий")
	case errors.Is(err, ErrBatchTooLarge):
		send(c, 400, "BATCH_TOO_LARGE", "Слишком много событий в батче")
	case errors.Is(err, ErrInvalidEventID):
		send(c, 400, "INVALID_EVENT_ID", "Неверный event_id")
	case errors.Is(err, ErrInvalidOccurredAt):
		send(c, 400, "INVALID_OCCURRED_AT", "Неверное время события")
	case errors.Is(err, ErrInvalidEventName):
		send(c, 400, "INVALID_EVENT_NAME", "Неверное имя события")
	case errors.Is(err, ErrInvalidSessionID):
		send(c, 400, "INVALID_SESSION_ID", "Неверный session_id")
	case errors.Is(err, ErrInvalidAnonymousID):
		send(c, 400, "INVALID_ANONYMOUS_ID", "Неверный anonymous_id")
	case errors.Is(err, ErrAnonymousRequired):
		send(c, 400, "ANONYMOUS_ID_REQUIRED", "Нужен anonymous_id без consent")
	case errors.Is(err, ErrConsentRequired):
		send(c, 403, "CONSENT_REQUIRED", "Нужно согласие пользователя")
	case errors.Is(err, ErrUserNotFound):
		send(c, 404, "USER_NOT_FOUND", "Пользователь не найден")
	default:
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
	}
}

func (h *Handler) tryUserID(c *gin.Context) (*int64, error) {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if authHeader == "" {
		return nil, nil
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return nil, errors.New("invalid token")
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
	if token == "" {
		return nil, errors.New("invalid token")
	}
	claims, err := h.jwt.Parse(token)
	if err != nil {
		return nil, err
	}
	return &claims.UserID, nil
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

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func maxInt(v, min int) int {
	if v < min {
		return min
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
