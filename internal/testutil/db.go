package testutil

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	schemaOnce sync.Once
	schemaErr  error
)

func NewTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := requireSafeTestDBURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)

	schemaOnce.Do(func() {
		schemaErr = applySchema(ctx, pool)
	})
	if schemaErr != nil {
		t.Fatalf("apply schema: %v", schemaErr)
	}

	return pool
}

func ResetTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	const truncateSQL = `
TRUNCATE TABLE
	analytics_events,
	notification_text_overrides,
	notification_preferences,
	intake_times,
	intake_schedules,
	vitamin_courses,
	user_vitamins,
	vitamin_catalog,
	users
RESTART IDENTITY CASCADE;
`
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, truncateSQL); err != nil {
		t.Fatalf("truncate test db: %v", err)
	}
}

func applySchema(ctx context.Context, pool *pgxpool.Pool) error {
	root := repoRoot()
	schemaPath := filepath.Join(root, "internal", "db", "models.sql")
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, string(content))
	return err
}

func requireSafeTestDBURL(t *testing.T) string {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Fatalf("TEST_DATABASE_URL is required for integration/e2e tests")
	}

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("invalid TEST_DATABASE_URL: %v", err)
	}
	dbName := strings.TrimPrefix(u.Path, "/")
	if !strings.Contains(strings.ToLower(dbName), "test") {
		t.Fatalf("unsafe TEST_DATABASE_URL db name %q: must contain 'test'", dbName)
	}
	return dsn
}

func repoRoot() string {
	_, current, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
}
