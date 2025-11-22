package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/models"
	"github.com/yourorg/pim-demo/services"
)

// SearchHandler handles HTTP requests for product search
type SearchHandler struct {
	searchService services.SearchService
}

// NewSearchHandler creates a new SearchHandler
func NewSearchHandler(searchService services.SearchService) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
	}
}

// Search handles POST /api/v1/products/search
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "POST /api/v1/products/search")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := ctx.Value("organization_id").(uuid.UUID)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Parse request body
	var req pb.SearchProductsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: %v", services.ErrInvalidProduct, err), generateRequestID())
		return
	}

	// Set default pagination if not provided
	if req.Pagination == nil {
		req.Pagination = &pb.PaginationRequest{
			Page:     1,
			PageSize: 20,
		}
	}

	// Perform search
	products, total, err := h.searchService.Search(ctx, orgID, &req)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Convert to protobuf search results
	results := make([]*pb.ProductSearchResult, len(products))
	for i, p := range products {
		results[i] = &pb.ProductSearchResult{
			Product:        modelToProtoProduct(p),
			RelevanceScore: 0.0, // TODO: Calculate actual relevance score
			MatchSnippet:   "",  // TODO: Extract match snippet
		}
	}

	totalPages := int32((total + int64(req.Pagination.PageSize) - 1) / int64(req.Pagination.PageSize))
	resp := &pb.SearchProductsResponse{
		Results: results,
		Pagination: &pb.PaginationResponse{
			Page:       req.Pagination.Page,
			PageSize:   req.Pagination.PageSize,
			TotalPages: totalPages,
		},
	}

	WriteJSON(w, http.StatusOK, resp)
}

// modelToProtoProduct converts a GORM model to protobuf product
func modelToProtoProduct(product *models.Product) *pb.Product {
	// Map status string to enum
	statusEnum := pb.ProductStatus_PRODUCT_STATUS_DRAFT
	switch product.Status {
	case models.ProductStatusDraft:
		statusEnum = pb.ProductStatus_PRODUCT_STATUS_DRAFT
	case models.ProductStatusActive:
		statusEnum = pb.ProductStatus_PRODUCT_STATUS_ACTIVE
	case models.ProductStatusDiscontinued:
		statusEnum = pb.ProductStatus_PRODUCT_STATUS_DISCONTINUED
	}

	// Parse attributes JSON
	var attributes map[string]interface{}
	json.Unmarshal(product.Attributes, &attributes)

	pbAttrs := make(map[string]*pb.AttributeValue)
	for key, value := range attributes {
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

	// Convert categories
	pbCategories := make([]string, len(product.Categories))
	for i, cat := range product.Categories {
		pbCategories[i] = cat.ID.String()
	}

	return &pb.Product{
		Id:             product.ID.String(),
		OrganizationId: product.OrganizationID.String(),
		Sku:            product.SKU,
		Name:           product.Name,
		Description:    product.Description,
		BasePrice:      product.BasePrice,
		Status:         statusEnum,
		Attributes:     pbAttrs,
		CategoryIds:    pbCategories,
		CreatedAt:      timestamppb.New(product.CreatedAt),
		UpdatedAt:      timestamppb.New(product.UpdatedAt),
	}
}

