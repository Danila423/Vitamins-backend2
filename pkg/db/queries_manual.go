package db

import "context"

//nolint:gosec // G101: this is a SQL query, not hardcoded credentials.
const updateUserPassword = `-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $1
WHERE id = $2
`

func (q *Queries) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	_, err := q.db.Exec(ctx, updateUserPassword, passwordHash, userID)
	return err
}
