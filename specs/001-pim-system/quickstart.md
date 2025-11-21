# Quick Start Guide: PIM System

**Feature**: Product Information Management System  
**Branch**: `001-pim-system`  
**Audience**: Developers implementing or integrating with the PIM system

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Project Setup](#project-setup)
3. [Database Setup](#database-setup)
4. [Generate Protobuf Code](#generate-protobuf-code)
5. [Run the Application](#run-the-application)
6. [API Examples](#api-examples)
7. [Running Tests](#running-tests)
8. [Development Workflow](#development-workflow)
9. [Troubleshooting](#troubleshooting)

---

## Prerequisites

Ensure you have the following installed:

- **Go 1.21+** ([download](https://go.dev/dl/))
- **PostgreSQL 15+** ([download](https://www.postgresql.org/download/))
- **Protocol Buffer Compiler** (`protoc`) ([install guide](https://grpc.io/docs/protoc-installation/))
- **Docker** (optional, for testcontainers) ([download](https://www.docker.com/get-started))

### Install Go Tools

```bash
# Install protoc-gen-go for generating Go code from .proto files
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

# Verify installation
protoc --version  # Should be 3.0 or higher
go version        # Should be 1.21 or higher
```

---

## Project Setup

### 1. Clone the Repository

```bash
git clone <repository-url>
cd pim-demo
git checkout 001-pim-system
```

### 2. Install Dependencies

```bash
go mod download
go mod verify
```

### 3. Project Structure Overview

```
pim-demo/
├── api/v1/              # Protobuf definitions (.proto files)
├── api/gen/v1/          # Generated protobuf Go code
├── services/            # Business logic (public, reusable)
├── handlers/            # HTTP handlers
├── internal/            # Internal implementation details
│   ├── models/          # GORM database models
│   ├── middleware/      # HTTP middleware
│   └── config/          # Configuration
├── cmd/api/             # Application entry point
├── tests/               # Integration tests
└── specs/               # Feature specifications
```

---

## Database Setup

### Option 1: Local PostgreSQL

**Create Database**:
```bash
createdb pim_demo
createuser pim_user -P  # Enter password when prompted
```

**Grant Permissions**:
```sql
psql -d pim_demo -c "GRANT ALL PRIVILEGES ON DATABASE pim_demo TO pim_user;"
psql -d pim_demo -c "GRANT ALL ON SCHEMA public TO pim_user;"
```

**Enable Extensions**:
```sql
psql -d pim_demo <<EOF
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
EOF
```

### Option 2: Docker PostgreSQL

```bash
docker run --name pim-postgres \
  -e POSTGRES_DB=pim_demo \
  -e POSTGRES_USER=pim_user \
  -e POSTGRES_PASSWORD=pim_password \
  -p 5432:5432 \
  -d postgres:15

# Wait for PostgreSQL to start
sleep 5

# Enable extensions
docker exec -it pim-postgres psql -U pim_user -d pim_demo \
  -c "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";" \
  -c "CREATE EXTENSION IF NOT EXISTS \"pg_trgm\";"
```

### Configure Database Connection

Create `.env` file in project root:

```bash
# .env
DATABASE_URL=postgres://pim_user:pim_password@localhost:5432/pim_demo?sslmode=disable
PORT=8080
ENV=development
```

Or set environment variables:

```bash
export DATABASE_URL="postgres://pim_user:pim_password@localhost:5432/pim_demo?sslmode=disable"
export PORT=8080
export ENV=development
```

---

## Generate Protobuf Code

Generate Go code from Protocol Buffer definitions:

```bash
# Generate all protobuf files
make proto-gen

# Or manually:
protoc \
  --proto_path=api/v1 \
  --go_out=api/gen/v1 \
  --go_opt=paths=source_relative \
  api/v1/*.proto
```

**Verify Generated Code**:
```bash
ls -la api/gen/v1/
# Should see: common.pb.go, product.pb.go, category.pb.go, variant.pb.go, asset.pb.go, import_export.pb.go
```

---

## Run the Application

### 1. Run Database Migrations

```bash
# Auto-migrate GORM models
go run cmd/api/main.go migrate

# Or use make target
make migrate
```

### 2. Start the Server

```bash
# Run with hot reload (using air or similar)
make dev

# Or run directly
go run cmd/api/main.go serve
```

**Expected Output**:
```
2025/11/21 10:00:00 Starting PIM API server...
2025/11/21 10:00:00 Database connected: pim_demo
2025/11/21 10:00:00 Migrations applied successfully
2025/11/21 10:00:00 Server listening on :8080
```

### 3. Verify Server is Running

```bash
curl http://localhost:8080/health
```

**Expected Response**:
```json
{
  "status": "healthy",
  "timestamp": "2025-11-21T10:00:00Z",
  "version": "0.1.0"
}
```

---

## API Examples

### Base URL
```
http://localhost:8080/api/v1
```

### Authentication Header
```
Authorization: Bearer <your-jwt-token>
```

### 1. Create a Product

```bash
curl -X POST http://localhost:8080/api/v1/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "sku": "LAPTOP-001",
    "name": "Professional Laptop",
    "description": "High-performance laptop for professionals",
    "base_price": 129900,
    "status": 2,
    "attributes": {
      "brand": {"string_value": "TechBrand"},
      "color": {"string_value": "Silver"},
      "weight_kg": {"number_value": 1.5}
    }
  }'
```

**Response**:
```json
{
  "product": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "organization_id": "org-uuid",
    "sku": "LAPTOP-001",
    "name": "Professional Laptop",
    "description": "High-performance laptop for professionals",
    "base_price": 129900,
    "status": 2,
    "attributes": {
      "brand": {"string_value": "TechBrand"},
      "color": {"string_value": "Silver"},
      "weight_kg": {"number_value": 1.5}
    },
    "created_at": "2025-11-21T10:00:00Z",
    "updated_at": "2025-11-21T10:00:00Z"
  }
}
```

### 2. Get a Product

```bash
curl -X GET http://localhost:8080/api/v1/products/123e4567-e89b-12d3-a456-426614174000 \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 3. List Products with Filters

```bash
curl -X GET "http://localhost:8080/api/v1/products?page=1&page_size=20&status=2&min_price=50000&max_price=200000" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 4. Search Products

```bash
curl -X POST http://localhost:8080/api/v1/products/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "query": "laptop professional",
    "pagination": {"page": 1, "page_size": 10}
  }'
```

### 5. Create a Category

```bash
curl -X POST http://localhost:8080/api/v1/categories \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "name": "Electronics",
    "slug": "electronics",
    "description": "Electronic devices and accessories",
    "is_active": true
  }'
```

### 6. Upload an Asset

```bash
curl -X POST http://localhost:8080/api/v1/assets \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -F "product_id=123e4567-e89b-12d3-a456-426614174000" \
  -F "file=@/path/to/product-image.jpg" \
  -F "is_primary=true" \
  -F "alt_text=Professional laptop front view"
```

### 7. Import Products (CSV)

```bash
curl -X POST http://localhost:8080/api/v1/import/products \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -F "csv_file=@/path/to/products.csv" \
  -F "mode=3"  # CREATE_OR_UPDATE
```

### 8. Export Products

```bash
curl -X POST http://localhost:8080/api/v1/export/products \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "format": 1,
    "filter": {"statuses": [2]},
    "include_variants": true
  }' \
  --output products_export.csv
```

---

## Running Tests

### Run All Tests

```bash
# Run all integration tests
go test ./tests/integration/... -v

# Run with race detector
go test ./tests/integration/... -v -race

# Run specific test file
go test ./tests/integration/product_test.go -v

# Run specific test function
go test ./tests/integration/product_test.go -v -run TestCreateProduct
```

### Test with Coverage

```bash
# Generate coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# View coverage in browser
open coverage.html
```

### Run Error Handling Tests

```bash
# Test all sentinel errors
go test ./tests/integration/error_handling_test.go -v -run TestAllSentinelErrors

# Test all HTTP error codes
go test ./tests/integration/error_handling_test.go -v -run TestAllHTTPErrorCodes

# Test complete error flow
go test ./tests/integration/error_handling_test.go -v -run TestErrorFlowEndToEnd
```

### Test Database Setup

Tests use **testcontainers-go** to spin up PostgreSQL automatically. No manual setup needed.

**First time setup** (downloads PostgreSQL Docker image):
```bash
# This may take a few minutes on first run
go test ./tests/integration/product_test.go -v
```

---

## Development Workflow

### 1. Make Code Changes

Follow TDD (Test-Driven Development):
1. Write test first (integration test)
2. Run test (should fail)
3. Write minimal code to pass test
4. Refactor if needed
5. Repeat

### 2. Run Tests Continuously

```bash
# Watch for file changes and run tests (using air or similar)
make test-watch

# Or manually after each change
go test ./... -v
```

### 3. Check Linting

```bash
# Run golangci-lint
make lint

# Or directly
golangci-lint run ./...
```

### 4. Format Code

```bash
# Format all Go files
go fmt ./...

# Or use goimports (includes import organization)
goimports -w .
```

### 5. Update Protobuf Contracts

When modifying `.proto` files:

```bash
# 1. Edit .proto file in api/v1/
vim api/v1/product.proto

# 2. Regenerate Go code
make proto-gen

# 3. Update tests to match new contract
vim tests/integration/product_test.go

# 4. Run tests
go test ./... -v
```

### 6. Database Migrations

```bash
# Add new GORM model field
vim internal/models/product.go

# Run migration
go run cmd/api/main.go migrate

# Verify in database
psql -d pim_demo -c "\d products"
```

---

## Makefile Commands

The project includes a `Makefile` for common tasks:

```bash
make help           # Show all available commands
make dev            # Run development server with hot reload
make test           # Run all tests
make test-race      # Run tests with race detector
make test-coverage  # Generate coverage report
make lint           # Run linter
make fmt            # Format code
make proto-gen      # Generate protobuf code
make migrate        # Run database migrations
make seed           # Seed database with test data
make clean          # Clean build artifacts
make build          # Build production binary
make docker-build   # Build Docker image
make docker-run     # Run in Docker
```

---

## Troubleshooting

### Issue: Database Connection Failed

**Error**: `pq: password authentication failed for user "pim_user"`

**Solution**:
```bash
# Verify PostgreSQL is running
pg_isready -h localhost -p 5432

# Check credentials in .env file
cat .env | grep DATABASE_URL

# Test connection manually
psql -U pim_user -d pim_demo -h localhost
```

### Issue: Protobuf Generation Failed

**Error**: `protoc-gen-go: program not found or is not executable`

**Solution**:
```bash
# Install protoc-gen-go
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

# Verify it's in PATH
which protoc-gen-go

# Add to PATH if needed
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Issue: Port Already in Use

**Error**: `bind: address already in use`

**Solution**:
```bash
# Find process using port 8080
lsof -i :8080

# Kill the process
kill -9 <PID>

# Or use different port
export PORT=8081
go run cmd/api/main.go serve
```

### Issue: Tests Fail with Docker Connection

**Error**: `Cannot connect to Docker daemon`

**Solution**:
```bash
# Start Docker daemon
# macOS: Open Docker Desktop
# Linux: sudo systemctl start docker

# Verify Docker is running
docker ps
```

### Issue: Import Performance Slow

**Problem**: CSV import taking too long for large files

**Solution**:
```bash
# Ensure database has proper indexes
psql -d pim_demo -c "\d+ products"

# Check for organization_id and sku indexes

# Use async import for files > 1000 rows
curl -X POST http://localhost:8080/api/v1/import/products \
  -F "csv_file=@large_file.csv" \
  -F "async=true"
```

---

## Additional Resources

- **Full Specification**: [spec.md](./spec.md)
- **Data Model**: [data-model.md](./data-model.md)
- **Research & Decisions**: [research.md](./research.md)
- **API Contracts**: [contracts/](./contracts/)
- **Constitution**: [.specify/memory/constitution.md](../../.specify/memory/constitution.md)

---

## Next Steps

1. ✅ Complete local setup following this guide
2. ⏭️ Read [spec.md](./spec.md) to understand requirements
3. ⏭️ Review [data-model.md](./data-model.md) to understand entities
4. ⏭️ Explore [contracts/](./contracts/) to understand API
5. ⏭️ Run `/speckit.tasks` to break down implementation tasks
6. ⏭️ Start implementing following TDD approach
7. ⏭️ Run tests continuously: `go test ./... -v`

---

**Questions or Issues?** Check the [troubleshooting section](#troubleshooting) or refer to the [constitution](../../.specify/memory/constitution.md) for development principles.

**Happy Coding! 🚀**

