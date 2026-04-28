package service

import (
	"context"
	"errors"
	"strings"
)

type ProfileService struct {
	users UserRepository
}

type ProfileUpdate struct {
	FirstName *string
	LastName  *string
	Email     *string
}

func (s *ProfileService) GetProfile(ctx context.Context, userID int64) (UserProfile, error) {
	if userID == 0 {
		return UserProfile{}, ErrUserNotFound
	}
	u, err := s.users.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return UserProfile{}, ErrUserNotFound
		}
		return UserProfile{}, err
	}
	return toUserProfile(u), nil
}

func (s *ProfileService) UpdateProfile(ctx context.Context, userID int64, in ProfileUpdate) (UserProfile, error) {
	if in.FirstName == nil && in.LastName == nil && in.Email == nil {
		return UserProfile{}, ErrNoFieldsToUpdate
	}
	if userID == 0 {
		return UserProfile{}, ErrUserNotFound
	}

	u, err := s.users.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return UserProfile{}, ErrUserNotFound
		}
		return UserProfile{}, err
	}

	nextEmail := u.Email
	if in.Email != nil {
		normEmail := strings.ToLower(strings.TrimSpace(*in.Email))
		if err := ValidateEmail(normEmail); err != nil {
			return UserProfile{}, err
		}
		nextEmail = normEmail
		if strings.ToLower(u.Email) != normEmail {
			existing, err := s.users.GetUserByEmail(ctx, normEmail)
			if err == nil && existing.ID != userID {
				return UserProfile{}, ErrEmailAlreadyExists
			}
			if err != nil && !errors.Is(err, ErrUserNotFound) {
				return UserProfile{}, err
			}
		}
	}

	nextFirstName := u.FirstName
	if in.FirstName != nil {
		nextFirstName = strings.TrimSpace(*in.FirstName)
	}
	nextLastName := u.LastName
	if in.LastName != nil {
		nextLastName = strings.TrimSpace(*in.LastName)
	}

	updated, err := s.users.UpdateUserProfile(ctx, userID, nextEmail, nextFirstName, nextLastName)
	if err != nil {
		if errors.Is(err, ErrEmailConflict) {
			return UserProfile{}, ErrEmailAlreadyExists
		}
		return UserProfile{}, err
	}
	return toUserProfile(updated), nil
}

func toUserProfile(u User) UserProfile {
	return UserProfile{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
	}
}
