package service

import (
	"context"

	"vitamins-backend_2/pkg/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SQLCRepository struct {
	*db.Queries
	pool *pgxpool.Pool
}

func NewRepository(q *db.Queries, pool *pgxpool.Pool) *SQLCRepository {
	return &SQLCRepository{Queries: q, pool: pool}
}

func (r *SQLCRepository) InTx(ctx context.Context, fn func(repo ReminderRepository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txRepo := &sqlcTxRepository{Queries: r.Queries.WithTx(tx)}
	if err := fn(txRepo); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type sqlcTxRepository struct {
	*db.Queries
}
