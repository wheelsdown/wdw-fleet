# AGENTS.md

Welcome. **wdw-fleet** is an API-first vehicle fleet management service.
Go backend, PostgreSQL, OpenAPI contract. Single binary, embedded
migrations, self-hosted, MIT-licensed. Rolls up the best ideas from
Road Trip HD, LubeLogger, Hammond, and May — with total cost of
ownership and a clean API-first story that none of them get right.

The web UI ships as **FleetAware** (same codebase; product/UI name).
Backend service name stays `wdw-fleet`.

If you're here to understand the project, start with
[api/openapi.yaml](api/openapi.yaml) — it's the contract everything
else implements or consumes.

Everything below is what you need to contribute code.

## Build & Test

All workflows go through [just](https://just.systems/). Never call `go`
tools directly — the justfile handles build flags and the ordering
of fmt/lint/test/build.

```bash
just build              # Build for current platform → bin/
just run                # Build and run the server locally
just ci                 # Full CI gate: fmt + lint + test + build
just test               # Tests only
just lint               # golangci-lint
just fmt                # gofmt + goimports
just dev-up             # Start PostgreSQL (docker compose)
just dev-down           # Stop PostgreSQL
just migrate-up         # Apply migrations manually (auto-applied on server start)
just generate           # Regenerate Go code from api/openapi.yaml
```

`just ci` must pass locally before every push. No exceptions. Don't
rely on GitHub Actions to catch what you could have caught locally.

## Code Conventions

- **Go 1.26+** required. Module path: `github.com/wheelsdown/wdw-fleet`.
- **Conventional commits**: `feat:`, `fix:`, `docs:`, `refactor:`,
  `test:`, `chore:`.
- **Prefer the standard library**. Third-party imports add supply chain
  risk, version churn, and transitive deps. Use stdlib when it can do
  the job. Discuss new dependencies before adding.
- **Context propagation**: Always pass the caller's `ctx` through to
  downstream calls. Never use `context.Background()` inside a handler
  that receives `ctx` — it breaks cancellation and deadline enforcement.
- **Error handling**: Handle errors explicitly. No silent swallowing.
  Wrap with context (`fmt.Errorf("doing X: %w", err)`). If it can fail,
  log it or return it — never drop.
- **Logging**: Structured via `slog` (JSON handler). Levels:
  - `INFO` — operator story (server started, migration applied, user
    signed in)
  - `DEBUG` — deep troubleshooting (SQL, request bodies, parser traces)
  - `WARN` — degraded (IMAP poll retry, webhook delivery failed)
  - `ERROR` — broken (panic recovered, DB unreachable)

  Include relevant context fields (`slog.String("vehicle_id", id)`) not
  interpolated into the message string.
- **Tests**: Table-driven where possible, always with `-race` in CI.
- **Go doc comments**: GoDoc is a primary audience for this codebase.
  Every exported symbol gets a doc comment starting with its name and
  reading as a complete sentence. Every package gets `// Package foo ...`.
  Write *why*, not *what* — the signature already says *what*.
- **Contract structs**: Exported types that cross package, API, or
  persistence boundaries need explicit serialization tags (`json:"..."`
  with `snake_case` names, `-` for runtime-only fields).
- **HTTP clients**: When outbound HTTP lands (webhook dispatch, IMAP
  auxiliary calls), use a single project-wide wrapper — don't construct
  `http.Client{}` directly in multiple packages. Wrapper enforces
  timeouts, retry policy, User-Agent, and TLS defaults.
- **Configuration**: Environment variables prefixed with `WDW_`. Load
  once at startup via `internal/config`. No scattered `os.Getenv()`.
- **No Google services.** Product decision, not a preference. No Google
  Fonts (use `fonts.bunny.net`), no Google OAuth, no Gmail API. All
  outbound email is via IMAP/SMTP to the operator's own mail server.

## Schema & Data

- **PostgreSQL only.** No abstraction layer. Use `pgx/v5` directly.
  PostgreSQL features (JSONB, partial indexes, arrays, CITEXT,
  LISTEN/NOTIFY) are all fair game.
- **Money as integer cents.** Column and field names end in `_cents`
  (`total_cents`, `labor_rate_cents`). Never float for currency.
- **UUIDs for user-facing primary keys.** `gen_random_uuid()` default.
- **Cursor-based pagination** on all list endpoints (`cursor` + `limit`,
  response includes `next_cursor` nullable).
- **`timestamptz` always**, never naive `timestamp`. The site timezone
  lives on `site_config`; display conversion happens at render time.
- **Soft delete on user-editable entities** via `deleted_at timestamptz`.
  Unique indexes on soft-deleted tables must include
  `WHERE deleted_at IS NULL`. Reference tables (sessions, task_parts,
  attachments, webhook_deliveries) hard-delete.
- **Enums as `text` + `CHECK`**, not PostgreSQL `ENUM` types. Widening
  an enum is a one-line migration instead of a four-step dance.
- **Migrations** live in `internal/database/migrations/` and are
  embedded into the binary via `go:embed`. They auto-apply on server
  startup. Name pattern: `NNN_short_name.up.sql` + `NNN_short_name.down.sql`.
- **Circular FKs** (e.g. `inbox_items` ↔ `tasks`/`expenses`) need
  `DROP TABLE ... CASCADE` in down migrations.

## Architecture at a Glance

- **API-first.** `api/openapi.yaml` is the source of truth. Handlers,
  typed clients, and the frontend all derive from it. When the API
  surface changes, update the spec first.
- **Single binary.** Server, migrations, and (eventually) the embedded
  frontend all ship as one `wdw-fleet` executable. The binary is always
  usable standalone, but the primary deployment path is containerized
  (see *Deployment* below).
- **Layers.**
  - `cmd/wdw-fleet/` — entry point, wiring
  - `internal/api/` — HTTP handlers + OpenAPI-generated code
  - `internal/config/` — env-var loading
  - `internal/database/` — pgx pool, embedded migrations
  - `internal/model/` — domain types (rebuilt after schema stabilizes)
  - `internal/service/` — business logic (thin; most work is SQL)
  - `internal/webhook/` — outbound dispatch; IMAP parsers
    under `internal/webhook/parsers/`
- **Frontend** (planned, not yet built): served via `embed.FS` from the
  same binary. See [`docs/design/`](docs/design/) for the FleetAware
  UI spec — it's the source of truth for look, behavior, and component
  anatomy. Stack (React+Vite+TS vs. HTMX+templates) not yet decided.

## Deployment

- **Container-first.** The primary deployment artifact is an OCI image.
  Target environments are Docker and Docker Compose; Kubernetes works
  but isn't the target.
- **Multi-arch builds**: `linux/amd64` and `linux/arm64` at minimum.
  Build with `docker buildx` from CI.
- **Base image**: distroless or Alpine. No shell in the final image
  unless there's a concrete reason. Builder stage handles compilation.
- **Rich tagging.** Every image push tags:
  - `vX.Y.Z` — semver release
  - `vX.Y` — minor track (auto-updates on patches)
  - `vX` — major track
  - `latest` — newest semver release (never a prerelease)
  - `sha-<shortsha>` — immutable git SHA tag for every build
  - `main` — tip of main (rolling, for CI/testing)
  - `edge` — prereleases / release candidates

  Additional OCI labels (`org.opencontainers.image.*`) carry source,
  revision, version, created, authors, licenses, and description.
- **Image registry**: GitHub Container Registry
  (`ghcr.io/wheelsdown/wdw-fleet`). Published via GitHub Actions on
  push to `main` and on tag.
- **Reproducibility**: Build args pin `SOURCE_DATE_EPOCH` and embed
  git SHA + version via `-ldflags`. Images should be bit-for-bit
  reproducible for a given source tree.
- **Config via environment**. `WDW_*` env vars only. No config files
  inside the image; all runtime config is injected.
- **Volumes**: `/var/lib/wdw-fleet/blob` for attachment storage.
  PostgreSQL is external (operator-managed or sidecar).
- **Health check**: `GET /healthz` on the configured listen port.

The in-tree [`Dockerfile`](Dockerfile) and
[`compose.yaml`](compose.yaml) are the reference for local dev and
the build pipeline. Production operators copy the compose file, edit
`WDW_*` env, and run.

## Security

- **Passwords**: argon2id only. Never bcrypt or scrypt for new code.
- **Session cookies**: random 256-bit token, stored as sha256 hash.
  Tokens are never logged or returned after initial issue.
- **API tokens**: same pattern — sha256-hashed in `site_config`.
  Raw token shown once on generation.
- **IMAP password**: encrypted at rest (`imap_password_encrypted bytea`).
  Key management TBD.
- **Webhook signatures**: HMAC-SHA256 using the webhook's secret.
- **CSRF**: on all state-changing form endpoints (when the frontend
  lands).
- **No TLS verification bypass** anywhere. If a self-signed cert needs
  trusting, document it and pin explicitly.
- **Avatars** resolve in this priority: (1) locally-uploaded
  `users.avatar_url`, (2) Libravatar (federated, open source) if
  `site_config.libravatar_enabled`, (3) Gravatar fallback if
  `site_config.libravatar_gravatar_fallback` (opt-in; Gravatar is
  proprietary and tracks IPs), (4) client-rendered initials. The
  default is Libravatar on, Gravatar off — no email hash reaches
  Automattic unless an operator explicitly enables it.

## Gotchas

- **Migrations auto-apply on server start.** A bad migration can brick
  a running deployment on restart. Test both up and down against a
  fresh Postgres before pushing.
- **`site_config` is single-row.** `CHECK (id = 1)` enforces this.
  Migration 001 seeds the row; onboarding wizard updates it.
- **Timestamps in DB are UTC.** Convert to site timezone only at the
  display boundary. Don't store local time.
- **Currency is site-wide.** Historical data is USD-normalized on
  import. No per-record currency override.
- **FleetAware design tokens are final.** Don't invent new colors,
  sizes, or radii. If something isn't in `tokens.css`, ask.

## Contributing

### Commits

- **All commits must be signed.** The session-local git config should
  be set to sign with `~/.claude/ssh/id_claude` as
  `Claude Code (nugget) <claude@nugget.info>`. PRs with unsigned
  commits will not merge.
- Conventional commit format for messages and PR titles.
- Reference issues with `Refs #NNN` or `Closes #NNN` in commit bodies.
- Keep commits focused — one logical change per commit.

### Pull Requests

- Run `just ci` locally before pushing. This is mandatory.
- Keep PRs focused — one logical change per PR.
- **Update docs in the same PR.** If your change affects behavior that's
  documented — API surface, configuration, schema, CLI flags — update
  the relevant docs (OpenAPI spec, AGENTS.md, README, GoDoc) before
  requesting review. Docs that drift from code are worse than no docs.

### Review Culture

Leave PRs clean and reflective of reality. Open review threads, stale
descriptions, and unchecked test-plan items signal unfinished work.

When addressing review feedback: fix the issue, reply to the thread
with the commit hash and a one-line explanation, then resolve the
conversation. If deferring, say why before resolving.

## Further Reading

- [api/openapi.yaml](api/openapi.yaml) — API contract (source of truth)
- [docs/design/](docs/design/) — FleetAware UI design handoff
- [CLAUDE.md](CLAUDE.md) — Claude Code operator-specific notes
