package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	analyticsv1 "vitamins-backend_2/gen/go/analytics/v1"
	"vitamins-backend_2/pkg/rabbitmq"
	"vitamins-backend_2/services/gateway/internal/middleware"
)

// EventPublisher is the minimal contract required by AnalyticsHandler to
// decouple ingestion from the synchronous gRPC call. It is satisfied by
// *rabbitmq.Publisher.
type EventPublisher interface {
	Publish(ctx context.Context, exchange, routingKey string, msg any) error
}

type AnalyticsHandler struct {
	client    analyticsv1.AnalyticsServiceClient
	publisher EventPublisher
}

func NewAnalyticsHandler(client analyticsv1.AnalyticsServiceClient) *AnalyticsHandler {
	return &AnalyticsHandler{client: client}
}

// WithEventPublisher enables async ingestion via RabbitMQ. When set, the
// handler publishes the batch to the broker and returns immediately instead
// of calling analytics-service synchronously.
func (h *AnalyticsHandler) WithEventPublisher(p EventPublisher) *AnalyticsHandler {
	h.publisher = p
	return h
}

type batchRequestJSON struct {
	BatchID *string          `json:"batch_id,omitempty"`
	SentAt  *string          `json:"sent_at,omitempty"`
	Events  []eventInputJSON `json:"events"`
}

type eventInputJSON struct {
	EventID     string         `json:"event_id"`
	OccurredAt  string         `json:"occurred_at"`
	EventName   string         `json:"event_name"`
	SessionID   string         `json:"session_id"`
	UserID      *int64         `json:"user_id,omitempty"`
	AnonymousID *string        `json:"anonymous_id,omitempty"`
	Properties  map[string]any `json:"properties,omitempty"`
	RequestID   *string        `json:"request_id,omitempty"`
	AppVersion  *string        `json:"app_version,omitempty"`
	Platform    *string        `json:"platform,omitempty"`
}

type ingestResponseJSON struct {
	Accepted     int `json:"accepted"`
	Deduplicated int `json:"deduplicated"`
}

type consentRequestJSON struct {
	Consent bool `json:"consent"`
}

type consentResponseJSON struct {
	Consent bool `json:"consent"`
}

func (h *AnalyticsHandler) IngestEvents(c *gin.Context) {
	var req batchRequestJSON
	if err := c.ShouldBindJSON(&req); err != nil {
		send(c, http.StatusBadRequest, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	var userID *int64
	if id, ok := middleware.UserIDFromContext(c); ok {
		userID = &id
	}

	if h.publisher != nil {
		if len(req.Events) == 0 {
			send(c, http.StatusBadRequest, "EMPTY_BATCH", "Пустой батч событий")
			return
		}
		events := make([]rabbitmq.AnalyticsEventInput, 0, len(req.Events))
		for _, e := range req.Events {
			events = append(events, rabbitmq.AnalyticsEventInput{
				EventID:     e.EventID,
				OccurredAt:  e.OccurredAt,
				EventName:   e.EventName,
				SessionID:   e.SessionID,
				UserID:      e.UserID,
				AnonymousID: e.AnonymousID,
				Properties:  e.Properties,
				RequestID:   e.RequestID,
				AppVersion:  e.AppVersion,
				Platform:    e.Platform,
			})
		}
		payload := rabbitmq.AnalyticsBatch{
			TokenUserID: userID,
			BatchID:     req.BatchID,
			SentAt:      req.SentAt,
			Events:      events,
		}
		if err := h.publisher.Publish(c.Request.Context(), rabbitmq.ExchangeEvents, rabbitmq.RoutingKeyAnalyticsEvent, payload); err != nil {
			logApp(c).ErrorContext(c.Request.Context(), "analytics publish failed",
				"operation", "analytics.ingest.publish", "error", err.Error())
			send(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Что-то пошло не так.")
			return
		}
		logApp(c).InfoContext(c.Request.Context(), "analytics events published",
			"operation", "analytics.ingest.publish",
			"events", len(events),
		)
		c.JSON(http.StatusOK, ingestResponseJSON{
			Accepted:     len(events),
			Deduplicated: 0,
		})
		return
	}

	pbEvents := make([]*analyticsv1.EventInput, 0, len(req.Events))
	for _, e := range req.Events {
		pbEvent := &analyticsv1.EventInput{
			EventId:     e.EventID,
			OccurredAt:  e.OccurredAt,
			EventName:   e.EventName,
			SessionId:   e.SessionID,
			UserId:      e.UserID,
			AnonymousId: e.AnonymousID,
			RequestId:   e.RequestID,
			AppVersion:  e.AppVersion,
			Platform:    e.Platform,
		}
		if e.Properties != nil {
			if b, err := json.Marshal(e.Properties); err == nil {
				pbEvent.PropertiesJson = b
			}
		}
		pbEvents = append(pbEvents, pbEvent)
	}

	pbReq := &analyticsv1.IngestRequest{
		UserId: userID,
		Batch: &analyticsv1.BatchRequest{
			BatchId: req.BatchID,
			SentAt:  req.SentAt,
			Events:  pbEvents,
		},
	}

	resp, err := h.client.Ingest(c.Request.Context(), pbReq)
	if err != nil {
		handleAnalyticsError(c, err)
		return
	}
	logApp(c).InfoContext(c.Request.Context(), "analytics events ingested",
		"operation", "analytics.ingest",
		"accepted", resp.GetAccepted(),
		"deduplicated", resp.GetDeduplicated(),
	)
	c.JSON(http.StatusOK, ingestResponseJSON{
		Accepted:     int(resp.GetAccepted()),
		Deduplicated: int(resp.GetDeduplicated()),
	})
}

func (h *AnalyticsHandler) SetConsent(c *gin.Context) {
	var req consentRequestJSON
	if err := c.ShouldBindJSON(&req); err != nil {
		send(c, http.StatusBadRequest, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		send(c, http.StatusUnauthorized, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	_, err := h.client.SetConsent(c.Request.Context(), &analyticsv1.SetConsentRequest{
		UserId:  userID,
		Consent: req.Consent,
	})
	if err != nil {
		logApp(c).ErrorContext(c.Request.Context(), "analytics consent update failed",
			"operation", "analytics.consent.set",
			"user_id", userID,
			"error", err.Error(),
		)
		send(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	logAudit(c).InfoContext(c.Request.Context(), "analytics consent updated",
		"operation", "analytics.consent.set",
		"user_id", userID,
		"consent", req.Consent,
	)
	c.JSON(http.StatusOK, consentResponseJSON(req))
}

func (h *AnalyticsHandler) GetConsent(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		send(c, http.StatusUnauthorized, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	resp, err := h.client.GetConsent(c.Request.Context(), &analyticsv1.GetConsentRequest{UserId: userID})
	if err != nil {
		logApp(c).ErrorContext(c.Request.Context(), "analytics consent read failed",
			"operation", "analytics.consent.get",
			"user_id", userID,
			"error", err.Error(),
		)
		send(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	c.JSON(http.StatusOK, consentResponseJSON{Consent: resp.GetConsent()})
}

func (h *AnalyticsHandler) Export(c *gin.Context) {
	pbReq := &analyticsv1.ExportRequest{
		From:      strPtr(c.Query("from")),
		To:        strPtr(c.Query("to")),
		EventName: strPtr(c.Query("event")),
		UserId:    parseInt64Ptr(c.Query("user_id")),
		Limit:     safeInt32(clampInt(parseIntDefault(c.Query("limit"), 10000), 1, 100000)),
		Offset:    safeInt32(maxInt(parseIntDefault(c.Query("offset"), 0), 0)),
	}
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "jsonl")))

	resp, err := h.client.Export(c.Request.Context(), pbReq)
	if err != nil {
		logApp(c).ErrorContext(c.Request.Context(), "analytics export failed",
			"operation", "analytics.export",
			"error", err.Error(),
		)
		send(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	rows := resp.GetRows()
	logAudit(c).InfoContext(c.Request.Context(), "analytics exported",
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
			uid := ""
			if row.UserId != nil {
				uid = strconv.FormatInt(*row.UserId, 10)
			}
			anonID := ""
			if row.AnonymousId != nil {
				anonID = *row.AnonymousId
			}
			requestID := ""
			if row.RequestId != nil {
				requestID = *row.RequestId
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
				row.GetEventId(),
				row.GetOccurredAt(),
				row.GetReceivedAt(),
				uid,
				anonID,
				row.GetSessionId(),
				row.GetEventName(),
				row.GetProperties(),
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
			b, err := json.Marshal(row)
			if err != nil {
				_, _ = c.Writer.WriteString("{}\n")
				continue
			}
			_, _ = c.Writer.WriteString(string(b) + "\n")
		}
	}
}

func handleAnalyticsError(c *gin.Context, err error) {
	st, ok := status.FromError(err)
	if !ok {
		logApp(c).ErrorContext(c.Request.Context(), "analytics handler failed",
			"operation", "analytics.handler.error", "error", err.Error())
		send(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	msg := st.Message()
	switch st.Code() {
	case codes.InvalidArgument:
		switch {
		case strings.Contains(msg, "EMPTY_BATCH"):
			send(c, http.StatusBadRequest, "EMPTY_BATCH", "Пустой батч событий")
		case strings.Contains(msg, "BATCH_TOO_LARGE"):
			send(c, http.StatusBadRequest, "BATCH_TOO_LARGE", "Слишком много событий в батче")
		case strings.Contains(msg, "INVALID_EVENT_ID"):
			send(c, http.StatusBadRequest, "INVALID_EVENT_ID", "Неверный event_id")
		case strings.Contains(msg, "INVALID_OCCURRED_AT"):
			send(c, http.StatusBadRequest, "INVALID_OCCURRED_AT", "Неверное время события")
		case strings.Contains(msg, "INVALID_EVENT_NAME"):
			send(c, http.StatusBadRequest, "INVALID_EVENT_NAME", "Неверное имя события")
		case strings.Contains(msg, "INVALID_SESSION_ID"):
			send(c, http.StatusBadRequest, "INVALID_SESSION_ID", "Неверный session_id")
		case strings.Contains(msg, "INVALID_ANONYMOUS_ID"):
			send(c, http.StatusBadRequest, "INVALID_ANONYMOUS_ID", "Неверный anonymous_id")
		case strings.Contains(msg, "ANONYMOUS_ID_REQUIRED"):
			send(c, http.StatusBadRequest, "ANONYMOUS_ID_REQUIRED", "Нужен anonymous_id без consent")
		default:
			send(c, http.StatusBadRequest, "BAD_REQUEST", msg)
		}
	case codes.PermissionDenied:
		send(c, http.StatusForbidden, "CONSENT_REQUIRED", "Нужно согласие пользователя")
	case codes.NotFound:
		send(c, http.StatusNotFound, "USER_NOT_FOUND", "Пользователь не найден")
	default:
		logApp(c).ErrorContext(c.Request.Context(), "analytics handler failed",
			"operation", "analytics.handler.error", "error", err.Error())
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

func safeInt32(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}
