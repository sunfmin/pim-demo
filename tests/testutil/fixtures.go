package testutil

import (
	"testing"

	"github.com/google/uuid"
	"github.com/yourorg/pim-demo/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CreateTestOrganization creates a test organization in the database
func CreateTestOrganization(t *testing.T, db *gorm.DB, overrides map[string]interface{}) *models.Organization {
	t.Helper()

	org := &models.Organization{
		Name:     "Test Organization",
		Slug:     "test-org-" + uuid.New().String()[:8],
		Settings: datatypes.JSON([]byte("{}")),
	}

	// Apply overrides
	if name, ok := overrides["name"].(string); ok {
		org.Name = name
	}
	if slug, ok := overrides["slug"].(string); ok {
		org.Slug = slug
	}
	if settings, ok := overrides["settings"].(datatypes.JSON); ok {
		org.Settings = settings
	}

	if err := db.Create(org).Error; err != nil {
		t.Fatalf("failed to create test organization: %v", err)
	}

	return org
}

// CreateTestProduct creates a test product in the database
func CreateTestProduct(t *testing.T, db *gorm.DB, orgID uuid.UUID, overrides map[string]interface{}) *models.Product {
	t.Helper()

	product := &models.Product{
		OrganizationID: orgID,
		SKU:            "TEST-SKU-" + uuid.New().String()[:8],
		Name:           "Test Product",
		Description:    "Test product description",
		BasePrice:      1999, // $19.99
		Status:         models.ProductStatusDraft,
		Attributes:     datatypes.JSON([]byte("{}")),
	}

	// Apply overrides
	if sku, ok := overrides["sku"].(string); ok {
		product.SKU = sku
	}
	if name, ok := overrides["name"].(string); ok {
		product.Name = name
	}
	if description, ok := overrides["description"].(string); ok {
		product.Description = description
	}
	if basePrice, ok := overrides["base_price"].(int64); ok {
		product.BasePrice = basePrice
	}
	if status, ok := overrides["status"].(models.ProductStatus); ok {
		product.Status = status
	}
	if attributes, ok := overrides["attributes"].(datatypes.JSON); ok {
		product.Attributes = attributes
	}

	if err := db.Create(product).Error; err != nil {
		t.Fatalf("failed to create test product: %v", err)
	}

	return product
}

// CreateTestProductBatch creates multiple test products for performance testing
func CreateTestProductBatch(t *testing.T, db *gorm.DB, orgID uuid.UUID, count int) []*models.Product {
	t.Helper()

	products := make([]*models.Product, count)
	for i := 0; i < count; i++ {
		products[i] = CreateTestProduct(t, db, orgID, map[string]interface{}{
			"sku":  uuid.New().String(),
			"name": "Bulk Product " + uuid.New().String()[:8],
		})
	}

	return products
}

