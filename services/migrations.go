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
		&models.ProductVariant{},
		&models.Asset{},
		// Additional models will be added here as they are created
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

	// Create unique index on (organization_id, variant_sku) for product_variants
	if err := db.WithContext(ctx).Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_product_variants_org_sku 
		ON product_variants (organization_id, variant_sku)
	`).Error; err != nil {
		return fmt.Errorf("failed to create unique index on product_variants: %w", err)
	}

	// Create full-text search indexes
	// GIN index on search_vector for full-text search
	if err := db.WithContext(ctx).Exec(`
		CREATE INDEX IF NOT EXISTS idx_products_search_vector 
		ON products USING GIN(search_vector)
	`).Error; err != nil {
		return fmt.Errorf("failed to create search vector index: %w", err)
	}

	// Trigram indexes for partial matching on SKU and name
	if err := db.WithContext(ctx).Exec(`
		CREATE INDEX IF NOT EXISTS idx_products_sku_trgm 
		ON products USING GIN(sku gin_trgm_ops)
	`).Error; err != nil {
		return fmt.Errorf("failed to create SKU trigram index: %w", err)
	}

	if err := db.WithContext(ctx).Exec(`
		CREATE INDEX IF NOT EXISTS idx_products_name_trgm 
		ON products USING GIN(name gin_trgm_ops)
	`).Error; err != nil {
		return fmt.Errorf("failed to create name trigram index: %w", err)
	}

	// Create trigger to maintain search_vector
	if err := db.WithContext(ctx).Exec(`
		CREATE OR REPLACE FUNCTION products_search_vector_update() 
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.search_vector := 
				setweight(to_tsvector('english', COALESCE(NEW.name, '')), 'A') ||
				setweight(to_tsvector('english', COALESCE(NEW.sku, '')), 'B') ||
				setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'C');
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql
	`).Error; err != nil {
		return fmt.Errorf("failed to create search vector function: %w", err)
	}

	if err := db.WithContext(ctx).Exec(`
		DROP TRIGGER IF EXISTS products_search_vector_trigger ON products
	`).Error; err != nil {
		return fmt.Errorf("failed to drop old search vector trigger: %w", err)
	}

	if err := db.WithContext(ctx).Exec(`
		CREATE TRIGGER products_search_vector_trigger 
		BEFORE INSERT OR UPDATE ON products
		FOR EACH ROW EXECUTE FUNCTION products_search_vector_update()
	`).Error; err != nil {
		return fmt.Errorf("failed to create search vector trigger: %w", err)
	}

	// Update existing products to populate search_vector
	if err := db.WithContext(ctx).Exec(`
		UPDATE products SET search_vector = 
			setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
			setweight(to_tsvector('english', COALESCE(sku, '')), 'B') ||
			setweight(to_tsvector('english', COALESCE(description, '')), 'C')
		WHERE search_vector IS NULL OR search_vector = ''
	`).Error; err != nil {
		return fmt.Errorf("failed to update existing search vectors: %w", err)
	}

	return nil
}

