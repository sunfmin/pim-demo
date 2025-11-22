.PHONY: help proto-gen test test-race test-coverage lint fmt migrate dev build clean

## help: Display this help message
help:
	@echo "Available targets:"
	@grep -E '^## ' Makefile | sed 's/^## /  /'

## proto-gen: Generate Go code from protobuf definitions
proto-gen:
	protoc --proto_path=api/v1 \
		--go_out=api/gen/v1 \
		--go_opt=paths=source_relative \
		api/v1/*.proto

## test: Run all tests
test:
	go test -v ./...

## test-race: Run tests with race detector
test-race:
	go test -v -race ./...

## test-coverage: Generate test coverage report
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## lint: Run linter
lint:
	golangci-lint run ./...

## fmt: Format code
fmt:
	go fmt ./...
	goimports -w .

## migrate: Run database migrations
migrate:
	go run cmd/api/main.go migrate

## dev: Run development server with hot reload
dev:
	go run cmd/api/main.go serve

## build: Build production binary
build:
	go build -o bin/pim-api cmd/api/main.go

## clean: Clean build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html

## docker-build: Build Docker image
docker-build:
	docker build -t pim-api:latest .

## docker-run: Run Docker container
docker-run:
	docker run -p 8080:8080 --env-file .env pim-api:latest

