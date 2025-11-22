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
		&models.Category{},
		&models.Product{},
		&models.ProductCategory{},
		// Additional models will be added here as they are created
		// &models.ProductVariant{},
		// &models.Asset{},
		// &models.AttributeDefinition{},
	); err != nil {
		return fmt.Errorf("failed to auto-migrate database: %w", err)
	}

	// Create unique index on (organization_id, sku) for products
	if err := db.WithContext(ctx).Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_products_org_sku 
		ON products (organization_id, sku) 
		WHERE deleted_at IS NULL
	`).Error; err != nil {
		return fmt.Errorf("failed to create unique index on products: %w", err)
	}

	// Create unique index on (organization_id, slug) for categories
	if err := db.WithContext(ctx).Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_org_slug 
		ON categories (organization_id, slug)
	`).Error; err != nil {
		return fmt.Errorf("failed to create unique index on categories: %w", err)
	}

	// Create unique index on (product_id, category_id) for product_categories
	if err := db.WithContext(ctx).Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_product_categories_unique 
		ON product_categories (product_id, category_id)
	`).Error; err != nil {
		return fmt.Errorf("failed to create unique index on product_categories: %w", err)
	}

	return nil
}

