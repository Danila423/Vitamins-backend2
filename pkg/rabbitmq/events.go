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

type AnalyticsBatch struct {
	TokenUserID *int64                `json:"token_user_id,omitempty"`
	BatchID     *string               `json:"batch_id,omitempty"`
	SentAt      *string               `json:"sent_at,omitempty"`
	Events      []AnalyticsEventInput `json:"events"`
}
