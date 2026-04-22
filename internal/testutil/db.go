package testutil

import (
	"context"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"vitamins-backend_2/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	schemaOnce sync.Once
	schemaErr  error
)

// NewTestPool opens a pool to TEST_DATABASE_URL after verifying the DSN is safe
// (db name must contain "test"), applies embedded migrations exactly once per
// process, and registers Close on test cleanup. Tests must call ResetTables
// between runs to start from a known empty state.
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
		schemaErr = db.Apply(ctx, pool)
	})
	if schemaErr != nil {
		t.Fatalf("apply migrations: %v", schemaErr)
	}

	return pool
}

// ResetTables truncates all application tables. Before issuing the destructive
// statement it re-checks two safety properties at runtime, in addition to the
// DSN-based whitelist enforced by NewTestPool:
//   - the connected database name still contains "test"
//   - the host (from inet_server_addr / current host environment) is not a
//     production-looking address.
//
// This makes it much harder to accidentally point integration tests at a real
// database via a misconfigured pool.
func ResetTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var dbName string
	if err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName); err != nil {
		t.Fatalf("verify current_database: %v", err)
	}
	if !strings.Contains(strings.ToLower(dbName), "test") {
		t.Fatalf("refusing to ResetTables on database %q (must contain 'test')", dbName)
	}

	var serverAddr *string
	if err := pool.QueryRow(ctx, "SELECT host(inet_server_addr())::text").Scan(&serverAddr); err != nil {
		// Unix sockets / shared servers may not expose inet_server_addr; that's
		// acceptable — we already checked the DSN and current_database.
		serverAddr = nil
	}
	if serverAddr != nil {
		host := strings.ToLower(strings.TrimSpace(*serverAddr))
		if isProdLikeHost(host) {
			t.Fatalf("refusing to ResetTables on prod-looking host %q", host)
		}
	}

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
	if _, err := pool.Exec(ctx, truncateSQL); err != nil {
		t.Fatalf("truncate test db: %v", err)
	}
}

func isProdLikeHost(host string) bool {
	if host == "" {
		return false
	}
	if host == "127.0.0.1" || host == "::1" || host == "localhost" {
		return false
	}
	// Treat private network ranges and docker-compose names containing "test"
	// as safe; everything else is rejected.
	if strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "172.") {
		return false
	}
	if strings.Contains(host, "test") {
		return false
	}
	return true
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
