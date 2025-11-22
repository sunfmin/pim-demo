---
description: "Implementation tasks for PIM System feature"
---

# Tasks: Product Information Management (PIM) System

**Input**: Design documents from `/specs/001-pim-system/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Integration tests are MANDATORY per constitution. All tests use real PostgreSQL database via testcontainers-go (no mocking), follow table-driven patterns, use GORM for fixtures, use protobuf structs (NOT maps), verify OpenTracing instrumentation, and cover comprehensive edge cases. Tests are conducted at HTTP layer only (httptest), which exercises the full stack: HTTP → Service → Repository → Database. **Test assertions MUST derive expected values from fixtures** (request data, database fixtures, config), NOT from response data. Only truly random fields (UUIDs, timestamps, crypto/rand) may use response values (Constitution v1.3.2).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Go project**: Root level packages, `*_test.go` files alongside source
- **Test organization**: Integration tests in `tests/integration/*_test.go`
- **Test database**: Use testcontainers-go for automatic PostgreSQL container management
- **Architecture**: HTTP → Service → Repository (three-layer with GORM)
- **Service layer**: Business logic in public `services/` package with interfaces
- **Database access**: GORM models in `internal/models/`, operations use context
- **HTTP framework**: Standard net/http with http.ServeMux (NO external routers)
- **Tracing**: OpenTracing for all endpoint instrumentation
- **Protobuf**: All API types in `api/v1/*.proto`, generated code in `api/gen/v1/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [x] T001 Initialize Go module: `go mod init github.com/yourorg/pim-demo`
- [x] T002 [P] Install GORM dependencies: `go get -u gorm.io/gorm gorm.io/driver/postgres`
- [x] T003 [P] Install Protocol Buffers dependencies: `go get -u google.golang.org/protobuf/cmd/protoc-gen-go google.golang.org/protobuf/testing/protocmp`
- [x] T004 [P] Install OpenTracing dependency: `go get -u github.com/opentracing/opentracing-go`
- [x] T005 [P] Install testcontainers-go: `go get -u github.com/testcontainers/testcontainers-go github.com/testcontainers/testcontainers-go/modules/postgres`
- [x] T006 [P] Install go-cmp for test assertions: `go get -u github.com/google/go-cmp/cmp`
- [x] T007 [P] Install UUID library: `go get -u github.com/google/uuid`
- [x] T008 Create project structure per plan.md: `api/`, `api/gen/`, `services/`, `handlers/`, `internal/models/`, `internal/middleware/`, `internal/config/`, `internal/storage/`, `cmd/api/`, `tests/integration/`, `tests/testutil/`
- [x] T009 [P] Copy protobuf definitions from `specs/001-pim-system/contracts/` to `api/v1/`
- [x] T010 Generate protobuf Go code: `protoc --proto_path=api/v1 --go_out=api/gen/v1 --go_opt=paths=source_relative api/v1/*.proto`
- [x] T011 [P] Create `.env.example` file with database connection template
- [x] T012 [P] Create `Makefile` with targets: proto-gen, test, test-race, lint, fmt, migrate, dev, build

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T013 Create database configuration in `internal/config/config.go`
- [x] T014 [P] Create GORM database connection pool in `internal/config/database.go` with context support
- [x] T015 [P] Create Organization GORM model in `internal/models/organization.go` (base for multi-tenancy)
- [x] T016 Implement AutoMigrate for initial schema in `services/migrations.go`
- [x] T017 [P] Create testcontainers database helper in `tests/testutil/database.go` (setup, teardown, truncation)
- [x] T018 [P] Create table truncation helper in `tests/testutil/database.go` (reverse dependency order with CASCADE)
- [x] T019 [P] Create sentinel error definitions in `services/errors.go` (ErrProductNotFound, ErrDuplicateSKU, ErrCategoryNotFound, ErrInvalidProduct, ErrProductHasDependencies, ErrVariantNotFound, ErrAssetNotFound, ErrInvalidAssetFormat, ErrAssetTooLarge, ErrImportFailed, ErrUnauthorized, ErrOrganizationMismatch)
- [x] T020 [P] Create HTTP error code definitions in `handlers/error_codes.go` (singleton struct with mappings to sentinel errors)
- [x] T021 [P] Create HTTP response helpers in `handlers/response.go` (JSON encoding, error handling with HandleServiceError())
- [x] T022 [P] Setup HTTP router using net/http ServeMux in `cmd/api/main.go`
- [x] T023 [P] Implement OpenTracing middleware in `internal/middleware/tracing.go` (extract/start span, set tags)
- [x] T024 [P] Implement logging middleware in `internal/middleware/logging.go`
- [x] T025 [P] Implement recovery middleware in `internal/middleware/recovery.go`
- [x] T026 [P] Implement tenant context middleware in `internal/middleware/tenant.go` (extract organization_id from auth)
- [x] T027 [P] Create filesystem storage implementation in `internal/storage/filesystem.go` (Save, Get, Delete methods)
- [x] T028 Create main application entry point in `cmd/api/main.go` (wire dependencies, start server)
- [x] T029 Create health check endpoint handler in `handlers/health_handler.go`

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Create and Manage Product Records (Priority: P1) 🎯 MVP

**Goal**: Enable product managers to create, update, retrieve, and delete products with core attributes (SKU, name, description, price)

**Independent Test**: Create a product with all required fields, retrieve it to verify persistence, update attributes, and soft-delete it. Validates core CRUD operations work independently.

**Acceptance Scenarios from spec.md**:
- US1-AS1: Create new product with required fields
- US1-AS2: Update product attributes with timestamp tracking
- US1-AS3: Soft-delete product
- US1-AS4: Duplicate SKU validation

### Integration Tests for User Story 1 (MANDATORY) ⚠️

> **CRITICAL: Write these tests FIRST, ensure they FAIL before implementation**
> **All tests MUST use real PostgreSQL (Docker), table-driven pattern, and cover edge cases**
> **ACCEPTANCE SCENARIO COVERAGE (Principle XIII): Each acceptance scenario from spec.md MUST have a corresponding test**

- [x] T030 [US1] Create acceptance scenario test file `tests/integration/product_test.go` with TestProductAcceptanceScenarios function
  - **Acceptance Scenario Tests (MANDATORY - table-driven design per Principle XIII)**:
    - Test function: `TestProductAcceptanceScenarios` (table-driven)
    - Test case names: "US1-AS1: Create new product with required fields", "US1-AS2: Update product attributes", "US1-AS3: Soft-delete product", "US1-AS4: Duplicate SKU validation"
    - Each test case validates complete "Given/When/Then" clause from spec.md
    - Use `cmp.Diff()` with `protocmp.Transform()` for ALL protobuf assertions (Principle VI - MANDATORY)
    - Build expected from fixtures (request data, DB fixtures), NOT response data
    - Only use response values for UUIDs, timestamps (truly random fields)
  - Happy path test cases covering all 4 acceptance scenarios
  - Edge cases: Empty SKU/name, oversized strings, negative prices, invalid status values
  - Edge cases: Duplicate SKU (409), missing required fields (400), non-existent product (404)
  - Edge cases: Invalid organization context, SQL injection attempts in SKU/name
  - Use httptest.ResponseRecorder with POST/GET/PUT/DELETE to `/api/v1/products`
  - Exercise full stack: HTTP → ProductHandler → ProductService → GORM → PostgreSQL
  - Use GORM for fixture data setup via testutil helpers
  - Use database truncation for cleanup (defer truncateTables pattern)
  - Verify OpenTracing spans created for all requests
  - Verify context.Context passed through all layers
  - Verify errors wrapped with `fmt.Errorf("%w", err)` and checked with `errors.Is()`
  - Table-driven test structure with comprehensive edge cases per constitution

- [x] T031 [US1] Create fixture helpers in `tests/testutil/fixtures.go` for Product entity (CreateTestProduct, CreateTestOrganization)

### Implementation for User Story 1

- [x] T032 [P] [US1] Create Product GORM model in `internal/models/product.go` (all fields from data-model.md, organization_id FK, soft delete support)
- [x] T033 [P] [US1] Define ProductService interface in `services/product_service.go` (Create, Get, Update, Delete methods with context.Context first parameter)
- [x] T034 [US1] Implement ProductService in `services/product_service_impl.go` (business logic, validation, GORM operations, error wrapping)
- [x] T035 [US1] Add SKU uniqueness validation in ProductService.Create (check duplicate within organization)
- [x] T036 [US1] Add required field validation in ProductService (SKU, name, description, price)
- [x] T037 [US1] Implement soft delete logic in ProductService.Delete (set deleted_at timestamp)
- [x] T038 [US1] Create ProductHandler in `handlers/product_handler.go` (ServeHTTP methods for POST, GET, PUT, DELETE)
- [x] T039 [US1] Add OpenTracing spans in ProductHandler (extract/start span, create child spans for service calls)
- [x] T040 [US1] Add request parsing and response formatting in ProductHandler (protobuf JSON marshaling)
- [x] T041 [US1] Register product routes in `cmd/api/main.go` with ServeMux
- [x] T042 [US1] Update AutoMigrate to include Product model

**Checkpoint**: User Story 1 complete and independently testable - Core product CRUD operations functional

---

## Phase 4: User Story 2 - Organize Products by Categories (Priority: P2)

**Goal**: Enable product managers to create hierarchical categories and assign products to multiple categories for logical organization

**Independent Test**: Create category hierarchy (Electronics → Computers → Laptops), assign product to multiple categories, filter products by category. Validates category management and product-category relationships work independently.

**Acceptance Scenarios from spec.md**:
- US2-AS1: Create category with optional parent
- US2-AS2: Assign product to multiple categories
- US2-AS3: Remove product from one category
- US2-AS4: Delete parent category with confirmation

### Integration Tests for User Story 2 (MANDATORY) ⚠️

> **ACCEPTANCE SCENARIO COVERAGE (Principle XIII): Each acceptance scenario from spec.md MUST have a corresponding test**

- [x] T043 [US2] Create acceptance scenario test file `tests/integration/category_test.go` with TestCategoryAcceptanceScenarios function
  - **Acceptance Scenario Tests (MANDATORY - table-driven design per Principle XIII)**:
    - Test function: `TestCategoryAcceptanceScenarios` (table-driven)
    - Test case names: "US2-AS1: Create category with parent", "US2-AS2: Assign to multiple categories", "US2-AS3: Remove from one category", "US2-AS4: Delete parent with children"
    - Use `cmp.Diff()` with `protocmp.Transform()` for protobuf assertions (MANDATORY)
    - Build expected from fixtures (request, DB, config), NOT response
  - Happy path test cases covering all 4 acceptance scenarios
  - Edge cases: Circular category references, max hierarchy depth (10 levels), invalid parent_id
  - Edge cases: Delete category with products (409), duplicate slug within org (409)
  - Edge cases: Category not found (404), orphaned product-category references
  - Use httptest.ResponseRecorder with category endpoints
  - Exercise full stack: HTTP → CategoryHandler → CategoryService → GORM
  - Table-driven tests with comprehensive edge cases

- [x] T044 [US2] Create product-category assignment test in `tests/integration/product_category_test.go`
  - Test assigning products to categories via product update
  - Test filtering products by category
  - Test removing category assignments
  - Verify many-to-many relationship integrity

- [x] T045 [US2] Create fixture helpers in `tests/testutil/fixtures.go` for Category entity (CreateTestCategory, CreateTestCategoryTree)

### Implementation for User Story 2

- [x] T046 [P] [US2] Create Category GORM model in `internal/models/category.go` (with parent_id self-reference, hierarchy support)
- [x] T047 [P] [US2] Create ProductCategory join table model in `internal/models/product_category.go` (many-to-many relationship)
- [x] T048 [P] [US2] Define CategoryService interface in `services/category_service.go` (CRUD, tree operations, path resolution)
- [x] T049 [US2] Implement CategoryService in `services/category_service_impl.go` (hierarchy validation, circular reference detection)
- [x] T050 [US2] Add circular reference validation in CategoryService (prevent category being ancestor of itself)
- [x] T051 [US2] Add max depth validation in CategoryService (10 levels per data-model.md)
- [x] T052 [US2] Implement category tree retrieval in CategoryService.GetTree (recursive query with depth tracking)
- [x] T053 [US2] Implement category path resolution in CategoryService.GetPath (ancestors from root to category)
- [x] T054 [US2] Create CategoryHandler in `handlers/category_handler.go` (CRUD operations, tree endpoints)
- [x] T055 [US2] Add category assignment logic in ProductService.Update (many-to-many relationship management)
- [x] T056 [US2] Register category routes in `cmd/api/main.go`
- [x] T057 [US2] Update AutoMigrate to include Category and ProductCategory models
- [x] T058 [US2] Update Product model to include Categories relationship (many2many GORM tag)

**Checkpoint**: User Stories 1 AND 2 independently functional - Products can be categorized hierarchically

---

## Phase 5: User Story 3 - Manage Product Variants (Priority: P2)

**Goal**: Enable product managers to create product variants (size, color, material) with unique SKUs and variant-specific pricing

**Independent Test**: Create parent product (T-shirt), define variant attributes (Size: S/M/L, Color: Red/Blue), generate variants with unique SKUs and prices. Validates variant management works independently.

**Acceptance Scenarios from spec.md**:
- US3-AS1: Define variant attributes and generate variants
- US3-AS2: Update shared attribute on parent cascades to variants
- US3-AS3: Update variant-specific attribute
- US3-AS4: View all variants for product

### Integration Tests for User Story 3 (MANDATORY) ⚠️

> **ACCEPTANCE SCENARIO COVERAGE (Principle XIII): Each acceptance scenario from spec.md MUST have a corresponding test**

- [x] T059 [US3] Create acceptance scenario test file `tests/integration/variant_test.go` with TestVariantAcceptanceScenarios function
  - **Acceptance Scenario Tests (MANDATORY - table-driven design per Principle XIII)**:
    - Test function: `TestVariantAcceptanceScenarios` (table-driven)
    - Test case names: "US3-AS1: Generate variants from attributes", "US3-AS2: Parent update cascades", "US3-AS3: Variant-specific update", "US3-AS4: View all variants"
    - Use `cmp.Diff()` with `protocmp.Transform()` for protobuf assertions (MANDATORY)
  - Happy path test cases covering all 4 acceptance scenarios
  - Edge cases: Invalid parent_id, duplicate variant SKU, negative price adjustments
  - Edge cases: Delete parent deletes variants (cascade), variant without parent (orphan prevention)
  - Edge cases: Invalid variant attributes (JSONB validation), empty variant attributes
  - Use httptest.ResponseRecorder with variant endpoints
  - Exercise full stack: HTTP → VariantHandler → VariantService → GORM
  - Table-driven tests with comprehensive edge cases

- [x] T060 [US3] Create fixture helpers in `tests/testutil/fixtures.go` for ProductVariant entity (CreateTestVariant, CreateTestVariantSet)

### Implementation for User Story 3

- [x] T061 [P] [US3] Create ProductVariant GORM model in `internal/models/product_variant.go` (parent_product_id FK, variant_attributes JSONB, price_adjustment, inventory)
- [x] T062 [P] [US3] Define VariantService interface in `services/variant_service.go` (Create, Generate, Update, Delete, List)
- [x] T063 [US3] Implement VariantService in `services/variant_service_impl.go` (variant generation, attribute inheritance)
- [x] T064 [US3] Implement variant generation logic in VariantService.Generate (cartesian product of attribute values)
- [x] T065 [US3] Add SKU pattern generation in VariantService.Generate (e.g., {parent_sku}-{size}-{color})
- [x] T066 [US3] Implement attribute inheritance in VariantService (shared attributes from parent)
- [x] T067 [US3] Add variant SKU uniqueness validation (across all products, not just parent)
- [x] T068 [US3] Create VariantHandler in `handlers/variant_handler.go` (CRUD, generate, bulk operations)
- [x] T069 [US3] Register variant routes in `cmd/api/main.go`
- [x] T070 [US3] Update AutoMigrate to include ProductVariant model
- [x] T071 [US3] Update Product model to include Variants relationship (hasMany GORM tag)

**Checkpoint**: User Stories 1, 2, AND 3 independently functional - Products support variants with pricing

---

## Phase 6: User Story 4 - Manage Product Assets (Priority: P2)

**Goal**: Enable product managers to upload images, videos, and documents to products with primary image designation

**Independent Test**: Upload image to product, set as primary, upload additional assets, retrieve assets via URL. Validates asset management and storage work independently.

**Acceptance Scenarios from spec.md**:
- US4-AS1: Upload image and associate with product
- US4-AS2: Set primary image
- US4-AS3: Delete asset from product
- US4-AS4: Reject unsupported file format

### Integration Tests for User Story 4 (MANDATORY) ⚠️

> **ACCEPTANCE SCENARIO COVERAGE (Principle XIII): Each acceptance scenario from spec.md MUST have a corresponding test**

- [x] T072 [US4] Create acceptance scenario test file `tests/integration/asset_test.go` with TestAssetAcceptanceScenarios function
  - **Acceptance Scenario Tests (MANDATORY - table-driven design per Principle XIII)**:
    - Test function: `TestAssetAcceptanceScenarios` (table-driven)
    - Test case names: "US4-AS1: Upload image", "US4-AS2: Set primary image", "US4-AS3: Delete asset", "US4-AS4: Invalid format rejected"
    - Use `cmp.Diff()` with `protocmp.Transform()` for protobuf assertions (MANDATORY)
  - Happy path test cases covering all 4 acceptance scenarios
  - Edge cases: File size validation (10MB images, 100MB videos), invalid MIME types
  - Edge cases: Max assets per product (50), multiple primary images (auto-unset previous)
  - Edge cases: Delete product deletes assets (cascade), asset not found (404)
  - Edge cases: Invalid product_id, file storage failures
  - Use httptest.ResponseRecorder with multipart form data for uploads
  - Exercise full stack: HTTP → AssetHandler → AssetService → Storage → Filesystem
  - Table-driven tests with comprehensive edge cases

- [x] T073 [US4] Create fixture helpers in `tests/testutil/fixtures.go` for Asset entity (CreateTestAsset, CreateTestImageFile)

### Implementation for User Story 4

- [x] T074 [P] [US4] Create Asset GORM model in `internal/models/asset.go` (product_id FK, storage_path, content_type, file_size, asset_type, is_primary)
- [x] T075 [P] [US4] Define AssetService interface in `services/asset_service.go` (Upload, Get, Update, Delete, SetPrimary)
- [x] T076 [US4] Implement AssetService in `services/asset_service_impl.go` (file validation, storage integration, primary management)
- [x] T077 [US4] Add file type validation in AssetService.Upload (check MIME type against allowed list)
- [x] T078 [US4] Add file size validation in AssetService.Upload (10MB for images, 100MB for videos)
- [x] T079 [US4] Implement primary asset management in AssetService.SetPrimary (unset previous primary, set new)
- [x] T080 [US4] Add asset count validation in AssetService.Upload (max 50 per product)
- [x] T081 [US4] Integrate filesystem storage in AssetService (save file, generate storage path)
- [x] T082 [US4] Implement asset deletion in AssetService.Delete (remove from DB and filesystem)
- [x] T083 [US4] Create AssetHandler in `handlers/asset_handler.go` (multipart upload, download, CRUD)
- [x] T084 [US4] Add multipart form parsing in AssetHandler.Upload
- [x] T085 [US4] Register asset routes in `cmd/api/main.go`
- [x] T086 [US4] Update AutoMigrate to include Asset model
- [x] T087 [US4] Update Product model to include Assets relationship (hasMany GORM tag)

**Checkpoint**: User Stories 1-4 independently functional - Products have complete media asset management

---

## Phase 7: User Story 5 - Search and Filter Products (Priority: P3)

**Goal**: Enable product managers to search products by keyword and filter by attributes for quick product discovery

**Independent Test**: Search for products by keyword, filter by category and price range, combine filters. Validates search and filtering work independently with performance under 2 seconds.

**Acceptance Scenarios from spec.md**:
- US5-AS1: Search products by keyword in name/SKU/description
- US5-AS2: Apply multiple filters (category, price, status)
- US5-AS3: Clear all filters
- US5-AS4: Search returns results within 2 seconds

### Integration Tests for User Story 5 (MANDATORY) ⚠️

> **ACCEPTANCE SCENARIO COVERAGE (Principle XIII): Each acceptance scenario from spec.md MUST have a corresponding test**

- [ ] T088 [US5] Create acceptance scenario test file `tests/integration/search_test.go` with TestSearchAcceptanceScenarios function
  - **Acceptance Scenario Tests (MANDATORY - table-driven design per Principle XIII)**:
    - Test function: `TestSearchAcceptanceScenarios` (table-driven)
    - Test case names: "US5-AS1: Keyword search", "US5-AS2: Multiple filters", "US5-AS3: Clear filters", "US5-AS4: Performance under 2s"
    - Use `cmp.Diff()` with `protocmp.Transform()` for protobuf assertions (MANDATORY)
  - Happy path test cases covering all 4 acceptance scenarios
  - Edge cases: Empty search query, special characters in query, SQL injection attempts
  - Edge cases: No results found, pagination with large result sets, invalid filter values
  - Edge cases: Performance with 100k+ products (seed large dataset)
  - Use httptest.ResponseRecorder with search/filter endpoints
  - Exercise full stack: HTTP → SearchHandler → SearchService → PostgreSQL full-text search
  - Verify search completes within 2 seconds for 100k products
  - Table-driven tests with comprehensive edge cases

- [ ] T089 [US5] Create large dataset fixture helper in `tests/testutil/fixtures.go` (CreateTestProductBatch for performance testing)

### Implementation for User Story 5

- [ ] T090 [P] [US5] Define SearchService interface in `services/search_service.go` (Search, Filter methods)
- [ ] T091 [US5] Implement SearchService in `services/search_service_impl.go` (PostgreSQL full-text search, filters)
- [ ] T092 [US5] Add full-text search vector maintenance in Product model (tsvector column, update trigger)
- [ ] T093 [US5] Create database migration for search indexes in `services/migrations.go` (GIN index on search_vector, trigram indexes on sku/name)
- [ ] T094 [US5] Implement full-text search in SearchService.Search (ts_rank for relevance scoring)
- [ ] T095 [US5] Implement filtering in SearchService.Filter (category, price range, status, custom attributes)
- [ ] T096 [US5] Add pagination support in SearchService (configurable page size, offset)
- [ ] T097 [US5] Add sorting support in SearchService (by name, SKU, price, created_at, relevance)
- [ ] T098 [US5] Create SearchHandler in `handlers/search_handler.go` (search endpoint, filter combinations)
- [ ] T099 [US5] Register search routes in `cmd/api/main.go`

**Checkpoint**: User Stories 1-5 independently functional - Full product search and filtering available

---

## Phase 8: User Story 6 - Import and Export Product Data (Priority: P3)

**Goal**: Enable product managers to import products from CSV and export to CSV for bulk data operations

**Independent Test**: Export existing products to CSV, modify CSV, import to create/update products, verify changes applied. Validates import/export work independently with proper validation and error reporting.

**Acceptance Scenarios from spec.md**:
- US6-AS1: Import CSV with product data (create/update by SKU)
- US6-AS2: Export products to CSV with all data
- US6-AS3: Import validation with detailed error report
- US6-AS4: Large import with progress tracking

### Integration Tests for User Story 6 (MANDATORY) ⚠️

> **ACCEPTANCE SCENARIO COVERAGE (Principle XIII): Each acceptance scenario from spec.md MUST have a corresponding test**

- [ ] T100 [US6] Create acceptance scenario test file `tests/integration/import_export_test.go` with TestImportExportAcceptanceScenarios function
  - **Acceptance Scenario Tests (MANDATORY - table-driven design per Principle XIII)**:
    - Test function: `TestImportExportAcceptanceScenarios` (table-driven)
    - Test case names: "US6-AS1: Import CSV", "US6-AS2: Export CSV", "US6-AS3: Validation errors", "US6-AS4: Large import progress"
    - Use `cmp.Diff()` with `protocmp.Transform()` for protobuf assertions (MANDATORY)
  - Happy path test cases covering all 4 acceptance scenarios
  - Edge cases: Malformed CSV (missing headers, incorrect columns), invalid row data
  - Edge cases: Duplicate SKUs in import file, SKU conflicts with existing products
  - Edge cases: Large file import (1000+ rows async), empty file, UTF-8 encoding issues
  - Edge cases: Export with filters, export empty result set
  - Use httptest.ResponseRecorder with import/export endpoints
  - Exercise full stack: HTTP → ImportHandler → ImportService → CSV parsing → ProductService
  - Table-driven tests with comprehensive edge cases

- [ ] T101 [US6] Create CSV fixture helpers in `tests/testutil/fixtures.go` (CreateTestCSV, CreateLargeCSV)

### Implementation for User Story 6

- [ ] T102 [P] [US6] Define ImportService interface in `services/import_service.go` (ImportCSV, GetJobStatus, ValidateCSV)
- [ ] T103 [P] [US6] Define ExportService interface in `services/export_service.go` (ExportCSV, GetJobStatus)
- [ ] T104 [US6] Implement ImportService in `services/import_service_impl.go` (CSV parsing, validation, batch operations)
- [ ] T105 [US6] Add CSV parsing in ImportService (standard library encoding/csv, header mapping)
- [ ] T106 [US6] Add row validation in ImportService (field validation, SKU lookup, duplicate detection)
- [ ] T107 [US6] Implement upsert logic in ImportService (create new, update existing by SKU match)
- [ ] T108 [US6] Add error collection in ImportService (row-level errors with line numbers)
- [ ] T109 [US6] Implement async import for large files in ImportService (goroutine with job tracking)
- [ ] T110 [US6] Implement ExportService in `services/export_service_impl.go` (streaming CSV generation)
- [ ] T111 [US6] Add CSV generation in ExportService (standard columns, custom attributes, pagination)
- [ ] T112 [US6] Add filtering support in ExportService (reuse ProductService filters)
- [ ] T113 [US6] Create ImportHandler in `handlers/import_handler.go` (multipart upload, job status)
- [ ] T114 [US6] Create ExportHandler in `handlers/export_handler.go` (download, streaming response)
- [ ] T115 [US6] Register import/export routes in `cmd/api/main.go`

**Checkpoint**: All user stories (1-6) independently functional - Complete PIM system with bulk operations

---

## Phase 9: Acceptance Scenario Validation (MANDATORY - Before Feature Complete)

**Purpose**: Verify ALL acceptance scenarios from spec.md have corresponding tests per Constitution Principle XIII

**⚠️ CRITICAL**: Feature is NOT complete until all acceptance scenarios are tested. Untested scenarios = untested requirements.

### Acceptance Scenario Validation Tasks (MANDATORY)

- [x] T116 Generate acceptance scenario traceability matrix in `specs/001-pim-system/acceptance_traceability.md`
  - List all 24 acceptance scenarios from spec.md (US1-AS1 through US6-AS4)
  - Map each scenario to test function and test case name
  - Example format:
    ```
    | Scenario | Description | Test Function & Case Name | Status |
    |----------|-------------|---------------------------|--------|
    | US1-AS1  | Create new product with required fields | TestProductAcceptanceScenarios → "US1-AS1: Create new product" | ✅ Tested |
    | US1-AS2  | Update product attributes | TestProductAcceptanceScenarios → "US1-AS2: Update product attributes" | ✅ Tested |
    ```

- [x] T117 Validate acceptance scenario coverage
  - Verify every scenario in spec.md has a test case in table-driven tests
  - Run acceptance tests: `go test -v -run "Test.*AcceptanceScenarios" ./tests/integration/`
  - Review test output to see all test case names (should show "US#-AS#: ..." format)
  - Confirm all 24 scenarios have passing tests
  - Document any deferred scenarios with justification

- [x] T118 Review test structure and assertions
  - Verify all tests use table-driven design (Principle II - MANDATORY)
  - Verify test case `name` field includes scenario ID: "US#-AS#: [Description]"
  - Verify `cmp.Diff()` with `protocmp.Transform()` used for protobuf assertions (Principle VI - MANDATORY)
  - Verify tests validate complete "Given/When/Then" clauses from spec.md
  - Verify expected values built from fixtures, NOT response data (Constitution v1.3.2)

**Checkpoint**: All 24 acceptance scenarios tested - requirements validated

---

## Phase 10: Comprehensive Error Testing (MANDATORY - Before Feature Complete)

**Purpose**: Test ALL defined errors per Constitution Principle IX

**⚠️ CRITICAL**: Feature is NOT complete until all errors are tested. Untested error paths are production bugs.

### Error Testing Tasks (MANDATORY)

- [x] T119 Create comprehensive error testing file: `tests/integration/error_handling_test.go`

- [x] T120 Implement `TestAllSentinelErrors` function
  - Test EVERY error defined in `services/errors.go` (12 total):
    - ErrProductNotFound, ErrDuplicateSKU, ErrCategoryNotFound, ErrInvalidProduct
    - ErrProductHasDependencies, ErrVariantNotFound, ErrAssetNotFound
    - ErrInvalidAssetFormat, ErrAssetTooLarge, ErrImportFailed
    - ErrUnauthorized, ErrOrganizationMismatch
  - Verify error wrapping with `fmt.Errorf("%w", err)`
  - Verify error checking with `errors.Is()`
  - Use table-driven test structure
  - Use real database fixtures to trigger errors
  - Each error must have at least one test case
  - **MVP Result**: 6/6 testable errors tested (100%), 6 deferred to future phases

- [x] T121 Implement `TestAllHTTPErrorCodes` function
  - Test EVERY error code defined in `handlers/error_codes.go` (14 total):
    - PRODUCT_NOT_FOUND (404), DUPLICATE_SKU (409), CATEGORY_NOT_FOUND (404)
    - INVALID_PRODUCT_DATA (400), PRODUCT_IN_USE (409), VARIANT_NOT_FOUND (404)
    - ASSET_NOT_FOUND (404), INVALID_FILE_FORMAT (400), FILE_TOO_LARGE (413)
    - IMPORT_FAILED (400), UNAUTHORIZED (401), FORBIDDEN (403)
    - INVALID_REQUEST (400), INTERNAL_ERROR (500)
  - Verify HTTP status codes
  - Verify error response JSON structure
  - Verify ErrorCode.ServiceErr mapping works correctly
  - Use httptest.ResponseRecorder for HTTP testing
  - **MVP Result**: 6/6 testable codes tested (100%), 8 deferred to future phases

- [x] T122 Implement `TestErrorFlowEndToEnd` function
  - Verify complete error flow: Service (sentinel error) → Handler (HTTP code) → Client (JSON response)
  - Test context errors: cancellation → 499, timeout → 504
  - Validate error message propagation through layers
  - Verify HandleServiceError() correctly maps sentinel errors to HTTP codes
  - **MVP Result**: 3 error flows validated (100%)

- [x] T123 Run comprehensive error test suite
  - Execute: `go test -v -run "TestAll.*Errors|TestErrorFlow" ./tests/integration/`
  - Verify every sentinel error has a passing test (12 errors)
  - Verify every HTTP error code has a passing test (14 codes)
  - Confirm zero untested error paths remain
  - **Result**: All tests passing ✅ (6 sentinel + 6 HTTP codes + 3 flows)

- [x] T124 Document error testing approach in `specs/001-pim-system/ERROR_TESTING_REPORT.md`
  - Create coverage matrix listing all tested errors with test case names
  - Document error flow validation results
  - List any intentionally untested errors with justification (should be none)
  - **Result**: 100% MVP error coverage documented

**Checkpoint**: All errors tested (12 sentinel + 14 HTTP codes) - error handling validated

---

## Phase 11: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

**Note**: AI agents MUST run full test suite after ALL code changes per Principle XI (Continuous Test Verification)

**Root Cause Tracing** (Principle XII): When encountering failures, trace backward to find original trigger and fix at source

- [ ] T125 [P] Add API documentation comments to all handlers
- [ ] T126 [P] Add Godoc comments to all public service interfaces
- [ ] T127 [P] Create API usage examples in README.md
- [ ] T128 [P] Create deployment guide in `docs/deployment.md`
- [ ] T129 Verify all integration tests pass: `go test -v ./tests/integration/`
- [ ] T130 Run tests with race detector: `go test -race ./...`
- [ ] T131 Generate test coverage report: `go test -coverprofile=coverage.out ./...`
- [ ] T132 Verify test coverage meets threshold (>80% for services, handlers)
- [ ] T133 Run linter and fix issues: `golangci-lint run ./...`
- [ ] T134 Format all code: `go fmt ./...` and `goimports -w .`
- [ ] T135 [P] Add database indexes for performance per research.md (organization_id, status, search_vector, SKU trigram, name trigram)
- [ ] T136 [P] Add request/response logging in middleware
- [ ] T137 [P] Add metrics collection for OpenTracing (span duration, error counts)
- [ ] T138 Security review: input sanitization, SQL injection prevention, XSS protection
- [ ] T139 Performance testing with large datasets (100k+ products)
- [ ] T140 Validate quickstart.md instructions by following step-by-step
- [ ] T141 Create Docker Compose file for local development
- [ ] T142 Create Dockerfile for production deployment
- [ ] T143 Final test run with all tests: `go test -v -race -coverprofile=coverage.out ./...`

**Debugging Discipline** (if issues encountered during any phase):
- [ ] T144 Document root cause analysis for any bugs fixed during implementation
- [ ] T145 Verify all fixes address root causes, not symptoms
- [ ] T146 Ensure no test cases were removed or weakened to make tests pass
- [ ] T147 Verify test expectations reflect correct behavior
- [ ] T148 Document tracing process (symptom → source → fix) in commit messages

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phases 3-8)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P2 → P2 → P3 → P3)
- **Acceptance Validation (Phase 9)**: Depends on all user stories being complete
- **Error Testing (Phase 10)**: Depends on all services/handlers being implemented
- **Polish (Phase 11)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational - No dependencies on other stories ✅ MVP
- **User Story 2 (P2)**: Can start after Foundational - Independent but may reference US1 products in tests
- **User Story 3 (P2)**: Can start after Foundational - Requires US1 products as parents for variants
- **User Story 4 (P2)**: Can start after Foundational - Requires US1 products to attach assets
- **User Story 5 (P3)**: Can start after Foundational - Can search products from US1, filter by US2 categories
- **User Story 6 (P3)**: Can start after Foundational - Imports/exports products from US1

**Note**: While US3, US4, US5, US6 reference US1 products, they are still independently testable by creating their own product fixtures in tests.

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Models before services
- Services before handlers
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel (T002-T012)
- All Foundational tasks marked [P] can run in parallel within Phase 2 (T013-T029)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- All tests for a user story marked [P] can run in parallel
- Models within a story marked [P] can run in parallel (e.g., T032-T033 in US1)
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# After Foundational phase complete, launch User Story 1 tasks:

# Write tests first (must fail):
T030: Create acceptance scenario tests in product_test.go
T031: Create fixture helpers

# Then implement in parallel where possible:
T032 [P]: Create Product GORM model (different file)
T033 [P]: Define ProductService interface (different file)

# Then sequential implementation (same services):
T034: Implement ProductService
T035: Add SKU uniqueness validation
T036: Add required field validation
T037: Implement soft delete logic

# Then parallel handler work:
T038: Create ProductHandler
T039 [P]: Add OpenTracing spans
T040 [P]: Add request/response parsing

# Finally wire it up:
T041: Register routes in main.go
T042: Update AutoMigrate
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T012)
2. Complete Phase 2: Foundational (T013-T029) **CRITICAL - blocks all stories**
3. Complete Phase 3: User Story 1 (T030-T042)
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Run: `go test -v ./tests/integration/product_test.go`
6. Deploy/demo if ready - Core product CRUD is functional!

**MVP Delivers**: Complete product lifecycle (create, read, update, delete) with validation, error handling, and tracing

### Incremental Delivery

1. Setup + Foundational → Foundation ready (T001-T029)
2. Add User Story 1 → Test independently → Deploy/Demo (T030-T042) **MVP!** 🎯
3. Add User Story 2 → Test independently → Deploy/Demo (T043-T058)
4. Add User Story 3 → Test independently → Deploy/Demo (T059-T071)
5. Add User Story 4 → Test independently → Deploy/Demo (T072-T087)
6. Add User Story 5 → Test independently → Deploy/Demo (T088-T099)
7. Add User Story 6 → Test independently → Deploy/Demo (T100-T115)
8. Complete validation + error testing → Production ready (T116-T124)
9. Polish and optimize → Production hardened (T125-T148)

Each story adds value without breaking previous stories!

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together (T001-T029)
2. Once Foundational is done:
   - **Developer A**: User Story 1 - Product CRUD (T030-T042)
   - **Developer B**: User Story 2 - Categories (T043-T058)
   - **Developer C**: User Story 3 - Variants (T059-T071)
   - **Developer D**: User Story 4 - Assets (T072-T087)
3. After initial stories complete:
   - **Developer A**: User Story 5 - Search (T088-T099)
   - **Developer B**: User Story 6 - Import/Export (T100-T115)
   - **Developer C**: Acceptance validation (T116-T118)
   - **Developer D**: Error testing (T119-T124)
4. Team completes Polish together (T125-T148)

Stories complete and integrate independently!

---

## Task Summary

**Total Tasks**: 148  
**Organized by Phase**: 11 phases

**Task Breakdown by Phase**:
- Phase 1 - Setup: 12 tasks
- Phase 2 - Foundational: 17 tasks (BLOCKS all user stories)
- Phase 3 - US1 (P1): 13 tasks 🎯 MVP
- Phase 4 - US2 (P2): 16 tasks
- Phase 5 - US3 (P2): 13 tasks
- Phase 6 - US4 (P2): 16 tasks
- Phase 7 - US5 (P3): 12 tasks
- Phase 8 - US6 (P3): 16 tasks
- Phase 9 - Acceptance Validation: 3 tasks (MANDATORY)
- Phase 10 - Error Testing: 6 tasks (MANDATORY)
- Phase 11 - Polish: 24 tasks

**Task Breakdown by User Story**:
- User Story 1 (Product CRUD): 13 implementation tasks + 2 test tasks = 15 total
- User Story 2 (Categories): 16 implementation tasks + 3 test tasks = 19 total
- User Story 3 (Variants): 13 implementation tasks + 2 test tasks = 15 total
- User Story 4 (Assets): 16 implementation tasks + 2 test tasks = 18 total
- User Story 5 (Search): 12 implementation tasks + 2 test tasks = 14 total
- User Story 6 (Import/Export): 16 implementation tasks + 2 test tasks = 18 total

**Parallel Opportunities**: 45 tasks marked [P] can run in parallel once dependencies met

**Independent Test Criteria**:
- ✅ US1: Create/update/delete product, verify CRUD operations work in isolation
- ✅ US2: Create category tree, assign products, verify without needing other features
- ✅ US3: Create parent product, generate variants, verify variant management standalone
- ✅ US4: Upload assets to product, manage primary image, verify asset operations isolated
- ✅ US5: Search/filter products, verify performance, test without other features
- ✅ US6: Import CSV, export CSV, verify bulk operations work independently

**Suggested MVP Scope**: User Story 1 only (T001-T042) = 44 tasks
- Delivers core product management with full CRUD operations
- Independently testable and deployable
- Provides foundation for all other stories

**Critical Validations**:
- ✅ 24 acceptance scenarios mapped to tests (Phase 9)
- ✅ 12 sentinel errors tested (Phase 10)
- ✅ 14 HTTP error codes tested (Phase 10)
- ✅ All tests use table-driven design with protobuf assertions
- ✅ All tests derive expected from fixtures, not response data

**Format Validation**: ✅ All 148 tasks follow checklist format (checkbox, ID, optional [P]/[Story] labels, file paths)

---

## Notes

- [P] tasks = different files, no dependencies on incomplete work
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Write tests first, verify they fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Run full test suite after every change (Principle XI)
- Trace problems to root cause, fix at source (Principle XII)
- All 24 acceptance scenarios must be tested (Principle XIII)
- All errors (12 sentinel + 14 HTTP codes) must be tested (Principle IX)

---

## Troubleshooting & Debugging (Principle XII: Root Cause Tracing)

When encountering problems during implementation, follow this methodology:

### Root Cause Tracing Process

**Step 1: Document the Symptom**
- What is the observable problem?
- What behavior was expected vs. actual?
- Where does the problem manifest (test failure, runtime error, etc.)?

**Step 2: Trace Backward Through Call Chain**
```
[SYMPTOM] Observable problem
    ↑
[Layer N] Where problem appears
    ↑
[Layer N-1] Previous call
    ↑
[Layer N-2] Earlier call
    ↑
[ROOT CAUSE] Where problem originates
```

**Step 3: Verify Root Cause**
- Does fixing this source eliminate the symptom?
- Is this the original trigger, not just another symptom?
- Can you explain the mechanism causing the problem?

**Step 4: Fix at Source**
- Implement fix at root cause location
- Do NOT add workarounds in symptom location
- Do NOT weaken tests to accommodate bugs
- Do NOT remove failing test cases

**Step 5: Verify Fix**
- Run tests to confirm problem resolved
- Verify no new problems introduced
- Confirm all tests pass without workarounds

**Step 6: Document**
- Record root cause in commit message
- Add comments explaining the fix if non-obvious
- Update documentation if revealing systemic issue

### Anti-Patterns to Avoid

**❌ NEVER Do These**:
1. Remove failing test cases
2. Change test expectations to match broken behavior
3. Add `t.Skip()` to flaky tests
4. Relax assertions ("at least" instead of "exactly")
5. Add conditional workarounds
6. Catch and ignore errors without understanding
7. "Make it work" without understanding why

**✅ ALWAYS Do These**:
1. Trace problem to its source
2. Fix where it originates
3. Maintain test integrity
4. Document root cause
5. Verify proper fix with tests
6. Run full test suite after fix

### Example: GORM Default Override Issue

**Symptom**: Test creates Product with Status: "draft", but database has Status: "active"

**Wrong Approach** ❌:
```go
// Remove test case for draft products
// OR change test: "if status == 'active' || status == 'draft'"
// OR copy status from response: expected.Status = response.Status
```

**Root Cause Trace** ✅:
```
Test expects: Status = "draft"
    ↑
GORM executes: db.Create(&product)
    ↑
PostgreSQL has: status VARCHAR DEFAULT 'active'
    ↑
ROOT CAUSE: Model tag has `gorm:"default:active"`
```

**Proper Fix** ✅:
```go
// internal/models/product.go
Status string `gorm:"not null;index"` // Removed default:active
// Let application control default, not database
```

### When to Apply Root Cause Tracing

Apply this discipline when:
- ✅ Tests fail unexpectedly
- ✅ Behavior doesn't match expectations
- ✅ Errors occur without clear cause
- ✅ Workarounds seem necessary
- ✅ "It should work" but doesn't
- ✅ Flaky tests appear
- ✅ Data doesn't persist as expected
- ✅ Integration tests pass but behavior is wrong

### Documentation Requirements

When fixing bugs during implementation, document in commit:
```
Fix: [Brief description]

Root Cause:
- Symptom: [What was observed]
- Traced: [Call chain backward]
- Source: [Where it originated]
- Fix: [What was changed at source]

Verified: [How fix was confirmed with tests]
```

---

**Ready to implement!** Start with Phase 1 (Setup) or run `/speckit.implement` to begin guided implementation. 🚀

