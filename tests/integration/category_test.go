package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/handlers"
	"github.com/yourorg/pim-demo/internal/middleware"
	"github.com/yourorg/pim-demo/internal/models"
	"github.com/yourorg/pim-demo/services"
	"github.com/yourorg/pim-demo/tests/testutil"
	"google.golang.org/protobuf/testing/protocmp"
)

// TestCategoryAcceptanceScenarios tests all acceptance scenarios for User Story 2
func TestCategoryAcceptanceScenarios(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	categoryService := services.NewCategoryService(db)
	categoryHandler := handlers.NewCategoryHandler(categoryService)
	productService := services.NewProductService(db)
	productHandler := handlers.NewProductHandler(productService)

	addOrgContext := func(r *http.Request, orgID uuid.UUID) *http.Request {
		ctx := context.WithValue(r.Context(), middleware.OrganizationIDKey, orgID)
		return r.WithContext(ctx)
	}

	testCases := []struct {
		name     string
		scenario string
		testFunc func(t *testing.T)
	}{
		{
			name:     "US2-AS1: Create category with optional parent",
			scenario: "Given I am logged in as a product manager, When I create a new category with a name and optional parent category, Then the category is created and appears in the category hierarchy",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)
				testOrg := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Create root category
				rootRequest := &pb.CreateCategoryRequest{
					Name:        "Electronics",
					Slug:        "electronics",
					Description: "Electronic devices",
					IsActive:    true,
				}

				body, _ := json.Marshal(rootRequest)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/categories", bytes.NewReader(body))
				req = addOrgContext(req, testOrg.ID)

				rec := httptest.NewRecorder()
				categoryHandler.Create(rec, req)

				if rec.Code != http.StatusCreated {
					t.Fatalf("Expected status %d, got %d. Body: %s", http.StatusCreated, rec.Code, rec.Body.String())
				}

				var response pb.CreateCategoryResponse
				json.NewDecoder(rec.Body).Decode(&response)

				// Build expected from fixtures
				expected := &pb.CreateCategoryResponse{
					Category: &pb.Category{
						Id:             response.Category.Id,              // Generated ID
						OrganizationId: testOrg.ID.String(),               // From test fixture
						ParentId:       "",                                // No parent (root)
						Name:           rootRequest.Name,                  // From request fixture
						Slug:           rootRequest.Slug,                  // From request fixture
						Description:    rootRequest.Description,           // From request fixture
						DisplayOrder:   rootRequest.DisplayOrder,          // From request fixture
						IsActive:       rootRequest.IsActive,              // From request fixture
						ProductCount:   0,                                 // No products yet
						CreatedAt:      response.Category.CreatedAt,       // Generated timestamp
						UpdatedAt:      response.Category.UpdatedAt,       // Generated timestamp
					},
				}

				if diff := cmp.Diff(expected, &response, protocmp.Transform()); diff != "" {
					t.Errorf("Response mismatch (-want +got):\n%s", diff)
				}

				// Create child category with parent
				childRequest := &pb.CreateCategoryRequest{
					ParentId:    response.Category.Id,
					Name:        "Computers",
					Slug:        "computers",
					Description: "Computer equipment",
					IsActive:    true,
				}

				body2, _ := json.Marshal(childRequest)
				req2 := httptest.NewRequest(http.MethodPost, "/api/v1/categories", bytes.NewReader(body2))
				req2 = addOrgContext(req2, testOrg.ID)

				rec2 := httptest.NewRecorder()
				categoryHandler.Create(rec2, req2)

				if rec2.Code != http.StatusCreated {
					t.Fatalf("Expected status %d, got %d", http.StatusCreated, rec2.Code)
				}

				var childResponse pb.CreateCategoryResponse
				json.NewDecoder(rec2.Body).Decode(&childResponse)

				// Verify parent ID is set
				if childResponse.Category.ParentId != response.Category.Id {
					t.Errorf("Expected parent_id %s, got %s", response.Category.Id, childResponse.Category.ParentId)
				}

				// Verify categories appear in list
				listReq := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
				listReq = addOrgContext(listReq, testOrg.ID)
				listRec := httptest.NewRecorder()
				categoryHandler.List(listRec, listReq)

				var listResponse pb.ListCategoriesResponse
				json.NewDecoder(listRec.Body).Decode(&listResponse)

				if len(listResponse.Categories) != 2 {
					t.Errorf("Expected 2 categories, got %d", len(listResponse.Categories))
				}
			},
		},
		{
			name:     "US2-AS2: Assign product to multiple categories",
			scenario: "Given a product and category exist, When I assign the product to one or more categories, Then the product appears in all assigned categories",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)
				testOrg := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Create categories
				cat1 := testutil.CreateTestCategory(t, db, testOrg.ID, map[string]interface{}{
					"name": "Category 1",
					"slug": "category-1",
				})
				cat2 := testutil.CreateTestCategory(t, db, testOrg.ID, map[string]interface{}{
					"name": "Category 2",
					"slug": "category-2",
				})

				// Create product with category assignments
				productRequest := &pb.CreateProductRequest{
					Sku:         "MULTI-CAT-001",
					Name:        "Multi-Category Product",
					BasePrice:   9999,
					CategoryIds: []string{cat1.ID.String(), cat2.ID.String()},
				}

				body, _ := json.Marshal(productRequest)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(body))
				req = addOrgContext(req, testOrg.ID)

				rec := httptest.NewRecorder()
				productHandler.Create(rec, req)

				if rec.Code != http.StatusCreated {
					t.Fatalf("Expected status %d, got %d. Body: %s", http.StatusCreated, rec.Code, rec.Body.String())
				}

				var response pb.CreateProductResponse
				json.NewDecoder(rec.Body).Decode(&response)

				// Verify product was created
				if response.Product.Id == "" {
					t.Error("Expected product ID to be set")
				}

				// Verify category assignments in database
				var associations []struct {
					ProductID  uuid.UUID
					CategoryID uuid.UUID
				}
				db.Raw("SELECT product_id, category_id FROM product_categories WHERE product_id = ?", response.Product.Id).Scan(&associations)

				if len(associations) != 2 {
					t.Errorf("Expected 2 category assignments, got %d", len(associations))
				}
			},
		},
		{
			name:     "US2-AS3: Remove product from one category",
			scenario: "Given a product is assigned to multiple categories, When I remove it from one category, Then the product is removed from only that category and remains in others",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)
				testOrg := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Create categories
				cat1 := testutil.CreateTestCategory(t, db, testOrg.ID, map[string]interface{}{
					"name": "Category 1",
					"slug": "cat-1",
				})
				cat2 := testutil.CreateTestCategory(t, db, testOrg.ID, map[string]interface{}{
					"name": "Category 2",
					"slug": "cat-2",
				})

				// Create product with both categories
				productRequest := &pb.CreateProductRequest{
					Sku:         "REMOVE-CAT-001",
					Name:        "Test Product",
					BasePrice:   5000,
					CategoryIds: []string{cat1.ID.String(), cat2.ID.String()},
				}

				body, _ := json.Marshal(productRequest)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(body))
				req = addOrgContext(req, testOrg.ID)

				rec := httptest.NewRecorder()
				productHandler.Create(rec, req)

				var createResponse pb.CreateProductResponse
				json.NewDecoder(rec.Body).Decode(&createResponse)

				// Update product to remove one category
				updateRequest := &pb.UpdateProductRequest{
					CategoryIds: []string{cat1.ID.String()}, // Only cat1 now
				}

				body2, _ := json.Marshal(updateRequest)
				req2 := httptest.NewRequest(http.MethodPut, "/api/v1/products/"+createResponse.Product.Id, bytes.NewReader(body2))
				req2 = addOrgContext(req2, testOrg.ID)

				rec2 := httptest.NewRecorder()
				productHandler.Update(rec2, req2)

				if rec2.Code != http.StatusOK {
					t.Errorf("Expected status %d, got %d", http.StatusOK, rec2.Code)
				}

				// Verify only one category association remains
				var count int64
				db.Model(&models.ProductCategory{}).
					Where("product_id = ?", createResponse.Product.Id).
					Count(&count)

				if count != 1 {
					t.Errorf("Expected 1 category assignment, got %d", count)
				}
			},
		},
		{
			name:     "US2-AS4: Delete parent category with confirmation",
			scenario: "Given a category has sub-categories, When I delete the parent category, Then I am prompted to either reassign or delete all sub-categories and products",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)
				testOrg := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Create parent-child category structure
				parent := testutil.CreateTestCategory(t, db, testOrg.ID, map[string]interface{}{
					"name": "Parent Category",
					"slug": "parent",
				})

				child := testutil.CreateTestCategory(t, db, testOrg.ID, map[string]interface{}{
					"name":      "Child Category",
					"slug":      "child",
					"parent_id": &parent.ID,
				})

				// Attempt to delete parent without force
				req := httptest.NewRequest(http.MethodDelete, "/api/v1/categories/"+parent.ID.String(), nil)
				req = addOrgContext(req, testOrg.ID)

				rec := httptest.NewRecorder()
				categoryHandler.Delete(rec, req)

				// Should fail because of children
				if rec.Code != http.StatusConflict {
					t.Errorf("Expected status %d (Conflict), got %d", http.StatusConflict, rec.Code)
				}

				// Verify parent still exists
				var parentExists int64
				db.Model(&models.Category{}).Where("id = ?", parent.ID).Count(&parentExists)
				if parentExists != 1 {
					t.Error("Expected parent category to still exist")
				}

				// Delete with force=true
				req2 := httptest.NewRequest(http.MethodDelete, "/api/v1/categories/"+parent.ID.String()+"?force=true", nil)
				req2 = addOrgContext(req2, testOrg.ID)

				rec2 := httptest.NewRecorder()
				categoryHandler.Delete(rec2, req2)

				if rec2.Code != http.StatusOK {
					t.Errorf("Expected status %d, got %d. Body: %s", http.StatusOK, rec2.Code, rec2.Body.String())
				}

				var deleteResponse pb.DeleteCategoryResponse
				json.NewDecoder(rec2.Body).Decode(&deleteResponse)

				if !deleteResponse.Success {
					t.Error("Expected success to be true")
				}

				// Verify parent and child are deleted
				var remainingCount int64
				db.Model(&models.Category{}).Where("id IN ?", []uuid.UUID{parent.ID, child.ID}).Count(&remainingCount)
				if remainingCount != 0 {
					t.Errorf("Expected 0 categories remaining, got %d", remainingCount)
				}
			},
		},
	}

	// Run all test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFunc(t)
		})
	}
}

// TestCategoryEdgeCases tests edge cases for category operations
func TestCategoryEdgeCases(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	defer testutil.TruncateTables(db)

	org := testutil.CreateTestOrganization(t, db, map[string]interface{}{})
	categoryService := services.NewCategoryService(db)
	categoryHandler := handlers.NewCategoryHandler(categoryService)

	addOrgContext := func(r *http.Request, orgID uuid.UUID) *http.Request {
		ctx := context.WithValue(r.Context(), middleware.OrganizationIDKey, orgID)
		return r.WithContext(ctx)
	}

	t.Run("Circular reference prevention", func(t *testing.T) {
		// Create parent and child
		parent := testutil.CreateTestCategory(t, db, org.ID, map[string]interface{}{
			"name": "Parent",
			"slug": "parent",
		})

		child := testutil.CreateTestCategory(t, db, org.ID, map[string]interface{}{
			"name":      "Child",
			"slug":      "child",
			"parent_id": &parent.ID,
		})

		// Attempt to make parent a child of child (circular reference)
		updateRequest := &pb.UpdateCategoryRequest{
			ParentId: child.ID.String(),
		}

		body, _ := json.Marshal(updateRequest)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/categories/"+parent.ID.String(), bytes.NewReader(body))
		req = addOrgContext(req, org.ID)

		rec := httptest.NewRecorder()
		categoryHandler.Update(rec, req)

		// Should return error
		if rec.Code == http.StatusOK {
			t.Error("Expected error for circular reference, but got success")
		}
	})

	t.Run("Non-existent category returns 404", func(t *testing.T) {
		nonExistentID := uuid.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/categories/"+nonExistentID.String(), nil)
		req = addOrgContext(req, org.ID)

		rec := httptest.NewRecorder()
		categoryHandler.Get(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("Category tree retrieval", func(t *testing.T) {
		// Create hierarchical structure
		root, child, grandchild := testutil.CreateTestCategoryTree(t, db, org.ID)

		// Get tree
		req := httptest.NewRequest(http.MethodGet, "/api/v1/categories/tree", nil)
		req = addOrgContext(req, org.ID)

		rec := httptest.NewRecorder()
		categoryHandler.GetTree(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var treeResponse pb.GetCategoryTreeResponse
		json.NewDecoder(rec.Body).Decode(&treeResponse)

		// Should have root categories
		if len(treeResponse.Trees) == 0 {
			t.Error("Expected at least one root category in tree")
		}

		// Verify hierarchy depth
		rootTree := treeResponse.Trees[0]
		if len(rootTree.Children) == 0 {
			t.Error("Expected root to have children")
		}

		t.Logf("✅ Created hierarchy: %s -> %s -> %s", root.Name, child.Name, grandchild.Name)
	})
}

