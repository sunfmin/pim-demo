package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/middleware"
	"github.com/yourorg/pim-demo/services"
)

// ProductHandler handles HTTP requests for product operations
type ProductHandler struct {
	productService services.ProductService
}

// NewProductHandler creates a new product handler
func NewProductHandler(productService services.ProductService) *ProductHandler {
	return &ProductHandler{
		productService: productService,
	}
}

// Create handles POST /api/v1/products
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Create child span for this handler
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "ProductHandler.Create")
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
	var req pb.CreateProductRequest
	if err := ReadJSON(r, &req); err != nil {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	// Call service
	product, err := h.productService.Create(ctx, &req, orgID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Return response
	response := &pb.CreateProductResponse{
		Product: product,
	}
	WriteJSON(w, http.StatusCreated, response)
}

// Get handles GET /api/v1/products/{id}
func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "ProductHandler.Get")
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

	// Extract product ID from URL path
	// URL format: /api/v1/products/{id}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	productID, err := uuid.Parse(pathParts[4])
	if err != nil {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	// Call service
	product, err := h.productService.Get(ctx, productID, orgID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Return response
	response := &pb.GetProductResponse{
		Product: product,
	}
	WriteJSON(w, http.StatusOK, response)
}

// Update handles PUT /api/v1/products/{id}
func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "ProductHandler.Update")
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

	// Extract product ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	productID, err := uuid.Parse(pathParts[4])
	if err != nil {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	// Parse request body
	var req pb.UpdateProductRequest
	if err := ReadJSON(r, &req); err != nil {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	// Set ID from URL
	req.Id = productID.String()

	// Call service
	product, err := h.productService.Update(ctx, productID, &req, orgID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Return response
	response := &pb.UpdateProductResponse{
		Product: product,
	}
	WriteJSON(w, http.StatusOK, response)
}

// Delete handles DELETE /api/v1/products/{id}
func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "ProductHandler.Delete")
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

	// Extract product ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	productID, err := uuid.Parse(pathParts[4])
	if err != nil {
		ext.Error.Set(span, true)
		WriteError(w, ErrCodeInvalidRequest, generateRequestID())
		return
	}

	// Call service
	err = h.productService.Delete(ctx, productID, orgID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Return response
	response := &pb.DeleteProductResponse{
		Success: true,
	}
	WriteJSON(w, http.StatusOK, response)
}

// List handles GET /api/v1/products
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "ProductHandler.List")
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
	// For MVP, we'll support basic pagination
	// Full filtering/sorting can be added later via JSON body or query params

	req := &pb.ListProductsRequest{
		Pagination: &pb.PaginationRequest{
			Page:     1,
			PageSize: 20,
		},
	}

	// Call service
	response, err := h.productService.List(ctx, req, orgID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Return response
	WriteJSON(w, http.StatusOK, response)
}

// generateRequestID generates a unique request ID for error tracking
func generateRequestID() string {
	return uuid.New().String()
}
