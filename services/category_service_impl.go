package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/models"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

const maxCategoryDepth = 10

// categoryServiceImpl implements CategoryService interface
type categoryServiceImpl struct {
	db *gorm.DB
}

// NewCategoryService creates a new category service instance
func NewCategoryService(db *gorm.DB) CategoryService {
	return &categoryServiceImpl{db: db}
}

// Create creates a new category
func (s *categoryServiceImpl) Create(ctx context.Context, req *pb.CreateCategoryRequest, orgID uuid.UUID) (*pb.Category, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "CategoryService.Create")
	defer span.Finish()

	// Validate required fields
	if req.Name == "" {
		return nil, fmt.Errorf("category name is required: %w", ErrInvalidProduct)
	}

	// Generate slug if not provided
	slug := req.Slug
	if slug == "" {
		slug = generateSlug(req.Name)
	}

	// Validate parent if provided
	var parentID *uuid.UUID
	if req.ParentId != "" {
		pid, err := uuid.Parse(req.ParentId)
		if err != nil {
			return nil, fmt.Errorf("invalid parent ID: %w", ErrInvalidProduct)
		}

		// Verify parent exists and belongs to same organization
		var parent models.Category
		if err := s.db.WithContext(ctx).Where("id = ? AND organization_id = ?", pid, orgID).First(&parent).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("parent category not found: %w", ErrCategoryNotFound)
			}
			return nil, fmt.Errorf("failed to verify parent category: %w", err)
		}

		// Check depth limit
		depth, err := s.getCategoryDepth(ctx, pid)
		if err != nil {
			return nil, fmt.Errorf("failed to check category depth: %w", err)
		}
		if depth >= maxCategoryDepth {
			return nil, fmt.Errorf("maximum category depth (%d) exceeded: %w", maxCategoryDepth, ErrInvalidProduct)
		}

		parentID = &pid
	}

	// Check for duplicate slug within the same organization
	var existingCategory models.Category
	if err := s.db.WithContext(ctx).
		Where("organization_id = ? AND slug = ?", orgID, slug).
		First(&existingCategory).Error; err == nil {
		// Category with this slug already exists
		return nil, fmt.Errorf("category with slug '%s' already exists: %w", slug, ErrDuplicateCategorySlug)
	} else if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check for duplicate category slug: %w", err)
	}

	// Create category
	category := &models.Category{
		OrganizationID: orgID,
		ParentID:       parentID,
		Name:           req.Name,
		Slug:           slug,
		Description:    req.Description,
		DisplayOrder:   int(req.DisplayOrder),
		IsActive:       req.IsActive,
	}

	if err := s.db.WithContext(ctx).Create(category).Error; err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	return convertCategoryModelToProto(category), nil
}

// Get retrieves a category by ID
func (s *categoryServiceImpl) Get(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*pb.GetCategoryResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "CategoryService.Get")
	defer span.Finish()

	var category models.Category
	err := s.db.WithContext(ctx).
		Where("id = ? AND organization_id = ?", id, orgID).
		Preload("Children").
		First(&category).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("category with ID '%s' not found: %w", id, ErrCategoryNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("failed to retrieve category: %w", err)
	}

	// Get ancestors
	ancestors, err := s.getAncestors(ctx, &category)
	if err != nil {
		return nil, fmt.Errorf("failed to get ancestors: %w", err)
	}

	// Convert to protobuf
	pbCategory := convertCategoryModelToProto(&category)
	pbAncestors := make([]*pb.Category, len(ancestors))
	for i, ancestor := range ancestors {
		pbAncestors[i] = convertCategoryModelToProto(&ancestor)
	}
	pbChildren := make([]*pb.Category, len(category.Children))
	for i, child := range category.Children {
		pbChildren[i] = convertCategoryModelToProto(&child)
	}

	return &pb.GetCategoryResponse{
		Category:  pbCategory,
		Ancestors: pbAncestors,
		Children:  pbChildren,
	}, nil
}

// Update updates an existing category
func (s *categoryServiceImpl) Update(ctx context.Context, id uuid.UUID, req *pb.UpdateCategoryRequest, orgID uuid.UUID) (*pb.Category, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "CategoryService.Update")
	defer span.Finish()

	// Retrieve existing category
	var category models.Category
	err := s.db.WithContext(ctx).
		Where("id = ? AND organization_id = ?", id, orgID).
		First(&category).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("category with ID '%s' not found: %w", id, ErrCategoryNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("failed to retrieve category: %w", err)
	}

	// Update fields if provided
	if req.Name != "" {
		category.Name = req.Name
	}
	if req.Slug != "" {
		category.Slug = req.Slug
	}
	if req.Description != "" {
		category.Description = req.Description
	}
	if req.DisplayOrder >= 0 {
		category.DisplayOrder = int(req.DisplayOrder)
	}
	if req.IsActive {
		category.IsActive = req.IsActive
	}

	// Handle parent change (move category)
	if req.ParentId != "" {
		newParentID, err := uuid.Parse(req.ParentId)
		if err != nil {
			return nil, fmt.Errorf("invalid parent ID: %w", ErrInvalidProduct)
		}

		// Check for circular reference
		if err := s.checkCircularReference(ctx, id, newParentID); err != nil {
			return nil, err
		}

		category.ParentID = &newParentID
	}

	// Save updates
	if err := s.db.WithContext(ctx).Save(&category).Error; err != nil {
		return nil, fmt.Errorf("failed to update category: %w", err)
	}

	return convertCategoryModelToProto(&category), nil
}

// Delete deletes a category
func (s *categoryServiceImpl) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID, force bool, reassignToCategoryID *uuid.UUID) (*pb.DeleteCategoryResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "CategoryService.Delete")
	defer span.Finish()

	// Check if category exists
	var category models.Category
	err := s.db.WithContext(ctx).
		Where("id = ? AND organization_id = ?", id, orgID).
		Preload("Children").
		Preload("Products").
		First(&category).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("category with ID '%s' not found: %w", id, ErrCategoryNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("failed to retrieve category: %w", err)
	}

	// Check for children
	if len(category.Children) > 0 && !force {
		return nil, fmt.Errorf("category has %d sub-categories: %w", len(category.Children), ErrProductHasDependencies)
	}

	// Check for products
	productsReassigned := 0
	if len(category.Products) > 0 && !force && reassignToCategoryID == nil {
		return nil, fmt.Errorf("category has %d products: %w", len(category.Products), ErrProductHasDependencies)
	}

	// Handle reassignment or force delete
	if reassignToCategoryID != nil {
		// Reassign products to different category
		// This would be implemented when we add category assignment
		productsReassigned = len(category.Products)
	}

	// Delete category
	subcategoriesDeleted := 0
	if force && len(category.Children) > 0 {
		// Delete all children recursively
		subcategoriesDeleted = s.deleteChildrenRecursive(ctx, id)
	}

	if err := s.db.WithContext(ctx).Delete(&category).Error; err != nil {
		return nil, fmt.Errorf("failed to delete category: %w", err)
	}

	return &pb.DeleteCategoryResponse{
		Success:              true,
		ProductsReassigned:   int32(productsReassigned),
		SubcategoriesDeleted: int32(subcategoriesDeleted),
	}, nil
}

// List retrieves categories with filtering and pagination
func (s *categoryServiceImpl) List(ctx context.Context, req *pb.ListCategoriesRequest, orgID uuid.UUID) (*pb.ListCategoriesResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "CategoryService.List")
	defer span.Finish()

	query := s.db.WithContext(ctx).Where("organization_id = ?", orgID)

	// Apply filters if provided
	if req.Filter != nil {
		if req.Filter.ParentId != "" {
			parentID, err := uuid.Parse(req.Filter.ParentId)
			if err == nil {
				query = query.Where("parent_id = ?", parentID)
			}
		} else if req.Filter.ParentId == "" {
			// Empty string means root categories only
			// query = query.Where("parent_id IS NULL")
		}

		if req.Filter.IsActive {
			query = query.Where("is_active = ?", true)
		}
	}

	// Count total items
	var totalItems int64
	if err := query.Model(&models.Category{}).Count(&totalItems).Error; err != nil {
		return nil, fmt.Errorf("failed to count categories: %w", err)
	}

	// Apply pagination
	page := int32(1)
	pageSize := int32(20)
	if req.Pagination != nil {
		if req.Pagination.Page > 0 {
			page = req.Pagination.Page
		}
		if req.Pagination.PageSize > 0 {
			pageSize = req.Pagination.PageSize
		}
	}

	// Apply sorting
	orderBy := "display_order ASC, name ASC"
	if req.Sort != nil {
		switch req.Sort.Field {
		case pb.CategoriesSort_SORT_FIELD_NAME:
			orderBy = "name"
		case pb.CategoriesSort_SORT_FIELD_DISPLAY_ORDER:
			orderBy = "display_order"
		case pb.CategoriesSort_SORT_FIELD_CREATED_AT:
			orderBy = "created_at"
		}

		if req.Sort.Order == pb.SortOrder_SORT_ORDER_DESC {
			orderBy += " DESC"
		} else {
			orderBy += " ASC"
		}
	}

	// Retrieve categories
	var categories []models.Category
	offset := (page - 1) * pageSize
	if err := query.Order(orderBy).Limit(int(pageSize)).Offset(int(offset)).Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve categories: %w", err)
	}

	// Convert to protobuf
	pbCategories := make([]*pb.Category, len(categories))
	for i, category := range categories {
		pbCategories[i] = convertCategoryModelToProto(&category)
	}

	// Calculate pagination metadata
	totalPages := (totalItems + int64(pageSize) - 1) / int64(pageSize)

	return &pb.ListCategoriesResponse{
		Categories: pbCategories,
		Pagination: &pb.PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: int32(totalItems),
			TotalPages: int32(totalPages),
		},
	}, nil
}

// GetTree retrieves the full category hierarchy
func (s *categoryServiceImpl) GetTree(ctx context.Context, req *pb.GetCategoryTreeRequest, orgID uuid.UUID) (*pb.GetCategoryTreeResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "CategoryService.GetTree")
	defer span.Finish()

	// Get root categories or specific category
	var rootCategories []models.Category
	query := s.db.WithContext(ctx).Where("organization_id = ?", orgID)

	if req.RootId != "" {
		rootID, err := uuid.Parse(req.RootId)
		if err != nil {
			return nil, fmt.Errorf("invalid root ID: %w", ErrInvalidProduct)
		}
		query = query.Where("id = ?", rootID)
	} else {
		query = query.Where("parent_id IS NULL")
	}

	if req.ActiveOnly {
		query = query.Where("is_active = ?", true)
	}

	if err := query.Order("display_order ASC, name ASC").Find(&rootCategories).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve root categories: %w", err)
	}

	// Build trees recursively
	trees := make([]*pb.CategoryTree, len(rootCategories))
	maxDepth := int(req.MaxDepth)
	if maxDepth == 0 {
		maxDepth = maxCategoryDepth
	}

	for i, rootCat := range rootCategories {
		tree, err := s.buildCategoryTree(ctx, &rootCat, 0, maxDepth, req.ActiveOnly)
		if err != nil {
			return nil, err
		}
		trees[i] = tree
	}

	return &pb.GetCategoryTreeResponse{
		Trees: trees,
	}, nil
}

// GetPath retrieves the full path to a category
func (s *categoryServiceImpl) GetPath(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*pb.GetCategoryPathResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "CategoryService.GetPath")
	defer span.Finish()

	var category models.Category
	err := s.db.WithContext(ctx).
		Where("id = ? AND organization_id = ?", id, orgID).
		First(&category).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("category with ID '%s' not found: %w", id, ErrCategoryNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("failed to retrieve category: %w", err)
	}

	// Get ancestors
	ancestors, err := s.getAncestors(ctx, &category)
	if err != nil {
		return nil, fmt.Errorf("failed to get ancestors: %w", err)
	}

	// Build path (root to current)
	path := make([]*pb.Category, len(ancestors)+1)
	for i, ancestor := range ancestors {
		path[i] = convertCategoryModelToProto(&ancestor)
	}
	path[len(ancestors)] = convertCategoryModelToProto(&category)

	// Build path string
	pathNames := make([]string, len(path))
	for i, cat := range path {
		pathNames[i] = cat.Name
	}
	pathString := strings.Join(pathNames, " > ")

	return &pb.GetCategoryPathResponse{
		Path:       path,
		PathString: pathString,
	}, nil
}

// Helper functions

// getCategoryDepth calculates the depth of a category in the hierarchy
func (s *categoryServiceImpl) getCategoryDepth(ctx context.Context, categoryID uuid.UUID) (int, error) {
	depth := 0
	currentID := &categoryID

	for currentID != nil && depth < maxCategoryDepth {
		var category models.Category
		if err := s.db.WithContext(ctx).Where("id = ?", currentID).First(&category).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				break
			}
			return 0, err
		}

		depth++
		currentID = category.ParentID
	}

	return depth, nil
}

// checkCircularReference checks if setting newParentID as parent would create a circular reference
func (s *categoryServiceImpl) checkCircularReference(ctx context.Context, categoryID uuid.UUID, newParentID uuid.UUID) error {
	// A category cannot be its own ancestor
	if categoryID == newParentID {
		return fmt.Errorf("category cannot be its own parent: %w", ErrInvalidProduct)
	}

	// Check if newParentID is a descendant of categoryID
	currentID := &newParentID
	visited := make(map[uuid.UUID]bool)

	for currentID != nil {
		if *currentID == categoryID {
			return fmt.Errorf("circular reference detected: new parent is a descendant of this category: %w", ErrInvalidProduct)
		}

		// Prevent infinite loops
		if visited[*currentID] {
			return fmt.Errorf("circular reference detected in category hierarchy: %w", ErrInvalidProduct)
		}
		visited[*currentID] = true

		var parent models.Category
		if err := s.db.WithContext(ctx).Where("id = ?", currentID).First(&parent).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				break
			}
			return fmt.Errorf("failed to check parent: %w", err)
		}

		currentID = parent.ParentID
	}

	return nil
}

// getAncestors retrieves all ancestor categories from root to immediate parent
func (s *categoryServiceImpl) getAncestors(ctx context.Context, category *models.Category) ([]models.Category, error) {
	ancestors := []models.Category{}
	currentID := category.ParentID

	for currentID != nil {
		var parent models.Category
		if err := s.db.WithContext(ctx).Where("id = ?", currentID).First(&parent).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				break
			}
			return nil, err
		}

		// Prepend to get root-to-parent order
		ancestors = append([]models.Category{parent}, ancestors...)
		currentID = parent.ParentID
	}

	return ancestors, nil
}

// buildCategoryTree recursively builds a category tree
func (s *categoryServiceImpl) buildCategoryTree(ctx context.Context, category *models.Category, currentDepth int, maxDepth int, activeOnly bool) (*pb.CategoryTree, error) {
	tree := &pb.CategoryTree{
		Category: convertCategoryModelToProto(category),
		Depth:    int32(currentDepth),
	}

	// Stop if max depth reached
	if currentDepth >= maxDepth {
		return tree, nil
	}

	// Get children
	var children []models.Category
	query := s.db.WithContext(ctx).
		Where("parent_id = ? AND organization_id = ?", category.ID, category.OrganizationID).
		Order("display_order ASC, name ASC")

	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	if err := query.Find(&children).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve children: %w", err)
	}

	// Build child trees
	tree.Children = make([]*pb.CategoryTree, len(children))
	for i, child := range children {
		childTree, err := s.buildCategoryTree(ctx, &child, currentDepth+1, maxDepth, activeOnly)
		if err != nil {
			return nil, err
		}
		tree.Children[i] = childTree
	}

	return tree, nil
}

// deleteChildrenRecursive deletes all children recursively
func (s *categoryServiceImpl) deleteChildrenRecursive(ctx context.Context, parentID uuid.UUID) int {
	var children []models.Category
	s.db.WithContext(ctx).Where("parent_id = ?", parentID).Find(&children)

	count := 0
	for _, child := range children {
		count += s.deleteChildrenRecursive(ctx, child.ID)
		s.db.WithContext(ctx).Delete(&child)
		count++
	}

	return count
}

// generateSlug generates a URL-friendly slug from a name
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove special characters (simplified version)
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, slug)
	return slug
}

// convertCategoryModelToProto converts a Category model to protobuf
func convertCategoryModelToProto(category *models.Category) *pb.Category {
	parentID := ""
	if category.ParentID != nil {
		parentID = category.ParentID.String()
	}

	return &pb.Category{
		Id:             category.ID.String(),
		OrganizationId: category.OrganizationID.String(),
		ParentId:       parentID,
		Name:           category.Name,
		Slug:           category.Slug,
		Description:    category.Description,
		DisplayOrder:   int32(category.DisplayOrder),
		IsActive:       category.IsActive,
		ProductCount:   0, // Will be populated when needed
		CreatedAt:      timestamppb.New(category.CreatedAt),
		UpdatedAt:      timestamppb.New(category.UpdatedAt),
	}
}
