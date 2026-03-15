package analytics

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrEmptyBatch         = errors.New("EMPTY_BATCH")
	ErrBatchTooLarge      = errors.New("BATCH_TOO_LARGE")
	ErrInvalidEventID     = errors.New("INVALID_EVENT_ID")
	ErrInvalidOccurredAt  = errors.New("INVALID_OCCURRED_AT")
	ErrInvalidEventName   = errors.New("INVALID_EVENT_NAME")
	ErrInvalidSessionID   = errors.New("INVALID_SESSION_ID")
	ErrInvalidAnonymousID = errors.New("INVALID_ANONYMOUS_ID")
	ErrAnonymousRequired  = errors.New("ANONYMOUS_ID_REQUIRED")
	ErrConsentRequired    = errors.New("CONSENT_REQUIRED")
	ErrUserNotFound       = errors.New("USER_NOT_FOUND")
)

const (
	maxBatchSize = 100
)

var piiKeys = map[string]struct{}{
	"email":         {},
	"user_email":    {},
	"phone":         {},
	"phone_number":  {},
	"password":      {},
	"token":         {},
	"access_token":  {},
	"refresh_token": {},
	"authorization": {},
	"auth":          {},
	"session_token": {},
	"api_key":       {},
	"secret":        {},
}

func parseUUID(raw string) (pgtype.UUID, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return pgtype.UUID{}, ErrInvalidEventID
	}
	clean := strings.ReplaceAll(value, "-", "")
	if len(clean) != 32 {
		return pgtype.UUID{}, ErrInvalidEventID
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil || len(decoded) != 16 {
		return pgtype.UUID{}, ErrInvalidEventID
	}
	var b [16]byte
	copy(b[:], decoded)
	return pgtype.UUID{Bytes: b, Valid: true}, nil
}

func parseUUIDAllowEmpty(raw string, errOnEmpty error) (pgtype.UUID, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return pgtype.UUID{}, errOnEmpty
	}
	clean := strings.ReplaceAll(value, "-", "")
	if len(clean) != 32 {
		return pgtype.UUID{}, errOnEmpty
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil || len(decoded) != 16 {
		return pgtype.UUID{}, errOnEmpty
	}
	var b [16]byte
	copy(b[:], decoded)
	return pgtype.UUID{Bytes: b, Valid: true}, nil
}

func parseTimestamp(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, ErrInvalidOccurredAt
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t, err = time.Parse(time.RFC3339, value)
	}
	if err != nil {
		return time.Time{}, ErrInvalidOccurredAt
	}
	return t, nil
}

func sanitizeProperties(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	result := make(map[string]any, len(input))
	for k, v := range input {
		key := strings.ToLower(strings.TrimSpace(k))
		if _, blocked := piiKeys[key]; blocked {
			continue
		}
		result[k] = sanitizeValue(v)
	}
	return result
}

func sanitizeValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return sanitizeProperties(t)
	case []any:
		out := make([]any, 0, len(t))
		for _, item := range t {
			out = append(out, sanitizeValue(item))
		}
		return out
	case string:
		return redactIfPIIValue(t)
	default:
		return v
	}
}

func redactIfPIIValue(v string) string {
	if looksLikeEmail(v) {
		return "[redacted]"
	}
	return v
}

func looksLikeEmail(v string) bool {
	value := strings.TrimSpace(v)
	at := strings.Index(value, "@")
	if at <= 0 {
		return false
	}
	dot := strings.LastIndex(value, ".")
	return dot > at+1 && dot < len(value)-1
}

func uuidToString(v pgtype.UUID) string {
	if !v.Valid {
		return ""
	}
	b := v.Bytes
	return strings.ToLower(hex.EncodeToString(b[:4])) + "-" +
		strings.ToLower(hex.EncodeToString(b[4:6])) + "-" +
		strings.ToLower(hex.EncodeToString(b[6:8])) + "-" +
		strings.ToLower(hex.EncodeToString(b[8:10])) + "-" +
		strings.ToLower(hex.EncodeToString(b[10:16]))
}

func serializeJSONL(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}\n"
	}
	return string(b) + "\n"
}
