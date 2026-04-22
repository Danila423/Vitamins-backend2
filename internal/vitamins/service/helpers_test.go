package service

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func TestParseDaysMaskAndBack(t *testing.T) {
	t.Parallel()

	mask, err := parseDaysMask([]string{"mon", "wed", "sun"})
	require.NoError(t, err)
	assert.Equal(t, int16((1<<0)|(1<<2)|(1<<6)), mask)

	got := daysFromMask(mask)
	want := []string{"mon", "wed", "sun"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("days mismatch (-want +got):\n%s", diff)
	}
}

func TestParseDaysMaskInvalid(t *testing.T) {
	t.Parallel()
	_, err := parseDaysMask([]string{"monday"})
	assert.ErrorIs(t, err, ErrInvalidDays)
}

func TestParseFormAndDefaultUnit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		form string
		unit string
	}{
		{form: "tablet", unit: "шт"},
		{form: "capsule", unit: "шт"},
		{form: "drops", unit: "капли"},
		{form: "powder", unit: "г"},
		{form: "chewable_tablet", unit: "шт"},
		{form: "liquid", unit: "мл"},
		{form: "ampoule", unit: "шт"},
		{form: "spray", unit: "нажатия"},
		{form: "injection", unit: "шт"},
		{form: "other", unit: "шт"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.form, func(t *testing.T) {
			t.Parallel()
			v, err := parseForm(ptr(tt.form))
			require.NoError(t, err)
			assert.Equal(t, tt.unit, defaultUnitForForm(v))
		})
	}
}

func TestValidateTimes(t *testing.T) {
	t.Parallel()

	ok, err := validateTimes([]string{"09:00", "21:30"})
	require.NoError(t, err)
	assert.Len(t, ok, 2)

	_, err = validateTimes([]string{"09:00", "09:00"})
	require.ErrorIs(t, err, ErrInvalidTimes)

	_, err = validateTimes([]string{"99:99"})
	require.ErrorIs(t, err, ErrInvalidTimes)
}
