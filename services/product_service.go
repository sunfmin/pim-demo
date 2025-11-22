package services

import (
	"context"

	"github.com/google/uuid"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
)

// ProductService defines the interface for product business logic
type ProductService interface {
	// Create creates a new product
	Create(ctx context.Context, req *pb.CreateProductRequest, orgID uuid.UUID) (*pb.Product, error)

	// Get retrieves a product by ID
	Get(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*pb.Product, error)

	// Update updates an existing product
	Update(ctx context.Context, id uuid.UUID, req *pb.UpdateProductRequest, orgID uuid.UUID) (*pb.Product, error)

	// Delete soft-deletes a product
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error

	// List retrieves products with filtering and pagination
	List(ctx context.Context, req *pb.ListProductsRequest, orgID uuid.UUID) (*pb.ListProductsResponse, error)
}
