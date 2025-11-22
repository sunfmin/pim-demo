package services

import (
	"context"
	"fmt"

	"github.com/yourorg/pim-demo/internal/models"
	"gorm.io/gorm"
)

// AutoMigrate runs database migrations for all models
func AutoMigrate(ctx context.Context, db *gorm.DB) error {
	// Enable UUID extension if not already enabled
	if err := db.WithContext(ctx).Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error; err != nil {
		return fmt.Errorf("failed to create uuid-ossp extension: %w", err)
	}

	// Enable pg_trgm extension for full-text search
	if err := db.WithContext(ctx).Exec("CREATE EXTENSION IF NOT EXISTS \"pg_trgm\"").Error; err != nil {
		return fmt.Errorf("failed to create pg_trgm extension: %w", err)
	}

	// Run auto-migration for all models
	// Models will be added as they are created in subsequent phases
	if err := db.WithContext(ctx).AutoMigrate(
		&models.Organization{},
		// Additional models will be added here as they are created
		// &models.Product{},
		// &models.Category{},
		// &models.ProductCategory{},
		// &models.ProductVariant{},
		// &models.Asset{},
		// &models.AttributeDefinition{},
	); err != nil {
		return fmt.Errorf("failed to auto-migrate database: %w", err)
	}

	return nil
}

