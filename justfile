set dotenv-load

default:
    @just --list

# Build the binary
build:
    go build -o bin/wdw-fleet ./cmd/wdw-fleet

# Run the server
run: build
    ./bin/wdw-fleet

# Run tests
test:
    go test ./...

# Run tests with coverage
test-coverage:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
    golangci-lint run ./...

# Format code
fmt:
    gofmt -w .
    goimports -w .

# Run database migrations up
migrate-up:
    go run ./cmd/wdw-fleet migrate up

# Run database migrations down
migrate-down:
    go run ./cmd/wdw-fleet migrate down

# Generate code from OpenAPI spec
generate:
    oapi-codegen -package api -generate types,chi-server,spec -o internal/api/openapi.gen.go api/openapi.yaml

# Full CI gate — run before every push
ci: fmt lint test build

# Start local dev dependencies (PostgreSQL)
dev-up:
    docker compose up -d

# Stop local dev dependencies
dev-down:
    docker compose down

# Build Docker image
docker-build:
    docker build -t wdw-fleet:latest .
