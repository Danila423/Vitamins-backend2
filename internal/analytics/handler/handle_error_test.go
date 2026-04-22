package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vitamins-backend_2/internal/analytics/service"

	"github.com/gin-gonic/gin"
)

// TestHandleError_MapsAllDomainErrors exercises every branch of (*Handler).handleError
// to make sure that domain errors keep producing the JSON shape and HTTP codes
// that the frontend already depends on. We rely on errors.Is, so wrapped errors
// are also exercised.
func TestHandleError_MapsAllDomainErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}

	cases := []struct {
		name     string
		err      error
		wantCode int
		wantKey  string
	}{
		{"empty batch", service.ErrEmptyBatch, http.StatusBadRequest, "EMPTY_BATCH"},
		{"batch too large", service.ErrBatchTooLarge, http.StatusBadRequest, "BATCH_TOO_LARGE"},
		{"invalid event id", service.ErrInvalidEventID, http.StatusBadRequest, "INVALID_EVENT_ID"},
		{"invalid occurred at", service.ErrInvalidOccurredAt, http.StatusBadRequest, "INVALID_OCCURRED_AT"},
		{"invalid event name", service.ErrInvalidEventName, http.StatusBadRequest, "INVALID_EVENT_NAME"},
		{"invalid session id", service.ErrInvalidSessionID, http.StatusBadRequest, "INVALID_SESSION_ID"},
		{"invalid anonymous id", service.ErrInvalidAnonymousID, http.StatusBadRequest, "INVALID_ANONYMOUS_ID"},
		{"anonymous required", service.ErrAnonymousRequired, http.StatusBadRequest, "ANONYMOUS_ID_REQUIRED"},
		{"consent required", service.ErrConsentRequired, http.StatusForbidden, "CONSENT_REQUIRED"},
		{"user not found", service.ErrUserNotFound, http.StatusNotFound, "USER_NOT_FOUND"},
		{"unknown -> 500", errors.New("boom"), http.StatusInternalServerError, "INTERNAL_ERROR"},
		{"wrapped consent required", wrap(service.ErrConsentRequired), http.StatusForbidden, "CONSENT_REQUIRED"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

			h.handleError(c, tc.err)

			if rec.Code != tc.wantCode {
				t.Fatalf("status: want %d, got %d (body=%s)", tc.wantCode, rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, `"code":"`+tc.wantKey+`"`) {
				t.Fatalf("body should contain code=%s, got %s", tc.wantKey, body)
			}
		})
	}
}

func wrap(err error) error {
	return &wrappedErr{inner: err}
}

type wrappedErr struct{ inner error }

func (w *wrappedErr) Error() string { return "wrapped: " + w.inner.Error() }
func (w *wrappedErr) Unwrap() error { return w.inner }
