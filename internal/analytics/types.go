package analytics

type BatchRequest struct {
	BatchID *string      `json:"batch_id,omitempty"`
	SentAt  *string      `json:"sent_at,omitempty"`
	Events  []EventInput `json:"events"`
}

type EventInput struct {
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

type IngestResponse struct {
	Accepted     int `json:"accepted"`
	Deduplicated int `json:"deduplicated"`
}

type ConsentRequest struct {
	Consent bool `json:"consent"`
}

type ConsentResponse struct {
	Consent bool `json:"consent"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ExportFilter struct {
	From      *string
	To        *string
	EventName *string
	UserID    *int64
	Limit     int
	Offset    int
}

type ExportRow struct {
	EventID     string
	OccurredAt  string
	ReceivedAt  string
	UserID      *int64
	AnonymousID *string
	SessionID   string
	EventName   string
	Properties  string
	RequestID   *string
	AppVersion  *string
	Platform    *string
}
