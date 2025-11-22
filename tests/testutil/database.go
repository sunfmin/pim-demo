package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/yourorg/pim-demo/services"
	"gorm.io/driver/postgresDriver"
	"gorm.io/gorm"
)

// SetupTestDB starts a PostgreSQL container and returns a configured GORM DB instance
func SetupTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	ctx := context.Background()

	// Start PostgreSQL container
	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("test_pim"),
		postgres.WithUsername("test_user"),
		postgres.WithPassword("test_password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start PostgreSQL container: %v", err)
	}

	// Get connection string
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	// Open GORM connection
	db, err := gorm.Open(postgresDriver.Open(connStr), &gorm.Config{
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	// Run migrations
	if err := services.AutoMigrate(ctx, db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return db, cleanup
}

// TruncateTables truncates all tables in reverse dependency order for test isolation
func TruncateTables(db *gorm.DB, tables ...string) error {
	ctx := context.Background()

	// If no specific tables provided, truncate all in reverse order
	if len(tables) == 0 {
		tables = []string{
			"assets",
			"product_variants",
			"product_categories",
			"categories",
			"products",
			"organizations",
		}
	}

	// Disable foreign key checks temporarily (PostgreSQL)
	if err := db.WithContext(ctx).Exec("SET session_replication_role = 'replica'").Error; err != nil {
		return fmt.Errorf("failed to disable foreign key checks: %w", err)
	}

	// Truncate each table with CASCADE
	for _, table := range tables {
		if err := db.WithContext(ctx).Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)).Error; err != nil {
			// Re-enable foreign key checks before returning error
			db.WithContext(ctx).Exec("SET session_replication_role = 'origin'")
			return fmt.Errorf("failed to truncate table %s: %w", table, err)
		}
	}

	// Re-enable foreign key checks
	if err := db.WithContext(ctx).Exec("SET session_replication_role = 'origin'").Error; err != nil {
		return fmt.Errorf("failed to re-enable foreign key checks: %w", err)
	}

	return nil
}

