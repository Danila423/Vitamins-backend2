package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeProperties_RemovesPIIKeysAndRedactsEmails(t *testing.T) {
	t.Parallel()

	in := map[string]any{
		"email": "user@test.local",
		"safe":  "ok",
		"nested": map[string]any{
			"token": "secret",
			"note":  "mail user@test.local",
		},
		"items": []any{"value", "person@test.local"},
	}

	out := sanitizeProperties(in)

	_, hasEmail := out["email"]
	assert.False(t, hasEmail)
	assert.Equal(t, "ok", out["safe"])

	nested, ok := out["nested"].(map[string]any)
	require.True(t, ok)
	_, hasToken := nested["token"]
	assert.False(t, hasToken)
	assert.Equal(t, "[redacted]", nested["note"])

	items, ok := out["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2)
	assert.Equal(t, "value", items[0])
	assert.Equal(t, "[redacted]", items[1])
}

func TestParseUUIDAndTimestamp_Validation(t *testing.T) {
	t.Parallel()

	_, err := parseUUID("bad")
	require.ErrorIs(t, err, ErrInvalidEventID)

	uuid, err := parseUUID("11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	assert.True(t, uuid.Valid)

	_, err = parseTimestamp("bad")
	require.ErrorIs(t, err, ErrInvalidOccurredAt)

	_, err = parseTimestamp("2026-04-22T10:00:00Z")
	require.NoError(t, err)
}
