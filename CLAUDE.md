# wdw-fleet — Wheels Down Workshop Fleet Manager

API-first vehicle fleet management service. Go backend, PostgreSQL, OpenAPI contract.

## Quick Start

```sh
just dev-up     # Start PostgreSQL
just run        # Build and run the server
just ci         # Full CI gate: fmt, lint, test, build
```

## Project Layout

```
cmd/wdw-fleet/          Main application entry point
internal/
  api/                  HTTP handlers and OpenAPI generated code
  config/               Configuration loading (env vars)
  database/             Connection pool and embedded migrations
  model/                Domain types
  service/              Business logic layer
  webhook/              Webhook dispatch
migrations/             SQL migration files (also embedded in binary)
api/                    OpenAPI spec (source of truth)
```

## Conventions

- **Config**: Environment variables prefixed with `WDW_`
- **Money**: Stored as integers in smallest currency unit (cents for USD)
- **IDs**: UUIDs everywhere
- **Pagination**: Cursor-based (`cursor` + `limit` params)
- **Database**: PostgreSQL only — no abstraction layer. Use pgx directly.
- **Migrations**: Embedded in the binary via `go:embed`, auto-applied on startup
- **Dependencies**: `justfile` is the task runner. Always use `just <target>`.
