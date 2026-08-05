# FleetAware design handoff

Verbatim vendor artifact delivered 2026-04-21. This is the source of
truth for the web UI — colors, typography, component anatomy, screen
layouts, and interaction semantics.

## Files

- [`README.md`](README.md) — Full design spec (tokens, screens, data
  shapes, behavior, integrations)
- [`CLAUDE.md`](CLAUDE.md) — Implementation brief for Claude Code
  (checkpoint order, what to ask vs. decide, verification)
- [`FleetAware.html`](FleetAware.html) — Self-contained React + inline
  Babel prototype. Open in a browser to see each screen in action.
  All state is client-side mock data.
- [`tokens.css`](tokens.css) — Paste-ready CSS custom properties
  (light + dark palettes)
- [`tokens.json`](tokens.json) — Same values, JSON (for JS tooling)
- [`api-additions.yaml`](api-additions.yaml) — OpenAPI fragment for
  endpoints the UI assumes. These need to be reconciled into the Go
  route table at `internal/server/api/routes.go` (spec is generated,
  not hand-authored); use this fragment as a design reference for the
  operations and DTO shapes the UI expects.

## How to use

1. Read `README.md` first — orientation.
2. Scan `FleetAware.html` in a browser — this is the visual spec.
3. `tokens.css` / `tokens.json` when wiring the theme system.
4. `api-additions.yaml` when extending the OpenAPI spec.

Don't treat `FleetAware.html` as production code — it's a reference.
Rebuild idiomatically in whatever frontend stack gets chosen.
