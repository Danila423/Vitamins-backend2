// Package validation owns the small, pure email/password validation rules
// used by the auth service. Keeping them in a dedicated sub-package makes the
// unit tests a one-file affair and keeps the top-level service package from
// ballooning. The parent service package re-exports these symbols so callers
// (AuthService, ProfileService, handlers) keep using the familiar names.
package validation

import (
	"errors"
	"regexp"
	"unicode"
)

var (
	ErrEmailRequired        = errors.New("EMAIL_REQUIRED")
	ErrPasswordRequired     = errors.New("PASSWORD_REQUIRED")
	ErrInvalidEmailFormat   = errors.New("INVALID_EMAIL_FORMAT")
	ErrInvalidPasswordRules = errors.New("INVALID_PASSWORD_FORMAT")
	ErrPasswordMismatch     = errors.New("PASSWORD_CONFIRMATION_MISMATCH")
)

var emailRegexp = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func ValidateEmail(email string) error {
	if email == "" {
		return ErrEmailRequired
	}
	if !emailRegexp.MatchString(email) {
		return ErrInvalidEmailFormat
	}
	return nil
}

func ValidatePassword(password string) error {
	if password == "" {
		return ErrPasswordRequired
	}
	if len(password) < 8 {
		return ErrInvalidPasswordRules
	}
	var letter, digit bool
	for _, r := range password {
		if unicode.IsLetter(r) {
			letter = true
		}
		if unicode.IsDigit(r) {
			digit = true
		}
	}
	if !letter || !digit {
		return ErrInvalidPasswordRules
	}
	return nil
}

func ValidateEmailPassword(email, password string) error {
	if err := ValidateEmail(email); err != nil {
		return err
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	return nil
}
