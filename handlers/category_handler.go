package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/middleware"
	"github.com/yourorg/pim-demo/services"
)

// CategoryHandler handles HTTP requests for category operations
type CategoryHandler struct {
	categoryService services.CategoryService
}

// NewCategoryHandler creates a new category handler
func NewCategoryHandler(categoryService services.CategoryService) *CategoryHandler {
	return &CategoryHandler{
		categoryService: categoryService,
	}
}

// Create handles POST /api/v1/categories
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "CategoryHandler.Create")
	defer span.Finish()
	ext.HTTPMethod.Set(span, r.Method)
	ext.HTTPUrl.Set(span, r.URL.String())

	// Get organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok || orgID == uuid.Nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, services.ErrUnauthorized, generateRequestID())
		return
	}

	// Parse request body
	var req pb.CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	// Call service
	category, err := h.categoryService.Create(ctx, &req, orgID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Return response
	response := &pb.CreateCategoryResponse{
		Category: category,
	}
	WriteJSON(w, http.StatusCreated, response)
}

// Get handles GET /api/v1/categories/{id}
func (h *CategoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "CategoryHandler.Get")
	defer span.Finish()
	ext.HTTPMethod.Set(span, r.Method)
	ext.HTTPUrl.Set(span, r.URL.String())

	// Get organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok || orgID == uuid.Nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, services.ErrUnauthorized, generateRequestID())
		return
	}

	// Extract category ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	categoryID, err := uuid.Parse(pathParts[4])
	if err != nil {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	// Call service
	response, err := h.categoryService.Get(ctx, categoryID, orgID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Return response
	WriteJSON(w, http.StatusOK, response)
}

// Update handles PUT /api/v1/categories/{id}
func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "CategoryHandler.Update")
	defer span.Finish()
	ext.HTTPMethod.Set(span, r.Method)
	ext.HTTPUrl.Set(span, r.URL.String())

	// Get organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok || orgID == uuid.Nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, services.ErrUnauthorized, generateRequestID())
		return
	}

	// Extract category ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	categoryID, err := uuid.Parse(pathParts[4])
	if err != nil {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	// Parse request body
	var req pb.UpdateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	req.Id = categoryID.String()

	// Call service
	category, err := h.categoryService.Update(ctx, categoryID, &req, orgID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Return response
	response := &pb.UpdateCategoryResponse{
		Category: category,
	}
	WriteJSON(w, http.StatusOK, response)
}

// Delete handles DELETE /api/v1/categories/{id}
func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "CategoryHandler.Delete")
	defer span.Finish()
	ext.HTTPMethod.Set(span, r.Method)
	ext.HTTPUrl.Set(span, r.URL.String())

	// Get organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok || orgID == uuid.Nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, services.ErrUnauthorized, generateRequestID())
		return
	}

	// Extract category ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	categoryID, err := uuid.Parse(pathParts[4])
	if err != nil {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	// Parse optional query parameters
	force := r.URL.Query().Get("force") == "true"

	// Call service
	response, err := h.categoryService.Delete(ctx, categoryID, orgID, force, nil)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Return response
	WriteJSON(w, http.StatusOK, response)
}

// List handles GET /api/v1/categories
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "CategoryHandler.List")
	defer span.Finish()
	ext.HTTPMethod.Set(span, r.Method)
	ext.HTTPUrl.Set(span, r.URL.String())

	// Get organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok || orgID == uuid.Nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, services.ErrUnauthorized, generateRequestID())
		return
	}

	// For MVP, use basic pagination
	req := &pb.ListCategoriesRequest{
		Pagination: &pb.PaginationRequest{
			Page:     1,
			PageSize: 50,
		},
	}

	// Call service
	response, err := h.categoryService.List(ctx, req, orgID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Return response
	WriteJSON(w, http.StatusOK, response)
}

// GetTree handles GET /api/v1/categories/tree
func (h *CategoryHandler) GetTree(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "CategoryHandler.GetTree")
	defer span.Finish()
	ext.HTTPMethod.Set(span, r.Method)
	ext.HTTPUrl.Set(span, r.URL.String())

	// Get organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok || orgID == uuid.Nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, services.ErrUnauthorized, generateRequestID())
		return
	}

	// Parse query parameters
	req := &pb.GetCategoryTreeRequest{
		MaxDepth:   10,
		ActiveOnly: false,
	}

	if rootID := r.URL.Query().Get("root_id"); rootID != "" {
		req.RootId = rootID
	}

	// Call service
	response, err := h.categoryService.GetTree(ctx, req, orgID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Return response
	WriteJSON(w, http.StatusOK, response)
}

