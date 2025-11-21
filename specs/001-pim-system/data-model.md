# Data Model: PIM System

**Created**: November 21, 2025  
**Feature**: [spec.md](./spec.md)  
**Purpose**: Define entities, relationships, and validation rules

---

## Entity Relationship Diagram

```
┌─────────────────┐
│  Organization   │
└────────┬────────┘
         │ 1
         │
         │ *
    ┌────┴──────┬──────────┬──────────┐
    │           │          │          │
┌───▼────────┐ │      ┌───▼──────┐  │
│  Product   │ │      │ Category │  │
└───┬────────┘ │      └───┬──────┘  │
    │ 1        │          │ 1       │
    │          │          │         │
    │ *        │          │ *       │
┌───▼─────────┐│      ┌───▼─────────┤
│   Variant   ││      │  Product    │
└─────────────┘│      │  Category   │
               │      │  (join)     │
           ┌───▼──────▼──────┐      │
           │      Asset      │      │
           └─────────────────┘      │
                                    │
           ┌────────────────────────▼──┐
           │   Attribute Definition    │
           └───────────────────────────┘
```

---

## Core Entities

### 1. Organization

Multi-tenant container for all PIM data.

**Fields**:
- `id` (UUID, PK): Unique organization identifier
- `name` (string, required): Organization name
- `slug` (string, unique): URL-friendly identifier
- `settings` (JSONB): Organization-wide settings
- `created_at` (timestamp): Creation time
- `updated_at` (timestamp): Last modification time

**Validation Rules**:
- Name: 1-255 characters
- Slug: lowercase alphanumeric with hyphens, 3-63 characters, unique
- Settings: Valid JSON, max 10KB

**Indexes**:
- Primary key on `id`
- Unique index on `slug`

**Business Rules**:
- Organizations cannot be deleted if they have products
- Slug is immutable after creation

---

### 2. Product

Core entity representing a sellable item.

**Fields**:
- `id` (UUID, PK): Unique product identifier
- `organization_id` (UUID, FK, required): Owner organization
- `sku` (string, required): Stock Keeping Unit
- `name` (string, required): Product name
- `description` (text): Product description
- `base_price` (decimal): Base price in cents (e.g., $19.99 = 1999)
- `status` (enum): Product status (draft, active, discontinued)
- `attributes` (JSONB): Custom attributes defined by organization
- `search_vector` (tsvector): Full-text search index
- `created_at` (timestamp): Creation time
- `updated_at` (timestamp): Last modification time
- `deleted_at` (timestamp, nullable): Soft delete timestamp

**Validation Rules**:
- SKU: Alphanumeric with hyphens/underscores, 1-50 characters, unique per organization
- Name: 1-500 characters
- Description: 0-5000 characters
- Base price: >= 0, max 999999999 (9 digits)
- Status: One of [draft, active, discontinued]
- Attributes: Valid JSON, max 50KB

**Indexes**:
- Primary key on `id`
- Unique index on `(organization_id, sku)` WHERE deleted_at IS NULL
- Index on `organization_id`
- Index on `status`
- GIN index on `search_vector`
- GIN index on `attributes`
- Trigram index on `sku` and `name`

**Business Rules**:
- SKU must be unique within organization (excluding soft-deleted products)
- Status transitions: draft → active, active → discontinued, draft → discontinued
- Cannot change SKU after product has orders (future constraint)
- Soft delete: Set deleted_at instead of physical deletion

**State Transitions**:
```
draft ──────────────> active ──────────> discontinued
  │                                            ▲
  └────────────────────────────────────────────┘
```

---

### 3. Category

Hierarchical organizational structure for products.

**Fields**:
- `id` (UUID, PK): Unique category identifier
- `organization_id` (UUID, FK, required): Owner organization
- `parent_id` (UUID, FK, nullable): Parent category (null for root)
- `name` (string, required): Category name
- `slug` (string, required): URL-friendly identifier
- `description` (text): Category description
- `display_order` (integer): Sort order within parent
- `is_active` (boolean): Whether category is visible
- `created_at` (timestamp): Creation time
- `updated_at` (timestamp): Last modification time

**Validation Rules**:
- Name: 1-255 characters
- Slug: lowercase alphanumeric with hyphens, unique within organization
- Description: 0-2000 characters
- Display order: 0-999999
- Parent must belong to same organization
- Circular references not allowed (no category can be ancestor of itself)

**Indexes**:
- Primary key on `id`
- Unique index on `(organization_id, slug)`
- Index on `organization_id`
- Index on `parent_id`
- Index on `display_order`

**Business Rules**:
- Maximum hierarchy depth: 10 levels
- Cannot delete category with products (must reassign or delete products first)
- Cannot delete category with sub-categories (must delete/move sub-categories first)
- Root categories have parent_id = NULL

**Hierarchy Example**:
```
Electronics (parent_id: NULL)
├── Computers (parent_id: Electronics)
│   ├── Laptops (parent_id: Computers)
│   └── Desktops (parent_id: Computers)
└── Phones (parent_id: Electronics)
```

---

### 4. ProductCategory (Join Table)

Many-to-many relationship between products and categories.

**Fields**:
- `id` (UUID, PK): Unique identifier
- `product_id` (UUID, FK, required): Product reference
- `category_id` (UUID, FK, required): Category reference
- `created_at` (timestamp): Assignment time

**Validation Rules**:
- Product and category must exist
- Product and category must belong to same organization
- No duplicate assignments (product-category pair must be unique)

**Indexes**:
- Primary key on `id`
- Unique index on `(product_id, category_id)`
- Index on `product_id`
- Index on `category_id`

**Business Rules**:
- Products can belong to multiple categories
- Deleting product removes all category assignments
- Deleting category requires handling products (reassign or remove)

---

### 5. ProductVariant

Specific configuration of a parent product (e.g., size, color combinations).

**Fields**:
- `id` (UUID, PK): Unique variant identifier
- `organization_id` (UUID, FK, required): Owner organization
- `parent_product_id` (UUID, FK, required): Parent product reference
- `variant_sku` (string, required): Variant SKU (unique)
- `variant_attributes` (JSONB, required): Variant-specific attributes (e.g., {size: "L", color: "Blue"})
- `price_adjustment` (decimal): Price difference from parent (can be negative)
- `inventory_quantity` (integer): Available stock
- `is_active` (boolean): Whether variant is available
- `created_at` (timestamp): Creation time
- `updated_at` (timestamp): Last modification time

**Validation Rules**:
- Variant SKU: Alphanumeric with hyphens, 1-50 characters, unique per organization
- Variant attributes: Valid JSON, required, 1-20 attributes, max 5KB
- Price adjustment: -999999999 to 999999999
- Inventory quantity: >= 0
- Parent product must exist and belong to same organization

**Indexes**:
- Primary key on `id`
- Unique index on `(organization_id, variant_sku)`
- Index on `parent_product_id`
- Index on `organization_id`
- GIN index on `variant_attributes`

**Business Rules**:
- Variant SKU must be unique across all products (not just within parent)
- Final variant price = parent base_price + price_adjustment
- Variants inherit non-variant attributes from parent
- Cannot create variant without parent product
- Deleting parent product deletes all variants

**Example**:
```
Parent Product: T-Shirt (SKU: TSHIRT-001, Price: 1999)
Variants:
  - TSHIRT-001-S-RED: size=S, color=Red, price_adjustment=0 → $19.99
  - TSHIRT-001-L-BLUE: size=L, color=Blue, price_adjustment=200 → $21.99
```

---

### 6. Asset

Digital files (images, videos, documents) associated with products.

**Fields**:
- `id` (UUID, PK): Unique asset identifier
- `organization_id` (UUID, FK, required): Owner organization
- `product_id` (UUID, FK, required): Product reference
- `file_name` (string, required): Original filename
- `storage_path` (string, required): Path in storage system
- `content_type` (string, required): MIME type
- `file_size` (bigint, required): File size in bytes
- `asset_type` (enum, required): Type of asset (image, video, document)
- `is_primary` (boolean): Whether this is the primary product image
- `display_order` (integer): Sort order for display
- `alt_text` (string): Alternative text for accessibility
- `created_at` (timestamp): Upload time

**Validation Rules**:
- File name: 1-255 characters
- Storage path: 1-1000 characters, unique
- Content type: Valid MIME type
- File size: > 0, max 104857600 (100MB)
- Asset type: One of [image, video, document]
- Supported image types: image/jpeg, image/png, image/webp, image/gif
- Supported video types: video/mp4, video/webm
- Supported document types: application/pdf
- Alt text: 0-500 characters
- Display order: 0-999999
- Only one primary image per product

**Indexes**:
- Primary key on `id`
- Unique index on `storage_path`
- Index on `product_id`
- Index on `organization_id`
- Index on `is_primary`
- Index on `display_order`

**Business Rules**:
- Maximum 50 assets per product
- Only one asset can be marked as primary per product
- Setting new primary asset automatically unsets previous primary
- Deleting product deletes all associated assets and files
- File size limits: 10MB for images, 100MB for videos, 10MB for documents

**Storage Path Format**:
```
{organization_id}/{product_id}/{asset_id}.{extension}
Example: 123e4567-e89b-12d3-a456-426614174000/789abc12-e34f-56g7-h890-123456789012/asset123.jpg
```

---

### 7. AttributeDefinition

Defines custom attributes that can be used by products within an organization.

**Fields**:
- `id` (UUID, PK): Unique attribute definition identifier
- `organization_id` (UUID, FK, required): Owner organization
- `attribute_name` (string, required): Attribute name (e.g., "color", "size")
- `attribute_key` (string, required): Key used in JSONB (e.g., "color", "size")
- `data_type` (enum, required): Data type (string, number, boolean, date)
- `is_required` (boolean): Whether attribute is required for products
- `validation_rules` (JSONB): Validation rules (e.g., regex, min/max, enum values)
- `display_order` (integer): Sort order in UI
- `created_at` (timestamp): Creation time
- `updated_at` (timestamp): Last modification time

**Validation Rules**:
- Attribute name: 1-100 characters
- Attribute key: lowercase alphanumeric with underscores, 1-50 characters, unique per organization
- Data type: One of [string, number, boolean, date]
- Validation rules: Valid JSON, max 5KB
- Display order: 0-999999

**Indexes**:
- Primary key on `id`
- Unique index on `(organization_id, attribute_key)`
- Index on `organization_id`

**Business Rules**:
- Attribute keys must be unique within organization
- Cannot delete attribute definition if products use it (soft delete or migration required)
- Validation rules enforced at service layer

**Validation Rules Examples**:
```json
{
  "string": {
    "min_length": 1,
    "max_length": 100,
    "pattern": "^[A-Za-z0-9 ]+$",
    "enum": ["Red", "Blue", "Green"]
  },
  "number": {
    "min": 0,
    "max": 1000,
    "integer_only": true
  },
  "date": {
    "min": "2020-01-01",
    "max": "2030-12-31"
  }
}
```

---

## Relationships Summary

| Relationship | Type | Description |
|--------------|------|-------------|
| Organization → Product | One-to-Many | Each product belongs to one organization |
| Organization → Category | One-to-Many | Each category belongs to one organization |
| Organization → Asset | One-to-Many | Each asset belongs to one organization |
| Product → ProductCategory ← Category | Many-to-Many | Products can be in multiple categories |
| Product → ProductVariant | One-to-Many | Product can have multiple variants |
| Product → Asset | One-to-Many | Product can have multiple assets |
| Category → Category (self) | Hierarchical | Categories can have parent/child relationships |
| Organization → AttributeDefinition | One-to-Many | Custom attributes defined per organization |

---

## Database Constraints

### Foreign Key Constraints
```sql
-- Product
ALTER TABLE products ADD CONSTRAINT fk_products_organization 
  FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT;

-- Category
ALTER TABLE categories ADD CONSTRAINT fk_categories_organization 
  FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT;
ALTER TABLE categories ADD CONSTRAINT fk_categories_parent 
  FOREIGN KEY (parent_id) REFERENCES categories(id) ON DELETE RESTRICT;

-- ProductCategory
ALTER TABLE product_categories ADD CONSTRAINT fk_product_categories_product 
  FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE;
ALTER TABLE product_categories ADD CONSTRAINT fk_product_categories_category 
  FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE RESTRICT;

-- ProductVariant
ALTER TABLE product_variants ADD CONSTRAINT fk_variants_organization 
  FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT;
ALTER TABLE product_variants ADD CONSTRAINT fk_variants_parent 
  FOREIGN KEY (parent_product_id) REFERENCES products(id) ON DELETE CASCADE;

-- Asset
ALTER TABLE assets ADD CONSTRAINT fk_assets_organization 
  FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT;
ALTER TABLE assets ADD CONSTRAINT fk_assets_product 
  FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE;

-- AttributeDefinition
ALTER TABLE attribute_definitions ADD CONSTRAINT fk_attributes_organization 
  FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT;
```

### Check Constraints
```sql
-- Product
ALTER TABLE products ADD CONSTRAINT chk_product_base_price 
  CHECK (base_price >= 0 AND base_price < 1000000000);
ALTER TABLE products ADD CONSTRAINT chk_product_status 
  CHECK (status IN ('draft', 'active', 'discontinued'));

-- ProductVariant
ALTER TABLE product_variants ADD CONSTRAINT chk_variant_inventory 
  CHECK (inventory_quantity >= 0);
ALTER TABLE product_variants ADD CONSTRAINT chk_variant_price_adjustment 
  CHECK (price_adjustment > -1000000000 AND price_adjustment < 1000000000);

-- Asset
ALTER TABLE assets ADD CONSTRAINT chk_asset_file_size 
  CHECK (file_size > 0 AND file_size <= 104857600);
ALTER TABLE assets ADD CONSTRAINT chk_asset_type 
  CHECK (asset_type IN ('image', 'video', 'document'));

-- AttributeDefinition
ALTER TABLE attribute_definitions ADD CONSTRAINT chk_attribute_data_type 
  CHECK (data_type IN ('string', 'number', 'boolean', 'date'));
```

---

## GORM Model Examples

```go
// Organization
type Organization struct {
    ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    Name      string         `gorm:"not null;size:255"`
    Slug      string         `gorm:"uniqueIndex;not null;size:63"`
    Settings  datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
    CreatedAt time.Time      `gorm:"not null;default:now()"`
    UpdatedAt time.Time      `gorm:"not null;default:now()"`
}

// Product
type Product struct {
    ID             uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index"`
    SKU            string         `gorm:"not null;size:50"`
    Name           string         `gorm:"not null;size:500"`
    Description    string         `gorm:"type:text"`
    BasePrice      int64          `gorm:"not null;default:0"`
    Status         string         `gorm:"not null;default:'draft';index"`
    Attributes     datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
    SearchVector   string         `gorm:"type:tsvector;index:,type:gin"`
    CreatedAt      time.Time      `gorm:"not null;default:now()"`
    UpdatedAt      time.Time      `gorm:"not null;default:now()"`
    DeletedAt      gorm.DeletedAt `gorm:"index"`
    
    Organization   Organization   `gorm:"foreignKey:OrganizationID"`
    Categories     []Category     `gorm:"many2many:product_categories;"`
    Variants       []ProductVariant `gorm:"foreignKey:ParentProductID"`
    Assets         []Asset        `gorm:"foreignKey:ProductID"`
}

// Category
type Category struct {
    ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    OrganizationID uuid.UUID  `gorm:"type:uuid;not null;index"`
    ParentID       *uuid.UUID `gorm:"type:uuid;index"`
    Name           string     `gorm:"not null;size:255"`
    Slug           string     `gorm:"not null;size:255"`
    Description    string     `gorm:"type:text"`
    DisplayOrder   int        `gorm:"not null;default:0;index"`
    IsActive       bool       `gorm:"not null;default:true"`
    CreatedAt      time.Time  `gorm:"not null;default:now()"`
    UpdatedAt      time.Time  `gorm:"not null;default:now()"`
    
    Organization   Organization `gorm:"foreignKey:OrganizationID"`
    Parent         *Category    `gorm:"foreignKey:ParentID"`
    Children       []Category   `gorm:"foreignKey:ParentID"`
    Products       []Product    `gorm:"many2many:product_categories;"`
}

// Asset
type Asset struct {
    ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    OrganizationID uuid.UUID `gorm:"type:uuid;not null;index"`
    ProductID      uuid.UUID `gorm:"type:uuid;not null;index"`
    FileName       string    `gorm:"not null;size:255"`
    StoragePath    string    `gorm:"uniqueIndex;not null;size:1000"`
    ContentType    string    `gorm:"not null;size:100"`
    FileSize       int64     `gorm:"not null"`
    AssetType      string    `gorm:"not null"`
    IsPrimary      bool      `gorm:"not null;default:false;index"`
    DisplayOrder   int       `gorm:"not null;default:0;index"`
    AltText        string    `gorm:"size:500"`
    CreatedAt      time.Time `gorm:"not null;default:now()"`
    
    Organization   Organization `gorm:"foreignKey:OrganizationID"`
    Product        Product      `gorm:"foreignKey:ProductID"`
}
```

---

## Migration Strategy

1. **Initial schema**: Create all tables with constraints
2. **Indexes**: Add performance indexes after table creation
3. **Full-text search**: Create tsvector column and trigger
4. **Seed data**: Create default organization for testing
5. **Validation**: Run constraints validation on existing data

**Migration Order**:
1. Organizations
2. Categories
3. Products
4. Product Categories (join table)
5. Product Variants
6. Assets
7. Attribute Definitions

---

## Data Integrity Rules

✅ **Referential Integrity**: All foreign keys enforced at database level  
✅ **Uniqueness**: SKUs unique per organization, slugs unique per organization  
✅ **Soft Deletes**: Products soft-deleted, cascade rules on hard deletes  
✅ **Timestamps**: All entities track creation and modification times  
✅ **Multi-tenancy**: organization_id on all entities for isolation  
✅ **Validation**: Check constraints for enums, ranges, and business rules  

**Status**: ✅ Data model complete and ready for implementation

