package services

import (
	"context"

	"github.com/google/uuid"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/models"
)

// SearchService defines the interface for product search operations
type SearchService interface {
	// Search performs full-text search with filters
	Search(ctx context.Context, orgID uuid.UUID, req *pb.SearchProductsRequest) ([]*models.Product, int64, error)

	// Filter applies filters without full-text search
	Filter(ctx context.Context, orgID uuid.UUID, filter *pb.ProductsFilter, pagination *pb.PaginationRequest) ([]*models.Product, int64, error)
}

