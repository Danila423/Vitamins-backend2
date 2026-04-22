package db

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// Apply runs every embedded migration file in lexicographic order. Migrations
// are written to be idempotent (CREATE TABLE IF NOT EXISTS, …) so they can be
// re-applied safely. This is a deliberately small replacement for goose/migrate
// — it keeps the runtime dependency surface minimal while letting tests apply
// the same SQL the production database uses.
func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := embeddedMigrations.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		data, err := embeddedMigrations.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}
