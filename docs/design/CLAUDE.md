# Claude Code — implementation brief

You're implementing the FleetAware web UI inside the existing **wdw-fleet** repo (Go/PostgreSQL/OpenAPI). The design artifacts are in this folder; the target repo already has a `CLAUDE.md` at its root — obey both.

## Read these first, in this order

1. `README.md` — full design spec. Tokens, screens, data shapes, behavior.
2. `tokens.css` — paste-ready CSS custom properties for both palettes.
3. `tokens.json` — same values, JSON. Useful if you pick a JS tooling path.
4. `api-additions.yaml` — OpenAPI fragment for endpoints the UI needs that likely aren't in `api/openapi.yaml` yet. Reconcile with the existing spec; don't blindly append.
5. `FleetAware.html` — the prototype. **This is the source of truth for look and behavior.** Open it in a browser to see each screen; grep it to extract exact values, component structure, and copy. All state is client-side mock data.

## Order of work

Do this sequentially. Stop and ask the user after each checkpoint before moving on.

### Checkpoint 0 — environment decisions

Before writing code, confirm with the user:

- **Frontend stack.** README recommends React + Vite + TS served via `embed.FS`. HTMX + templates is a viable alternative. Do not decide alone.
- **Where it lives in the repo.** Suggest `web/` for frontend source, `internal/web/` for the Go handler that serves it. Match existing repo conventions.
- **API client generation.** Typed client from `api/openapi.yaml` via `openapi-typescript` + a fetch wrapper, or something else. Confirm.

Once confirmed, scaffold the empty project + build pipeline + `go:embed` serving. Verify `just ci` still passes.

### Checkpoint 1 — tokens + shell

- Port `tokens.css` to the chosen stack. Wire the `[data-theme]` switcher on `<html>`.
- Port `applyPalette` / `resolvePalette` from the prototype — runtime overrides for `primaryColor` / `accentColor` must hot-swap without reload.
- Build `Topnav`, `AvatarMenu`, `UserAvatar`, `StatusDot`, `VGlyph`, `PhotoHolder` as real components. Get the nav visible with no screens behind it.
- Wire theme persistence via a GET/PATCH `/v1/me` endpoint (see `api-additions.yaml`).

### Checkpoint 2 — Fleet + Vehicle detail

These two screens are the highest-traffic path. Build them against real endpoints (`GET /v1/vehicles`, `GET /v1/vehicles/{id}`, `GET /v1/vehicles/{id}/timeline`). Seed the database with fixture vehicles matching the prototype's mock data so you have something to render.

### Checkpoint 3 — Reminders + Parts + Expenses

Data-heavy list screens. Same pattern: endpoint → query → table. KPI strips compute on the server — don't ship all rows to the client and tally in JS.

### Checkpoint 4 — Log-task modal

The biggest interaction. Build it last; all three modes (Form / AI chat / Template). The AI chat mode calls Claude (Anthropic API) to extract structured task data — design assumes a server-side passthrough so the API key doesn't ship to the browser. Add `POST /v1/tasks/extract` accordingly.

### Checkpoint 5 — Fuel fill-up modal

Second camera flow. Image → `POST /v1/fuel/extract` → review → `POST /v1/fuel`. Camera access via `getUserMedia`; fall back to file input on desktop.

### Checkpoint 6 — Inbox (IMAP)

Backend-heavy. The UI is the easy half. See `api-additions.yaml` for the Inbox endpoints; see `README.md` "Integrations → Vendor receipt import" for IMAP behavior and parser contract.

### Checkpoint 7 — Profile + Admin

Settings screens. Mostly forms. Site & branding → color pickers hot-reload tokens on save. Avatar upload goes to `POST /v1/me/avatar` — do **not** persist data URLs to Postgres; upload to a real blob store or a configured filesystem path.

### Checkpoint 8 — Polish

- Empty states for every list screen (prototype only has populated states)
- Error toasts for failed requests (prototype has success toast only)
- Loading skeletons
- Keyboard shortcuts (Esc closes modals, `g f` → fleet, `g r` → reminders, `/` focuses search — confirm with user)
- Mobile layout pass for Fleet, Vehicle detail, Log task (below 900px wide)

## Critical rules

- **Money is integer cents.** Never float. Per repo `CLAUDE.md`.
- **IDs are UUIDs.** Per repo `CLAUDE.md`.
- **No Google anywhere.** No Google Fonts, no Google OAuth, no Gmail API. Fonts via fonts.bunny.net or self-hosted. This is a product decision, not a preference.
- **Don't invent design values.** If a color, size, or radius isn't in `tokens.css`, ask. Don't eyeball a new one.
- **Don't copy the HTML file as production code.** It's a reference. Rebuild idiomatically.
- **Use `just <target>`.** Never call `go build` / `go test` directly per repo convention.
- **Migrations are embedded.** Any new tables for Inbox / Crew / SiteConfig go in `migrations/` and are picked up automatically by the existing boot path.

## When to ask, when to decide

**Ask the user:**
- Stack, layout, and placement decisions (checkpoint 0)
- Any UX ambiguity not explicitly pinned down by the prototype
- Whether to add something the prototype doesn't show (empty states are OK to invent in-palette; whole new screens are not)

**Decide yourself:**
- File organization inside whatever frontend folder gets chosen
- Test framework + test layout (match existing Go test conventions where overlap exists)
- Internal naming of components, hooks, utilities

## Verification

After each checkpoint, run `just ci` (fmt, lint, test, build). Take a screenshot of the affected screen(s) and compare side-by-side with the prototype open in another tab. Pixel-diff tolerances: ±2px spacing, exact colors, exact typography.

## What the design is *not*

- Not responsive to phones. Tablet is the smallest target; phone layout is a follow-up.
- Not internationalized. Strings are US English. i18n is a follow-up.
- Not accessible beyond baseline (semantic HTML, focus rings). A11y pass is a follow-up.
- Not themeable beyond primary/accent overrides. Don't add a full theme editor.

Flag these as follow-ups in the handoff summary; don't silently implement them.
