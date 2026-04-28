package testutil

import (
	"context"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"vitamins-backend_2/pkg/db"

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
		schemaErr = db.Apply(ctx, pool)
	})
	if schemaErr != nil {
		t.Fatalf("apply migrations: %v", schemaErr)
	}

	return pool
}

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
