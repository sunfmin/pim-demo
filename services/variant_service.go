package services

import (
	"context"

	"github.com/google/uuid"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/models"
)

// VariantService defines the interface for product variant operations
type VariantService interface {
	// Create creates a new product variant
	Create(ctx context.Context, orgID uuid.UUID, req *pb.CreateVariantRequest) (*models.ProductVariant, error)

	// Get retrieves a variant by ID
	Get(ctx context.Context, orgID, variantID uuid.UUID) (*models.ProductVariant, error)

	// Update updates an existing variant
	Update(ctx context.Context, orgID uuid.UUID, req *pb.UpdateVariantRequest) (*models.ProductVariant, error)

	// Delete soft-deletes a variant
	Delete(ctx context.Context, orgID, variantID uuid.UUID) error

	// List retrieves all variants with optional filters
	List(ctx context.Context, orgID uuid.UUID, filters *pb.ListVariantsRequest) ([]*models.ProductVariant, int64, error)

	// ListByProduct retrieves all variants for a specific product
	ListByProduct(ctx context.Context, orgID, productID uuid.UUID) ([]*models.ProductVariant, error)

	// Generate generates variants from attribute combinations
	Generate(ctx context.Context, orgID uuid.UUID, req *pb.GenerateVariantsRequest) ([]*models.ProductVariant, error)
}

