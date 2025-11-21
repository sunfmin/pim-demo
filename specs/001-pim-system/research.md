# Research & Technical Decisions: PIM System

**Created**: November 21, 2025  
**Feature**: [spec.md](./spec.md)  
**Purpose**: Document research findings and technical decisions for implementation

---

## 1. PostgreSQL Full-Text Search

### Decision
Use PostgreSQL's built-in full-text search with `tsvector` and `tsquery` for product search functionality.

### Rationale
- **Performance**: PostgreSQL full-text search handles 100k+ products efficiently with proper indexing
- **No external dependencies**: Eliminates need for Elasticsearch or similar, reducing operational complexity
- **ACID guarantees**: Search results always consistent with database state (no sync lag issues)
- **Built-in relevance ranking**: `ts_rank()` function provides relevance scoring
- **Trigram support**: `pg_trgm` extension enables partial/fuzzy matching for SKU and name searches

### Implementation Approach
```sql
-- Add tsvector column to products table
ALTER TABLE products ADD COLUMN search_vector tsvector;

-- Create GIN index for fast full-text search
CREATE INDEX idx_products_search ON products USING GIN(search_vector);

-- Create trigram index for partial matching
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_products_sku_trgm ON products USING GIN(sku gin_trgm_ops);
CREATE INDEX idx_products_name_trgm ON products USING GIN(name gin_trgm_ops);

-- Update trigger to maintain search_vector
CREATE TRIGGER products_search_vector_update BEFORE INSERT OR UPDATE
ON products FOR EACH ROW EXECUTE FUNCTION
tsvector_update_trigger(search_vector, 'pg_catalog.english', name, sku, description);
```

### Performance Expectations
- Search queries: < 50ms for 100k products
- Scales to 1M+ products with proper indexing
- Memory efficient (indexes stored in shared buffers)

### Alternatives Considered
- **Elasticsearch**: Rejected due to operational complexity, eventual consistency issues, and infrastructure requirements
- **SQLite FTS5**: Rejected as database is already PostgreSQL per constitution
- **Simple LIKE queries**: Rejected due to poor performance at scale (requires full table scans)

---

## 2. GORM JSONB Handling for Custom Attributes

### Decision
Use PostgreSQL JSONB column type with GORM's `datatypes.JSON` for flexible custom product attributes.

### Rationale
- **Flexibility**: Organizations can define custom attributes without schema migrations
- **Performance**: JSONB provides indexing and query capabilities (unlike plain JSON)
- **Type safety**: GORM's `datatypes.JSON` provides Go struct mapping
- **Queryability**: Can filter and search within JSONB using PostgreSQL operators

### Implementation Approach
```go
// GORM Model
type Product struct {
    ID          uuid.UUID       `gorm:"type:uuid;primary_key"`
    Name        string          `gorm:"not null"`
    Attributes  datatypes.JSON  `gorm:"type:jsonb;default:'{}'"`
}

// Usage in code
attributes := map[string]interface{}{
    "color": "blue",
    "size": "large",
    "material": "cotton",
}
product.Attributes = datatypes.JSON(attributes)

// Querying JSONB
db.Where("attributes->>'color' = ?", "blue").Find(&products)
db.Where("attributes @> ?", `{"size":"large"}`).Find(&products)
```

### JSONB Indexing
```sql
-- GIN index for contains operations
CREATE INDEX idx_products_attributes ON products USING GIN(attributes);

-- Specific attribute indexes if needed
CREATE INDEX idx_products_color ON products ((attributes->>'color'));
```

### Validation Strategy
- Define attribute schemas per organization in `attribute_definitions` table
- Validate attribute values in service layer before saving
- Store validation rules in database for runtime checking

### Alternatives Considered
- **EAV (Entity-Attribute-Value) pattern**: Rejected due to query complexity and poor performance
- **Separate attributes table**: Rejected due to N+1 query issues and increased join complexity
- **Fixed schema with many nullable columns**: Rejected due to lack of flexibility

---

## 3. Asset Storage Pattern

### Decision
**MVP**: Local filesystem storage with paths stored in database  
**Future**: Pluggable storage interface supporting S3/cloud storage

### Rationale
- **Simplicity for MVP**: Filesystem storage requires no external services or SDKs
- **Performance**: Local disk access faster than network storage for small deployments
- **Cost**: Zero storage costs for initial deployment
- **Extensibility**: Storage interface allows future migration to S3 without API changes

### Implementation Approach
```go
// Storage interface (internal/storage/storage.go)
type Storage interface {
    Save(ctx context.Context, file io.Reader, metadata FileMetadata) (string, error)
    Get(ctx context.Context, path string) (io.ReadCloser, error)
    Delete(ctx context.Context, path string) error
}

// Filesystem implementation (MVP)
type FilesystemStorage struct {
    basePath string
}

// Future S3 implementation
type S3Storage struct {
    bucket string
    client *s3.Client
}
```

### File Organization
```
/var/pim/assets/
├── {organization_id}/
│   └── {product_id}/
│       ├── {asset_id}.jpg
│       ├── {asset_id}.png
│       └── {asset_id}.mp4
```

### Database Schema
```sql
CREATE TABLE assets (
    id UUID PRIMARY KEY,
    product_id UUID NOT NULL REFERENCES products(id),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    file_name TEXT NOT NULL,
    storage_path TEXT NOT NULL,  -- e.g., "{org_id}/{product_id}/{asset_id}.jpg"
    content_type TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    is_primary BOOLEAN DEFAULT false,
    created_at TIMESTAMP NOT NULL
);
```

### Security Considerations
- No direct file system access via HTTP (prevent directory traversal)
- Assets served through API endpoint that validates permissions
- Signed URLs for temporary direct access (future enhancement)

### Alternatives Considered
- **S3 from start**: Rejected for MVP due to complexity and cost
- **Database BLOB storage**: Rejected due to poor performance and database bloat
- **CDN integration**: Deferred to post-MVP for caching and delivery optimization

---

## 4. CSV Import/Export Libraries

### Decision
Use Go standard library `encoding/csv` for CSV processing. No external libraries needed.

### Rationale
- **Standard library**: No dependencies, well-tested, maintained by Go team
- **Sufficient functionality**: Handles CSV parsing, quoting, escaping
- **Streaming support**: Can process large files without loading entirely into memory
- **UTF-8 support**: Handles international characters correctly

### Implementation Approach

**Import Process**:
```go
// Synchronous for small files (< 1000 rows)
func ImportProducts(ctx context.Context, file io.Reader) (*ImportResult, error) {
    reader := csv.NewReader(file)
    reader.LazyQuotes = true
    reader.TrimLeadingSpace = true
    
    // Read header
    headers, err := reader.Read()
    
    // Process rows
    for {
        row, err := reader.Read()
        if err == io.EOF {
            break
        }
        // Validate and import row
    }
}

// Asynchronous for large files (1000+ rows)
func ImportProductsAsync(ctx context.Context, file io.Reader) (string, error) {
    jobID := uuid.New()
    go processImportJob(ctx, jobID, file)
    return jobID, nil
}
```

**Export Process**:
```go
func ExportProducts(ctx context.Context, filters Filters, w io.Writer) error {
    writer := csv.NewWriter(w)
    defer writer.Flush()
    
    // Write header
    writer.Write([]string{"SKU", "Name", "Description", "Price", "Category"})
    
    // Stream products in batches
    offset := 0
    limit := 1000
    for {
        products := fetchProducts(ctx, filters, offset, limit)
        for _, product := range products {
            writer.Write(productToRow(product))
        }
        if len(products) < limit {
            break
        }
        offset += limit
    }
}
```

### Column Mapping
**Standard columns** (required):
- `sku`: Product SKU (unique identifier)
- `name`: Product name
- `description`: Product description
- `price`: Base price (decimal)
- `status`: Product status (draft/active/discontinued)

**Optional columns**:
- `category`: Category name or ID
- `category_path`: Full category hierarchy (e.g., "Electronics > Computers > Laptops")
- Custom attribute columns: `attr_color`, `attr_size`, etc.

### Error Handling
- Collect all validation errors during import
- Return detailed report: row number, field, error message
- Rollback transaction if any row fails (synchronous import)
- Partial success allowed for async imports with error report

### Alternatives Considered
- **github.com/gocarina/gocsv**: Rejected as standard library sufficient
- **Excel format support**: Deferred to post-MVP (can use libraries like excelize if needed)
- **JSON import**: Deferred to post-MVP

---

## 5. Multi-Tenant Data Isolation

### Decision
Use row-level filtering with `organization_id` foreign key on all tables. Enforce isolation in service layer and database constraints.

### Rationale
- **Simplicity**: Single database, single schema for all tenants
- **Performance**: Efficient with proper indexing on organization_id
- **Cost-effective**: No need for per-tenant databases
- **Scalability**: Handles hundreds of organizations without architectural changes

### Implementation Approach

**Database Schema**:
```sql
CREATE TABLE organizations (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE products (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    sku TEXT NOT NULL,
    name TEXT NOT NULL,
    UNIQUE(organization_id, sku)  -- SKU unique within organization
);

CREATE INDEX idx_products_org ON products(organization_id);
```

**Service Layer Enforcement**:
```go
type ProductService struct {
    db *gorm.DB
}

func (s *ProductService) GetProduct(ctx context.Context, orgID, productID uuid.UUID) (*models.Product, error) {
    var product models.Product
    err := s.db.WithContext(ctx).
        Where("id = ? AND organization_id = ?", productID, orgID).
        First(&product).Error
    if err != nil {
        return nil, err
    }
    return &product, nil
}
```

**Middleware Enforcement**:
```go
// Extract organization from JWT claims
func TenantMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        orgID := extractOrgFromToken(r)
        ctx := context.WithValue(r.Context(), "organization_id", orgID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Security Considerations
- **Defense in depth**: Check organization_id at service, handler, and database levels
- **Unique constraints scoped**: All unique constraints include organization_id
- **Foreign keys**: Ensure cross-organization references impossible
- **Audit logging**: Log all access with organization context

### Performance Optimization
- Index all foreign key columns including organization_id
- Partition tables by organization_id if single tenant dominates (future)
- Connection pooling per organization (future enhancement)

### Alternatives Considered
- **Database per tenant**: Rejected due to operational complexity and migration difficulties
- **Schema per tenant**: Rejected due to PostgreSQL schema limitations and connection overhead
- **Shared tables with RLS (Row Level Security)**: Considered for future enhancement, but service-layer filtering sufficient for MVP

---

## 6. OpenTracing Span Granularity

### Decision
Create spans at HTTP endpoint level and service method level. Database operations traced as single span per transaction.

### Rationale
- **Observability without noise**: Too many spans (per SQL query) creates overwhelming trace data
- **Performance boundaries**: Span at each architectural layer (HTTP → Service → Database)
- **Debugging effectiveness**: Can identify which service method is slow without per-query noise
- **Cost efficiency**: Fewer spans = lower tracing infrastructure costs

### Span Hierarchy
```
POST /api/v1/products
├── ProductHandler.Create (HTTP layer)
├── ProductService.Create (Service layer)
│   ├── ValidationService.ValidateProduct (if complex validation)
│   └── Database Transaction (single span for all SQL)
└── CategoryService.ValidateCategories (if categories assigned)
    └── Database Query (single span)
```

### Implementation Approach
```go
// HTTP Handler
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
    span, ctx := opentracing.StartSpanFromContext(r.Context(), "POST /api/v1/products")
    defer span.Finish()
    
    // Extract request
    var req pb.CreateProductRequest
    
    // Call service
    product, err := h.productService.Create(ctx, &req)
    
    // Return response
}

// Service Layer
func (s *ProductService) Create(ctx context.Context, req *pb.CreateProductRequest) (*models.Product, error) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "ProductService.Create")
    defer span.Finish()
    
    // Database operation (single span for entire transaction)
    dbSpan, dbCtx := opentracing.StartSpanFromContext(ctx, "DB: Create Product")
    defer dbSpan.Finish()
    
    err := s.db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
        // All SQL queries within this transaction are NOT individually traced
        return tx.Create(&product).Error
    })
    
    return product, err
}
```

### Span Tags
```go
span.SetTag("http.method", r.Method)
span.SetTag("http.url", r.URL.Path)
span.SetTag("http.status_code", statusCode)
span.SetTag("organization.id", orgID)
span.SetTag("product.id", productID)
if err != nil {
    span.SetTag("error", true)
    span.LogKV("error.message", err.Error())
}
```

### Alternatives Considered
- **Per-query tracing**: Rejected due to noise and performance overhead
- **No database tracing**: Rejected as loses visibility into slow database operations
- **Only HTTP-level tracing**: Rejected as doesn't identify slow service methods

---

## Summary of Key Decisions

| Area | Decision | Key Benefit |
|------|----------|-------------|
| Search | PostgreSQL full-text search | No external dependencies, ACID guarantees |
| Custom Attributes | JSONB with GORM datatypes | Flexibility without schema migrations |
| Asset Storage | Filesystem (MVP), interface for future S3 | Simple start, extensible future |
| CSV Processing | Standard library `encoding/csv` | No dependencies, streaming support |
| Multi-tenancy | organization_id with row filtering | Simple, performant, cost-effective |
| Tracing | Endpoint + Service + DB spans | Observability without noise |

---

## Implementation Readiness

✅ All technical unknowns resolved  
✅ All decisions documented with rationale  
✅ Implementation approaches defined  
✅ Performance expectations established  
✅ Security considerations addressed  

**Status**: Ready for Phase 1 (Data Model & Contracts)

