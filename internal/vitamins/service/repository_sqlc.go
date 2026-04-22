package service

import (
	"context"

	"vitamins-backend_2/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SQLCRepository implements ReminderRepository using sqlc-generated *db.Queries.
// Both the pool-bound and tx-bound variants share their entire CRUD surface by
// embedding *db.Queries; this removes the previous large duplication between
// SQLCRepository and sqlcTxRepository.
type SQLCRepository struct {
	*db.Queries
	pool *pgxpool.Pool
}

func NewRepository(q *db.Queries, pool *pgxpool.Pool) *SQLCRepository {
	return &SQLCRepository{Queries: q, pool: pool}
}

// InTx runs fn inside a single database transaction. The repository handed to
// fn is bound to the same transaction, so all writes either commit together or
// roll back together.
func (r *SQLCRepository) InTx(ctx context.Context, fn func(repo ReminderRepository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	txRepo := &sqlcTxRepository{Queries: r.Queries.WithTx(tx)}
	if err := fn(txRepo); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type sqlcTxRepository struct {
	*db.Queries
}
