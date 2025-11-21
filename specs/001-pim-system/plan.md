# Implementation Plan: Product Information Management (PIM) System

**Branch**: `001-pim-system` | **Date**: November 21, 2025 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/001-pim-system/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

The Product Information Management (PIM) System provides a centralized platform for managing product data including core attributes, hierarchical categories, product variants, and digital assets. The system will expose a RESTful HTTP API using Protocol Buffers for type-safe contracts, leveraging PostgreSQL for data persistence with JSONB support for flexible custom attributes. The implementation follows a service-oriented architecture with comprehensive integration testing, distributed tracing, and multi-tenant isolation.

**Primary Capability**: Enable product managers to create, organize, and manage complete product information with support for variants, categories, and media assets.

**Technical Approach**: Go backend API with standard library HTTP server, GORM for database operations, OpenTracing for observability, and Protocol Buffers for API contracts. All business logic encapsulated in reusable public services with thin HTTP handlers.

## Technical Context

**Language/Version**: Go 1.21+ (latest stable recommended)  
**HTTP Framework**: Standard library `net/http` with `http.ServeMux` (MANDATORY per constitution - NO external routers)  
**Database**: PostgreSQL 15+ with JSONB support (MANDATORY per constitution)  
**Database Access**: GORM (gorm.io/gorm with gorm.io/driver/postgres) (MANDATORY per constitution)  
**Distributed Tracing**: OpenTracing (github.com/opentracing/opentracing-go) (MANDATORY per constitution)  
**Protocol Buffers**: protoc compiler, protoc-gen-go for API contracts (MANDATORY per constitution)  
**Testing**: Standard library `testing` with `httptest`, testcontainers-go for PostgreSQL (MANDATORY per constitution)  
**Test Comparison**: google/go-cmp with protocmp for protobuf assertions (MANDATORY per constitution)  
**Error Handling**: Standard library fmt.Errorf with %w for wrapping, errors.Is/As for checking (MANDATORY per constitution)  
**Error Testing**: ALL sentinel errors and HTTP error codes MUST be tested (MANDATORY per constitution Principle IX)  
**Context Propagation**: All service methods MUST accept context.Context as first parameter (MANDATORY per constitution)  
**Service Architecture**: Services in public `services/` package (NOT internal/) for external reusability (MANDATORY per constitution Principle VIII)  
**Target Platform**: Linux server with containerized deployment (Docker)  
**Project Type**: Backend API service (Go)  
**Performance Goals**: Support 1000 concurrent requests/sec with p99 < 200ms for read operations, p99 < 500ms for write operations  
**Constraints**: Response time p95 < 150ms for product retrieval, < 100MB memory per request, asset upload < 10MB (images) / 100MB (videos)  
**Scale/Scope**: Support 100,000+ products per organization, 50 concurrent users, 500k API requests per day

**Additional Technical Context**:
- **Asset Storage**: Local filesystem storage initially (with path stored in database), future extensibility for S3/cloud storage
- **Image Processing**: No server-side image processing in MVP (clients upload pre-sized images), future enhancement for thumbnails
- **Search**: PostgreSQL full-text search using tsvector/tsquery, trigram indexes for partial matching
- **Import/Export**: CSV format with standard column mappings, synchronous processing for files < 1000 rows, async with progress tracking for larger files
- **Multi-tenancy**: Organization-level isolation using organization_id foreign key on all entities
- **Authentication**: JWT bearer tokens (implementation details deferred to auth service integration)
- **Authorization**: Role-based access control (RBAC) with admin/editor/viewer roles

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

✅ **Principle I - Integration Testing First**: All tests will use real PostgreSQL via testcontainers-go. No mocking of database or HTTP clients. Fixture data prepared directly in test database.

✅ **Principle II - Table-Driven Test Design**: All test functions will use table-driven patterns with descriptive test case names and shared setup/teardown helpers.

✅ **Principle III - Edge Case Coverage**: All 24 acceptance scenarios will have corresponding integration tests covering input validation, boundary conditions, auth/authz, data state, database errors, and HTTP specifics as documented in spec.md edge cases section.

✅ **Principle IV - Real Database Fixtures**: Test fixtures will be created using GORM operations against real test database. Helper functions in `testutil/fixtures.go` will handle complex fixture creation.

✅ **Principle V - ServeHTTP Endpoint Testing**: All endpoint tests will use `httptest.ResponseRecorder` and pass requests through full HTTP handler chains including middleware.

✅ **Principle VI - Protobuf Data Structures**: All API contracts defined in `.proto` files. Tests will use `protocmp.Transform()` with `cmp.Diff()` for message assertions. Expected values derived from test fixtures, only copying truly random fields (UUIDs, timestamps).

✅ **Principle VII - Distributed Tracing**: All HTTP endpoints will create OpenTracing spans with operation names. Service operations will create child spans. Database transactions traced as single child spans (not per-query).

✅ **Principle VIII - Service Layer Architecture**: Business logic in public `services/` package with interface-based dependency injection. HTTP handlers will be thin wrappers calling service methods.

✅ **Principle IX - Comprehensive Error Handling**: Two-layer error strategy: sentinel errors in `services/errors.go` for internal flow, HTTP error codes in `handlers/error_codes.go` for client responses. Dedicated `tests/integration/error_handling_test.go` with `TestAllSentinelErrors` and `TestAllHTTPErrorCodes` functions.

✅ **Principle X - Context-Aware Operations**: All service methods accept `context.Context` as first parameter. All database operations use `db.WithContext(ctx)`. Context cancellation and timeouts tested.

✅ **Principle XI - Continuous Test Verification**: All code changes will be verified with `go test ./...` before committing. Integration tests run locally via testcontainers.

✅ **Principle XII - Root Cause Tracing**: All debugging will trace problems backward through call chain to original trigger. Fixes address root causes, not symptoms.

✅ **Principle XIII - Acceptance Scenario Coverage**: All 24 acceptance scenarios (US1-AS1 through US6-AS4) will have corresponding integration tests with matching test case names.

**Gate Status**: ✅ PASS - All constitutional principles will be followed. No violations requiring justification.

## Project Structure

### Documentation (this feature)

```text
specs/001-pim-system/
├── spec.md              # Feature specification
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   ├── product.proto
│   ├── category.proto
│   ├── asset.proto
│   └── common.proto
├── checklists/
│   └── requirements.md  # Specification quality checklist
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
pim-demo/
├── .specify/              # Spec-kit configuration
│   ├── memory/
│   │   ├── constitution.md
│   │   └── cursor-agent.md
│   ├── scripts/
│   │   └── bash/
│   └── templates/
├── api/                   # PUBLIC - protobuf definitions
│   └── v1/
│       ├── product.proto
│       ├── category.proto
│       ├── variant.proto
│       ├── asset.proto
│       ├── common.proto
│       └── errors.proto
├── api/gen/v1/            # Generated protobuf code
│   ├── product.pb.go
│   ├── category.pb.go
│   ├── variant.pb.go
│   ├── asset.pb.go
│   ├── common.pb.go
│   └── errors.pb.go
├── services/              # PUBLIC - reusable business logic
│   ├── product_service.go
│   ├── category_service.go
│   ├── variant_service.go
│   ├── asset_service.go
│   ├── import_service.go
│   ├── search_service.go
│   ├── errors.go          # Sentinel errors (ErrProductNotFound, etc.)
│   └── migrations.go      # AutoMigrate for external apps
├── handlers/              # PUBLIC - HTTP handlers
│   ├── product_handler.go
│   ├── category_handler.go
│   ├── variant_handler.go
│   ├── asset_handler.go
│   ├── import_handler.go
│   ├── search_handler.go
│   ├── error_codes.go     # HTTP error code mappings
│   └── response.go        # Standard response helpers
├── internal/              # INTERNAL - implementation details
│   ├── models/            # GORM models (not exposed)
│   │   ├── product.go
│   │   ├── category.go
│   │   ├── variant.go
│   │   ├── asset.go
│   │   └── organization.go
│   ├── middleware/        # HTTP middleware
│   │   ├── auth.go
│   │   ├── tracing.go
│   │   ├── logging.go
│   │   └── tenant.go
│   ├── config/            # Configuration
│   │   └── config.go
│   └── storage/           # Asset storage abstraction
│       └── filesystem.go
├── cmd/                   # Application entry points
│   └── api/
│       └── main.go
├── tests/
│   ├── integration/       # Integration tests
│   │   ├── product_test.go
│   │   ├── category_test.go
│   │   ├── variant_test.go
│   │   ├── asset_test.go
│   │   ├── import_test.go
│   │   ├── search_test.go
│   │   └── error_handling_test.go  # MANDATORY error tests
│   └── testutil/          # Test utilities
│       ├── database.go    # Database setup/teardown
│       ├── fixtures.go    # Fixture creation helpers
│       └── http.go        # HTTP test helpers
├── specs/                 # Feature specifications
│   └── 001-pim-system/    # This feature
├── go.mod
├── go.sum
├── Makefile               # Build and test targets
├── Dockerfile
└── README.md
```

**Structure Decisions**:
- **API versioning**: Using `api/v1/` directory structure for future API evolution without breaking changes
- **Public services package**: Business logic in top-level `services/` (not `internal/services/`) to enable external applications to import and reuse
- **Separate handlers per domain**: Each major entity (product, category, variant, asset) has dedicated handler and service files for maintainability
- **GORM models in internal**: Database models kept internal since external consumers should use protobuf types only
- **Testutil package**: Shared test utilities for database setup and fixture creation to reduce test code duplication
- **Middleware package**: Cross-cutting concerns (auth, tracing, logging) centralized in middleware for consistent application

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

*No violations - this section is empty.*

---

## Phase 0: Research & Decision Documentation

**Status**: ✅ Complete (see [research.md](./research.md))

**Research Topics**:
1. PostgreSQL full-text search capabilities and performance at 100k+ products
2. GORM JSONB handling for flexible custom attributes
3. Asset storage patterns (filesystem vs cloud)
4. CSV import/export libraries and large file handling
5. Multi-tenant data isolation patterns in PostgreSQL
6. OpenTracing span granularity best practices

**Outcome**: All technical unknowns resolved with documented decisions and rationale in research.md

---

## Phase 1: Data Model & API Contracts

**Status**: ✅ Complete

**Artifacts**:
- [data-model.md](./data-model.md) - Entity relationships and validation rules
- [contracts/](./contracts/) - Protocol Buffer API definitions
- [quickstart.md](./quickstart.md) - Developer onboarding guide

**Data Model Summary**:
- **Core entities**: Product, Category, ProductVariant, Asset, Organization
- **Relationships**: Many-to-many (Product-Category), One-to-many (Product-Variant, Product-Asset)
- **Custom attributes**: JSONB column on Product for organization-defined attributes
- **Multi-tenancy**: organization_id foreign key on all entities with unique constraints scoped to organization

**API Contracts Summary**:
- **Product API**: CRUD operations, search, category assignment
- **Category API**: CRUD operations, hierarchy management
- **Variant API**: Define variants, generate variant products
- **Asset API**: Upload, associate with products, manage primary image
- **Import/Export API**: CSV upload/download with progress tracking

---

## Phase 2: Implementation Tasks

**Status**: ⏸️ Pending - Use `/speckit.tasks` command to generate detailed implementation tasks

**Task Breakdown Preview**:
1. Database schema and migrations
2. GORM models and relationships
3. Service layer implementation (business logic)
4. HTTP handlers and routing
5. Middleware (auth, tracing, tenant isolation)
6. Integration tests for all acceptance scenarios
7. Error handling tests (TestAllSentinelErrors, TestAllHTTPErrorCodes)
8. Asset storage implementation
9. CSV import/export functionality
10. Full-text search implementation

---

## Next Steps

1. ✅ Specification complete ([spec.md](./spec.md))
2. ✅ Implementation plan complete (this file)
3. ✅ Research complete ([research.md](./research.md))
4. ✅ Data model complete ([data-model.md](./data-model.md))
5. ✅ API contracts complete ([contracts/](./contracts/))
6. ⏭️ **Run `/speckit.tasks`** to break down implementation into actionable tasks
7. ⏭️ Begin implementation following generated tasks
8. ⏭️ Run tests continuously: `go test ./...`
9. ⏭️ Review and merge feature branch after all tests pass
