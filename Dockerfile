# syntax=docker/dockerfile:1.9
#
# Multi-stage reproducible build. The builder compiles a static, stripped
# binary with build-time metadata injected via -ldflags; the final stage
# is distroless/static:nonroot -- no shell, no package manager, non-root
# by default.
#
# All build metadata arrives via --build-arg. GitHub Actions populates
# these from the metadata-action output; `just docker-build` passes the
# same values from the local git context.

ARG GO_VERSION=1.26

# ---------------------------------------------------------------------------
# Stage 1: builder
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
ARG SOURCE_DATE_EPOCH

WORKDIR /src

# Cache module downloads independently of source changes.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Static, stripped, cross-compiled. -trimpath + SOURCE_DATE_EPOCH +
# deterministic flags get us reproducible builds.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build \
        -trimpath \
        -buildvcs=false \
        -ldflags "-s -w \
            -X github.com/wheelsdown/wdw-fleet/internal/version.Version=${VERSION} \
            -X github.com/wheelsdown/wdw-fleet/internal/version.Commit=${COMMIT} \
            -X github.com/wheelsdown/wdw-fleet/internal/version.BuildDate=${BUILD_DATE}" \
        -o /out/wdw-fleet \
        ./cmd/wdw-fleet

# ---------------------------------------------------------------------------
# Stage 2: runtime
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# OCI image spec labels. Registries (ghcr.io, Docker Hub) surface these.
LABEL org.opencontainers.image.title="wdw-fleet" \
      org.opencontainers.image.description="API-first vehicle fleet management service (FleetAware)" \
      org.opencontainers.image.vendor="Wheels Down Workshop" \
      org.opencontainers.image.url="https://github.com/wheelsdown/wdw-fleet" \
      org.opencontainers.image.source="https://github.com/wheelsdown/wdw-fleet" \
      org.opencontainers.image.documentation="https://github.com/wheelsdown/wdw-fleet/blob/main/AGENTS.md" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.authors="Wheels Down Workshop <claude@nugget.info>"

COPY --from=builder /out/wdw-fleet /usr/local/bin/wdw-fleet

# distroless/static:nonroot runs as uid 65532.
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/wdw-fleet"]
