package rabbitmq

const (
	ExchangeEvents = "vitamins.events"

	RoutingKeyPasswordResetRequested  = "auth.password_reset.requested"
	RoutingKeyPasswordChangeRequested = "auth.password_change.requested"

	RoutingKeyAnalyticsEvent = "analytics.event"

	QueueNotifications = "notifications"
	QueueAnalytics     = "analytics"
)

type PasswordResetEvent struct {
	Email   string `json:"email"`
	Subject string `json:"subject"`
	Code    string `json:"code"`
}

type PasswordChangeEvent struct {
	Email   string `json:"email"`
	Subject string `json:"subject"`
	Code    string `json:"code"`
}

// AnalyticsEventInput mirrors service.EventInput fields so the payload is
// identical to what the gateway already accepts from the frontend. Kept here
// (not imported from services/analytics) to avoid cross-service dependencies.
type AnalyticsEventInput struct {
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

// AnalyticsBatch is the envelope published to RabbitMQ when the gateway
// decouples analytics ingestion from the analytics-service gRPC call.
type AnalyticsBatch struct {
	TokenUserID *int64                `json:"token_user_id,omitempty"`
	BatchID     *string               `json:"batch_id,omitempty"`
	SentAt      *string               `json:"sent_at,omitempty"`
	Events      []AnalyticsEventInput `json:"events"`
}
