package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/middleware"
	"github.com/yourorg/pim-demo/internal/models"
	"github.com/yourorg/pim-demo/services"
)

// VariantHandler handles HTTP requests for product variants
type VariantHandler struct {
	variantService services.VariantService
	productService services.ProductService
}

// NewVariantHandler creates a new VariantHandler
func NewVariantHandler(variantService services.VariantService, productService services.ProductService) *VariantHandler {
	return &VariantHandler{
		variantService: variantService,
		productService: productService,
	}
}

// Create handles POST /api/v1/variants
func (h *VariantHandler) Create(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "POST /api/v1/variants")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Parse request body
	var req pb.CreateVariantRequest
	if err := ReadJSON(r, &req); err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: %v", services.ErrInvalidProduct, err), generateRequestID())
		return
	}

	// Create variant
	variant, err := h.variantService.Create(ctx, orgID, &req)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Convert to protobuf response
	pbVariant := modelToProtoVariant(variant)

	resp := &pb.CreateVariantResponse{
		Variant: pbVariant,
	}

	WriteJSON(w, http.StatusCreated, resp)
}

// Get handles GET /api/v1/variants/{id}
func (h *VariantHandler) Get(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "GET /api/v1/variants/{id}")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Extract variant ID from URL
	variantIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/variants/")
	variantID, err := uuid.Parse(variantIDStr)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: invalid variant ID", services.ErrInvalidProduct), generateRequestID())
		return
	}

	// Fetch variant
	variant, err := h.variantService.Get(ctx, orgID, variantID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Convert to protobuf response
	pbVariant := modelToProtoVariant(variant)

	resp := &pb.GetVariantResponse{
		Variant: pbVariant,
	}

	WriteJSON(w, http.StatusOK, resp)
}

// Update handles PUT /api/v1/variants/{id}
func (h *VariantHandler) Update(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "PUT /api/v1/variants/{id}")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Extract variant ID from URL
	variantIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/variants/")
	variantID, err := uuid.Parse(variantIDStr)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: invalid variant ID", services.ErrInvalidProduct), generateRequestID())
		return
	}

	// Parse request body
	var req pb.UpdateVariantRequest
	if err := ReadJSON(r, &req); err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: %v", services.ErrInvalidProduct, err), generateRequestID())
		return
	}

	// Set variant ID from URL
	req.Id = variantID.String()

	// Update variant
	variant, err := h.variantService.Update(ctx, orgID, &req)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Convert to protobuf response
	pbVariant := modelToProtoVariant(variant)

	resp := &pb.UpdateVariantResponse{
		Variant: pbVariant,
	}

	WriteJSON(w, http.StatusOK, resp)
}

// Delete handles DELETE /api/v1/variants/{id}
func (h *VariantHandler) Delete(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "DELETE /api/v1/variants/{id}")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Extract variant ID from URL
	variantIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/variants/")
	variantID, err := uuid.Parse(variantIDStr)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: invalid variant ID", services.ErrInvalidProduct), generateRequestID())
		return
	}

	// Delete variant
	if err := h.variantService.Delete(ctx, orgID, variantID); err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	resp := &pb.DeleteVariantResponse{
		Success: true,
	}

	WriteJSON(w, http.StatusOK, resp)
}

// List handles GET /api/v1/variants
func (h *VariantHandler) List(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "GET /api/v1/variants")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Parse query parameters
	filters := &pb.ListVariantsRequest{
		ParentProductId: r.URL.Query().Get("parent_product_id"),
		Pagination: &pb.PaginationRequest{
			Page:     1,
			PageSize: 20,
		},
	}

	if isActive := r.URL.Query().Get("is_active"); isActive != "" {
		filters.Filter = &pb.VariantsFilter{
			IsActive: isActive == "true",
		}
	}

	// Fetch variants
	variants, total, err := h.variantService.List(ctx, orgID, filters)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Convert to protobuf response
	pbVariants := make([]*pb.ProductVariant, len(variants))
	for i, v := range variants {
		pbVariants[i] = modelToProtoVariant(v)
	}

	totalPages := int32((total + int64(filters.Pagination.PageSize) - 1) / int64(filters.Pagination.PageSize))
	resp := &pb.ListVariantsResponse{
		Variants: pbVariants,
		Pagination: &pb.PaginationResponse{
			Page:       filters.Pagination.Page,
			PageSize:   filters.Pagination.PageSize,
			TotalPages: totalPages,
		},
	}

	WriteJSON(w, http.StatusOK, resp)
}

// ListByProduct handles GET /api/v1/products/{id}/variants
func (h *VariantHandler) ListByProduct(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "GET /api/v1/products/{id}/variants")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Extract product ID from URL
	productIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/products/")
	productIDStr = strings.TrimSuffix(productIDStr, "/variants")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: invalid product ID", services.ErrInvalidProduct), generateRequestID())
		return
	}

	// Fetch variants
	variants, err := h.variantService.ListByProduct(ctx, orgID, productID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Convert to protobuf response
	pbVariants := make([]*pb.ProductVariant, len(variants))
	for i, v := range variants {
		pbVariants[i] = modelToProtoVariant(v)
	}

	resp := &pb.ListVariantsResponse{
		Variants: pbVariants,
	}

	WriteJSON(w, http.StatusOK, resp)
}

// Generate handles POST /api/v1/variants/generate
func (h *VariantHandler) Generate(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "POST /api/v1/variants/generate")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Parse request body
	var req pb.GenerateVariantsRequest
	if err := ReadJSON(r, &req); err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: %v", services.ErrInvalidProduct, err), generateRequestID())
		return
	}

	// Generate variants
	variants, err := h.variantService.Generate(ctx, orgID, &req)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Convert to protobuf response
	pbVariants := make([]*pb.ProductVariant, len(variants))
	for i, v := range variants {
		pbVariants[i] = modelToProtoVariant(v)
	}

	resp := &pb.GenerateVariantsResponse{
		Variants: pbVariants,
	}

	WriteJSON(w, http.StatusOK, resp)
}

// modelToProtoVariant converts a GORM model to protobuf variant
func modelToProtoVariant(variant *models.ProductVariant) *pb.ProductVariant {
	// Parse variant attributes JSON
	var variantAttrs map[string]interface{}
	json.Unmarshal(variant.VariantAttributes, &variantAttrs)

	pbAttrs := make(map[string]*pb.AttributeValue)
	for key, value := range variantAttrs {
		switch v := value.(type) {
		case string:
			pbAttrs[key] = &pb.AttributeValue{
				Value: &pb.AttributeValue_StringValue{StringValue: v},
			}
		case float64:
			pbAttrs[key] = &pb.AttributeValue{
				Value: &pb.AttributeValue_NumberValue{NumberValue: v},
			}
		}
	}

	return &pb.ProductVariant{
		Id:                variant.ID.String(),
		OrganizationId:    variant.OrganizationID.String(),
		ParentProductId:   variant.ParentProductID.String(),
		VariantSku:        variant.VariantSKU,
		VariantAttributes: pbAttrs,
		PriceAdjustment:   variant.PriceAdjustment,
		InventoryQuantity: variant.InventoryQuantity,
		IsActive:          variant.IsActive,
		CreatedAt:         timestamppb.New(variant.CreatedAt),
		UpdatedAt:         timestamppb.New(variant.UpdatedAt),
	}
}
