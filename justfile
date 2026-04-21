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

# Build a multi-arch OCI image locally via buildx. Matches CI.
# Requires: docker buildx (present in recent Docker Desktop / engine).
# By default loads linux/<host-arch> into the local daemon; pass
# PLATFORMS=linux/amd64,linux/arm64 PUSH=true to cross-build and push.
docker-build:
    #!/usr/bin/env bash
    set -euo pipefail
    VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
    COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
    BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    SOURCE_DATE_EPOCH="$(git log -1 --pretty=%ct 2>/dev/null || date +%s)"
    PLATFORMS="${PLATFORMS:-linux/$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')}"
    OUTPUT="${PUSH:+--push}"
    OUTPUT="${OUTPUT:---load}"
    # --load only supports a single platform; warn+override if user asked
    # for multi-arch without --push.
    if [[ "$PLATFORMS" == *,* && -z "${PUSH:-}" ]]; then
        echo "note: multi-arch build requires PUSH=true; falling back to single-arch --load"
        PLATFORMS="linux/$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"
    fi
    docker buildx build \
        --platform "$PLATFORMS" \
        --build-arg VERSION="$VERSION" \
        --build-arg COMMIT="$COMMIT" \
        --build-arg BUILD_DATE="$BUILD_DATE" \
        --build-arg SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" \
        -t wdw-fleet:latest \
        -t "wdw-fleet:$VERSION" \
        $OUTPUT \
        .

# Build both linux/amd64 and linux/arm64 and push to the configured registry.
# Intended for ad-hoc manual publishing; CI is the normal publish path.
docker-push:
    PUSH=true PLATFORMS=linux/amd64,linux/arm64 just docker-build
