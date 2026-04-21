package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		email   string
		wantErr error
	}{
		{name: "ok", email: "user@example.com", wantErr: nil},
		{name: "empty", email: "", wantErr: ErrEmailRequired},
		{name: "invalid", email: "invalid", wantErr: ErrInvalidEmailFormat},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateEmail(tt.email)
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestValidatePassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{name: "ok", password: "Passw0rd", wantErr: nil},
		{name: "empty", password: "", wantErr: ErrPasswordRequired},
		{name: "short", password: "P1a", wantErr: ErrInvalidPasswordRules},
		{name: "no digit", password: "Password", wantErr: ErrInvalidPasswordRules},
		{name: "no letter", password: "12345678", wantErr: ErrInvalidPasswordRules},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidatePassword(tt.password)
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
