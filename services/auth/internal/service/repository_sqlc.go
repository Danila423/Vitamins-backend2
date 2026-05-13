package service

import (
	"context"
	"errors"

	"vitamins-backend_2/pkg/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type sqlcUserRepository struct {
	q *db.Queries
}

func newSQLCUserRepository(q *db.Queries) UserRepository {
	return &sqlcUserRepository{q: q}
}

func fromDBUser(u db.User) User {
	return User{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		FirstName:    u.FirstName,
		LastName:     u.LastName,
	}
}

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUserNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrEmailConflict
	}
	return err
}

func (r *sqlcUserRepository) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	u, err := r.q.CreateUser(ctx, db.CreateUserParams{Email: email, PasswordHash: passwordHash})
	if err != nil {
		return User{}, mapDBError(err)
	}
	return fromDBUser(u), nil
}

func (r *sqlcUserRepository) GetUserByEmail(ctx context.Context, email string) (User, error) {
	u, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, mapDBError(err)
	}
	return fromDBUser(u), nil
}

func (r *sqlcUserRepository) GetUserByID(ctx context.Context, userID int64) (User, error) {
	u, err := r.q.GetUserByID(ctx, userID)
	if err != nil {
		return User{}, mapDBError(err)
	}
	return fromDBUser(u), nil
}

func (r *sqlcUserRepository) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	return mapDBError(r.q.UpdateUserPassword(ctx, userID, passwordHash))
}

func (r *sqlcUserRepository) UpdateUserProfile(ctx context.Context, userID int64, email, firstName, lastName string) (User, error) {
	u, err := r.q.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
		ID:        userID,
	})
	if err != nil {
		return User{}, mapDBError(err)
	}
	return fromDBUser(u), nil
}

func (r *sqlcUserRepository) DeleteUser(ctx context.Context, userID int64) error {
	rows, err := r.q.DeleteUserByID(ctx, userID)
	if err != nil {
		return mapDBError(err)
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}
