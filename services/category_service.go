package services

import (
	"context"

	"github.com/google/uuid"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
)

// CategoryService defines the interface for category business logic
type CategoryService interface {
	// Create creates a new category
	Create(ctx context.Context, req *pb.CreateCategoryRequest, orgID uuid.UUID) (*pb.Category, error)

	// Get retrieves a category by ID
	Get(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*pb.GetCategoryResponse, error)

	// Update updates an existing category
	Update(ctx context.Context, id uuid.UUID, req *pb.UpdateCategoryRequest, orgID uuid.UUID) (*pb.Category, error)

	// Delete deletes a category
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID, force bool, reassignToCategoryID *uuid.UUID) (*pb.DeleteCategoryResponse, error)

	// List retrieves categories with filtering and pagination
	List(ctx context.Context, req *pb.ListCategoriesRequest, orgID uuid.UUID) (*pb.ListCategoriesResponse, error)

	// GetTree retrieves the full category hierarchy
	GetTree(ctx context.Context, req *pb.GetCategoryTreeRequest, orgID uuid.UUID) (*pb.GetCategoryTreeResponse, error)

	// GetPath retrieves the full path to a category
	GetPath(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*pb.GetCategoryPathResponse, error)
}
