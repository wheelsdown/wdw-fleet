# Handoff — FleetAware web UI

Frontend design package for **wdw-fleet** (the Go/PostgreSQL fleet-management service described in `CLAUDE.md`). The existing repo ships a backend only; this package covers the entire web UI that will sit in front of the OpenAPI contract.

---

## About the design files

`FleetAware.html` in this folder is a **design reference**, not production code.

It's a self-contained React + inline-Babel prototype used to lock down look, layout, navigation, copy, and interaction semantics. Everything runs client-side from mock data; none of the state persists to a backend.

**The task is to recreate these screens inside wdw-fleet's target web environment**, wired to the real API. The HTML is the visual spec — extract tokens, component anatomy, and behavior from it, then build idiomatic components in whatever stack gets chosen (see *Target environment* below).

## Fidelity

**High fidelity.** Colors, typography, spacing, component sizing, copy, and interaction semantics are all final and should be recreated pixel-accurate. Data shapes and volumes in the mock are representative; treat them as guidance for real content.

The prototype does not cover:
- Small-viewport / mobile layouts (design was desktop/tablet first — see *Responsive behavior*)
- Empty states for most screens (have a single "first-run" state — see *Empty states*)
- Error toasts for failed requests (design exists for the success toast only)
- Progressive loading / skeletons

Those gaps should be filled using the existing design tokens — don't invent a new style.

---

## Target environment

The wdw-fleet backend is Go + pgx + embedded migrations + an OpenAPI spec under `api/`. The frontend does not yet exist.

**Recommended frontend stack** (ordered by fit with the project's single-binary, self-hosted, OSS ethos):

1. **React + Vite + TypeScript**, served from the Go binary via `embed.FS`. Matches the prototype 1:1 — component model, hooks, inline styles all translate directly. Generate a typed API client from `api/openapi.yaml` using `openapi-typescript` + `@hey-api/openapi-ts` or similar.
2. **HTMX + Go templates.** Very Go-idiomatic; zero JS toolchain; fewer moving parts. Viable for most screens but the Log-task modal, AI chat mode, and color picker will need sprinkled Alpine.js or custom JS.
3. **Svelte or Solid.** Lighter runtime than React. Workable but loses the direct mapping to the prototype.

Whichever stack is chosen, the design assumes:
- **Browser ≥ 2 years old** (`color-mix`, CSS variables, flexbox/grid, `prefers-color-scheme`)
- **Inter** (sans) and **JetBrains Mono** (mono) served from `fonts.bunny.net` — a privacy-respecting Google-Fonts mirror. No Google CDNs anywhere in the final build per product decision.

---

## Information architecture

```
Top nav tabs
├─ Fleet               — grid of all vehicles
│   └─ (row click)     → Vehicle detail
├─ Reminders           — upcoming + overdue maintenance across the fleet
├─ Tasks               — flat work-log feed (not yet wired)
├─ Parts               — inventory by vendor/category, reorder signals
├─ Unfiled             — ingestion inbox (IMAP-imported receipts, unknown-sender items)
└─ Reports             — expenses, categorised, filterable by vehicle

Avatar dropdown (top-right)
├─ Profile & preferences   — per-user settings (theme, density, identity, security)
└─ Site administration     — admin-only; site-wide settings
    ├─ Site & branding     — title, tagline, logo, primary/accent colors
    ├─ Crew                — attribution labels for "Performed by"
    ├─ Units & costs       — distance, volume, labor rate, fuel-price tracking
    ├─ Integrations        — Vendor receipt import (IMAP) + Webhook
    └─ Data & export       — full ZIP, per-vehicle PDF, API tokens

Overlays (modal, reachable from anywhere)
├─ Log a task          — form / AI chat / template modes
└─ Fuel fill-up        — camera capture → review (floating orange FAB bottom-right)
```

---

## Design tokens

All tokens live at the top of `FleetAware.html`. Two palettes (`WDW_LIGHT`, `WDW_DARK`) plus a runtime-mutable `WDW` that mirrors the active palette. If rebuilding in CSS, port these to CSS custom properties on `:root` / `[data-theme="dark"]`.

### Color — Light

| Role         | Hex       | Usage |
|---           |---        |---|
| `teal`       | `#1f7a7f` | Primary brand. Buttons, active tabs, focus ring, links |
| `tealDeep`   | `#155256` | Primary text on teal-soft backgrounds |
| `tealSoft`   | `#e6f2f2` | Pill backgrounds, focus halo, admin chip |
| `orange`     | `#e85c2b` | Accent. Floating action button, overdue callouts |
| `orangeSoft` | `#fbe5db` | Accent pill backgrounds |
| `ink`        | `#15181a` | Body text, headings |
| `ink2`       | `#2a2e31` | Secondary text |
| `ink3`       | `#4a4f54` | Tertiary, inactive tab labels |
| `muted`      | `#7a7f85` | Meta, placeholder |
| `faint`      | `#c5c9cd` | Disabled, off-state toggle track |
| `line`       | `#e4e1d9` | Hairline dividers, card borders |
| `lineStrong` | `#cfcac0` | Input borders, pill outlines |
| `paper`      | `#f7f4ed` | Page background (warm off-white) |
| `paperAlt`   | `#eeeae1` | Active tab background, photo placeholder base |
| `card`       | `#ffffff` | Card surfaces |
| `good`       | `#2f7d4a` | Success, "on track" status |
| `warn`       | `#b97a0e` | Due soon |
| `bad`        | `#b03a2e` | Overdue, destructive actions |

### Color — Dark

Same role semantics; values differ. See the `WDW_DARK` block at top of the HTML. Key differences: `paper = #14171a`, `card = #22272a`, `teal = #4fb6bb`, `ink = #ece8dc`. Don't hand-pick new darks — port verbatim.

### Color overrides

Admins can override `teal` (primary) and `orange` (accent) at the site level. When overridden, the override replaces both the light-mode and dark-mode value of that role. Persist to the backend keyed by `site_config.primary_color` / `site_config.accent_color` (hex string or null).

### Typography

```
sans: 'Inter', system-ui, sans-serif         (weights 400 / 500 / 600 / 700)
mono: 'JetBrains Mono', SFMono-Regular, monospace   (weights 400 / 500 / 600)
```

Feature flags: `font-feature-settings: 'cv11','ss01','ss03'` on `.wdw` root; `'zero','ss01'` on `.wdw-mono` (for the slashed zero and calendar alignment).

| Class         | Size | Weight | Letter-spacing | Line-height | Use |
|---            |---   |---     |---             |---          |---|
| `wdw-h1`      | 22   | 600    | -0.3           | 1.2         | Screen titles |
| `wdw-h2`      | 16   | 600    | -0.1           | 1.25        | Card titles, section headers |
| `wdw-eyebrow` | 11   | 600    | 0.6, UPPER     | —           | Field labels, table headers |
| `wdw-small`   | 13   | 400    | —              | 1.45        | Body copy |
| `wdw-tiny`    | 11   | 400    | —              | 1.4         | Meta |
| `wdw-mono`    | inherit | 500 | —              | —           | IDs, numbers, dates, hex, paths |

### Spacing, radii, shadows

- Base rhythm: **4px**. Cards use 14–20px padding, screens 22–32px.
- Radii: **3** (chips) · **4** (inputs, small pills) · **5–6** (buttons, nav tabs) · **8–10** (cards, modals) · **15** (avatar) · **999** (long pills).
- Shadows: cards are borderless-with-hairline (`1px solid line`), not shadow. Only shadows used:
  - Modal: `0 24px 60px rgba(0,0,0,0.28)`
  - Toast: `0 8px 20px rgba(0,0,0,0.22)`
  - Dropdown menu: `0 12px 28px rgba(0,0,0,0.18)`
  - Floating FAB: `0 8px 20px rgba(232,92,43,0.35)` (colored)
- Focus ring: `box-shadow: 0 0 0 3px var(--teal-soft)` + `border-color: var(--teal)`.

---

## Screens

Each screen below lists purpose, layout, components, and anchor file:line in `FleetAware.html`.

### Fleet (landing) — `screen === 'fleet'`

**Purpose.** The home screen. See every vehicle at a glance; jump into any one.

**Layout.** Full-viewport flex-column. Top nav (52px fixed), body scroll. Body is a header row (title + totals + "Add vehicle" button) followed by a responsive CSS grid of vehicle cards.

**Vehicle card.**
- ~320×180px, `card` surface, 10px radius, 1px `line` border.
- Top strip: glyph (custom SVG — see *Vehicle glyphs*), name (wdw-h2), VIN (wdw-tiny wdw-mono wdw-muted).
- Mid strip: odometer in mono, unit suffix (`mi` / `hr` / `events`).
- Bottom strip: status dot (`good`/`warn`/`bad` using the semantic palette) + `dueIn` string, tags on right as `wdw-chip`.
- Hover: border darkens to `lineStrong`; click navigates to vehicle detail.

### Vehicle detail — `screen === 'vehicle'`

**Purpose.** Everything about one vehicle; timeline-driven.

**Layout.** 380px left sidebar (passport) + flex-1 right rail (timeline + tabs).

**Passport sidebar.** Vehicle photo (square, photo-placeholder with glyph), name/make/model/year, VIN, odometer + last-read date, next-due reminder summary, tags, "Edit vehicle" button.

**Right rail.**
- Horizontal tabs: `Timeline / Parts / Docs / Modifications / Costs`. Active tab underlined with `teal`, 2px, inset 10px.
- Timeline is a vertical list of events: each row has a date (wdw-mono wdw-muted), a type chip, a title (wdw-small 500), a cost (wdw-mono right-aligned), and a photo-strip afterward.
- Event types: `service` / `repair` / `fuel` / `inspection` / `modification` / `track` / `note`.

### Reminders — `screen === 'reminders'`

**Purpose.** "What needs doing across the fleet?"

**Layout.** Top: 3 KPI cards (Overdue / Due soon / On track) with left-edge 4px accent stripes (`bad` / `warn` / `good`). Below: filter chips (status + group-by toggle), then a grouped table.

**Table row.** Vehicle glyph + name · task · last-done date · interval (text, e.g. `5,000 mi` / `6 months` / `3 events`) · next-due · status pill · "Log done" button.

**Reminder kinds.** `date` (every N months), `odometer` (every N miles), `event` (every N track events — unique to enthusiast use case).

### Parts inventory — `screen === 'parts'`

**Purpose.** What's on the shelf, what's running low.

**Layout.** Filter sidebar (vendor list, categories) + main table.

**Row.** SKU (mono) · name · vendor chip · on-hand qty · reorder-point · last-used · cost. Reorder-needed rows pill `warn` or `bad`.

### Unfiled / Inbox — `screen === 'inbox'`

**Purpose.** Triage items imported from IMAP and elsewhere that need routing to a vehicle or rejected.

**Layout.** 380px list + detail pane.

**List row.** Source badge (email / upload / webhook), sender, subject, date, attachment icons, confidence pill (parser guess: "likely Shop Truck" etc.).

**Detail.** Metadata grid, suggested routing, accept/reject actions, history snippet ("Pulled from IMAP · receipts@…").

### Reports / Expenses — `screen === 'expenses'`

**Purpose.** Where does the money go?

**Layout.** KPI strip (filtered total, by-category bar breakdown) + category chip filter + vehicle filter + table.

**Table row.** Date · vehicle (glyph + name) · category chip · description · vendor · amount · recurring flag.

### Profile — `screen === 'profile'`

**Purpose.** Per-user settings; reachable from avatar dropdown.

**Layout.** 780px max-width content column.

**Sections** (each a `ProfileSection` — card with rows):
- **Identity** — avatar uploader, display name, email, initials
- **Appearance** — theme (Light / Dark / Auto), density (Comfortable / Compact)
- **Preferences** — default vehicle, distance unit override, keyboard shortcuts toggle, daily digest toggle
- **Security** — sign-in method (read-only, set by admin), change password, active sessions list

### Admin — `screen === 'admin'`

**Purpose.** Site-wide settings; visible only when `user.isAdmin`.

**Layout.** 220px left rail tabs + content.

**Tabs.**
1. **Site & branding** — site title, tagline, logo, **color-scheme** section (primary + accent color pickers with native color input + hex chip + 6 presets + reset button + live brand preview), address, currency, timezone.
2. **Crew** — table of attribution labels (Name / Role / Tasks logged). Add via "Add person". Not logins — see below.
3. **Units & costs** — distance (mi/km), volume (gal/L), default labor rate, parts markup toggle, fuel-price-tracking toggle.
4. **Integrations** — exactly two cards:
   - **Vendor receipt import** — IMAP config form (host, port, TLS SSL pill, username, password, folder, Test-connection button) + 8 parsers listed as health tiles (healthy / beta / fallback).
   - **Webhook** — endpoint URL input; HMAC-SHA256 signing note.
5. **Data & export** — full ZIP download, per-vehicle PDF report, rotating API token.

### Log-task modal — overlay

**Purpose.** Enter a service/maintenance/repair/modification event for a vehicle. Reachable from the top-right "Log task" CTA and from every screen via `ctx.onLogTask`.

**Layout.** Centered modal, 1120×740 max, translucent `rgba(21,24,26,0.55)` scrim.

**Header.** Modal title + vehicle-picker select + mode switcher pill (`Form` / `AI chat` / `Template`) + close.

**Form mode (primary path).**
- Left pane: task-type chips (Oil change / Service / Repair / Inspection / Fuel / Modification / Other) · title · date + odometer + performed-by grid · notes · parts table (inline rows with SKU, name, qty input, unit price, line total, remove) + search-parts row at bottom · labor (hours × rate = total).
- Right rail: "Copy from last" card · photos/attachments · "Schedule next" (interval pills +5,000 mi / +6 mo / Custom) with computed next-due · AI-extracted hints card · Save-draft + Log-task buttons (primary includes grand total).

**AI chat mode.** Two-pane: chat transcript + composer on left, live-draft card on right that updates as the AI parses the conversation.

**Template mode.** Grid of pre-configured task templates with usage counts; clicking one jumps to Form mode pre-filled.

### Fuel fill-up modal — overlay

**Purpose.** Quick fill-up logging, tablet-friendly.

**Entry point.** Floating orange FAB bottom-right (`Snap fill-up`).

**Flow.** Step 1 (camera capture, dark panel mocking a pump shot) → Step 2 (review extracted gallons / price / total with edit fields) → save. Two-step modal with header showing `Step N of 2`.

---

## Interactions & behavior

### Navigation

Single-page app; routes stored in local state as `{ screen, vehicleId }` and persisted to `localStorage` under `wdw_route` so a refresh keeps your place. On first load, default to `screen: 'fleet'`.

Nav events are dispatched by string label (`'Fleet'`, `'Reminders'`, `'Parts'`, `'Unfiled'`, `'Reports'`, `'Profile'`, `'Admin'`) via `onNav`. The mapping to route screens lives in the App component.

### Theme

User's theme preference (`'light' | 'dark' | 'auto'`) lives in the user profile. When `'auto'`:
- On load, check `window.matchMedia('(prefers-color-scheme: dark)')`.
- Subscribe to `change` events — re-apply palette when the OS swaps.

On any theme or site-color change, the prototype calls `applyPalette(resolvePalette(theme, siteConfig))`, which mutates the active palette object and re-injects the stylesheet. In a real framework, switch to CSS custom properties set on `[data-theme]` on `html`.

### Overlays

Both modals (`Log task`, `Fuel fill-up`) sit on the App root as conditionally-mounted siblings of the current screen, with `position: absolute; inset: 0; z-index: 20`. Keyboard: `Esc` closes. Backdrop click does **not** close (data entry is easy to lose; require explicit cancel).

### Toasts

Success toasts center-bottom, dismiss after 2.4s. Single slot — new toasts replace previous.

### Persistence

The prototype persists:
- Route: `localStorage.wdw_route`
- User profile: `localStorage.wdw_user_profile`
- Site config: `localStorage.wdw_site_config`

In production, profile + site config should be server-side; route can remain client-side.

### Color picker

Native `<input type="color">` overlaid on a colored swatch; also a row of 6 preset swatches; also a hex-code chip (read-only display); also a "Reset to default" button visible only when overridden. On change, persist and re-apply palette.

### Avatar uploads

Image is read via FileReader, drawn to a `<canvas>` scaled to max 256×256, re-encoded as JPEG at 0.88 quality, and stored as a data URL. SVGs bypass the canvas and store as-is. 2 MB upload ceiling. In production, POST to backend, receive an asset URL, store the URL not the data.

### Responsive behavior

Desktop-first, works down to ~1100px before the card grid collapses. Below that, layouts will need a mobile design pass — not covered here. The fuel-fillup modal is the only flow intentionally sized for a tablet form factor.

---

## Data model — what the UI expects

These shapes are shown inline in the prototype (`window.FLEET`, `window.REMINDERS`, `window.EXPENSES`, `window.PARTS`, `window.INBOX_ITEMS`, etc.) and should inform your OpenAPI schema if it isn't final yet.

### Vehicle

```
id          string        uuid
name        string        display label, user-editable
make        string
model       string
year        int
vin         string
glyph       enum          'truck' | 'tractor' | 'atv' | 'trailer' | 'gen' | 'car' | 'track' | 'race'
odo         int           current odometer reading
unit        enum          'mi' | 'km' | 'hr' | 'events'
status      enum          'ok' | 'due-soon' | 'overdue' | 'idle'
dueIn       string        human-readable summary, computed from reminders
tags        string[]
photo       string?       URL
```

Money is stored as integer cents per `CLAUDE.md`. Convert for display.

### Reminder

```
id           string
vehicleId    string
title        string
kind         enum     'date' | 'odometer' | 'event'
intervalValue int     (months, miles, or events depending on kind)
lastDoneAt   ISO date
lastDoneOdo  int?
nextDueAt    ISO date
nextDueOdo   int?
status       enum     'ok' | 'due-soon' | 'overdue'
system       string   e.g. 'Engine', 'Brakes', 'Drivetrain'
```

### Task (work log)

```
id           string
vehicleId    string
taskType     enum   'oil_change' | 'service' | 'repair' | 'inspection' | 'fuel' | 'modification' | 'other'
title        string
loggedAt     ISO date
odoAt        int?
performedBy  string  (crew member name or 'outside shop')
notes        string  (markdown)
parts        Array<{ sku, name, qty, unitCost }>
laborHours   float
laborRate    int cents
photos       string[] (URLs)
totalCents   int
scheduleNext?: { intervalKind, intervalValue }
```

### Expense

```
id           string
vehicleId    string
date         ISO date
category     enum  'fuel' | 'maintenance' | 'insurance' | 'registration' | 'tax' | 'depreciation' | 'financing' | 'tolls' | 'parking' | 'track_fees' | 'tires' | 'other'
description  string
costCents    int
vendor       string
recurring    bool
```

### Part

```
sku            string
name           string
vendor         string
category       string
onHand         float
reorderPoint   float
lastUsedAt     ISO date?
unitCostCents  int
```

### InboxItem

```
id           string
source       enum   'imap' | 'upload' | 'webhook'
sender       string
subject      string
receivedAt   ISO date
attachments  Array<{ name, mime, size }>
parsed       {
  confidence:  float  0..1
  parser:      string  e.g. 'RockAuto', 'Generic PDF'
  vehicleGuess: string?  vehicleId
  vendor:       string?
  totalCents:   int?
  lineItems:    Array<{ sku?, name, qty?, unitCents? }>
}
status       enum   'unfiled' | 'routed' | 'rejected'
```

### SiteConfig

```
siteTitle     string     default 'FleetAware'
tagline       string     default 'Wheels Down Workshop'
primaryColor  string?    hex like '#1f7a7f'; null → palette default
accentColor   string?    hex like '#e85c2b'; null → palette default
logoUrl       string?
address       string
currency      enum       'USD' | 'CAD' | 'EUR' | 'GBP'
timezone      string     IANA zone
distanceUnit  enum       'mi' | 'km'
volumeUnit    enum       'gal' | 'L'
laborRateCents int
fuelTracking   bool
```

### UserProfile

```
id           string
name         string
email        string
initials     string (2 chars, uppercase)
avatarUrl    string?
theme        enum 'light' | 'dark' | 'auto'
density      enum 'comfortable' | 'compact'
isAdmin      bool
role         enum 'owner' | 'mechanic' | 'driver' | 'external'
defaultVehicleId string?
```

### Crew member

Attribution label, not a login. Crew members surface in the "Performed by" dropdown when logging tasks.

```
id           string
name         string
role         enum 'owner' | 'mechanic' | 'driver' | 'external'
tasksLogged  int   (computed)
```

Note: a crew member is distinct from a UserProfile. One person can be both — the UserProfile carries login+preferences, the Crew record carries attribution.

---

## Integrations

### Vendor receipt import (IMAP)

wdw-fleet is a client. Config stored on `SiteConfig`:

```
imapHost        string
imapPort        int      (default 993)
imapTLS         bool     (forced true per product decision — no plaintext)
imapUsername    string
imapPassword    string   (encrypted at rest)
imapFolder      string   (default 'INBOX')
```

Poll interval 10 minutes; honor IMAP IDLE if the server offers it. Mark messages `\Seen` after parse; do **not** delete. Each message becomes an `InboxItem`; attachments are extracted and stored.

**Parsers** live under `internal/webhook/parsers/` (per `CLAUDE.md` convention). Each parser implements:

```go
type Parser interface {
    Name() string
    Match(msg EmailMsg) bool      // e.g. by from-domain
    Parse(msg EmailMsg) (Parsed, error)
}
```

Parsers bundled: RockAuto, FCP Euro, Tire Rack, Summit Racing, ECS Tuning, Pelican Parts, Amazon, plus a `GenericPDF` fallback that grabs totals + line items with weaker confidence.

### Webhook

Outbound. Events: `task.logged`, `reminder.due`, `receipt.parsed`. Signed with HMAC-SHA256 using the site's API token.

### Not implemented (intentional non-goals)

- Any Google service (Gmail, Google Fonts, Maps, OAuth). Product decision: 100% IMAP/SSL; no Google anywhere.
- Telemetry integrations (OBD-Link, MyChron/AiM) — out of scope for v1.
- Apple Wallet — out of scope.
- Account billing, plan tiers, account deletion — not applicable. wdw-fleet is MIT-licensed, self-hosted, single-tenant.

---

## Assets

- **Vehicle glyphs** — custom SVGs defined inline in `VGlyph` component. 8 types; redraw as SVG components in your target stack. Stroke 1.6px, fill `currentColor` at 0.15 opacity, strokeLinejoin round.
- **Icons** — a lightweight set defined in the `I` object inline (plus, check, x, search, bell, camera, download). In production, use Lucide, Phosphor, or similar. Prototype icons are unstyled SVGs; size 14–16, stroke-width 1.7.
- **Logo** — top-left uses a base64-encoded PNG embedded in the prototype. **Replace with the actual Wheels Down Workshop logo** on real integration; site admins upload their own via Site & branding settings.
- **Photo placeholders** — `wdw-photo` class paints a subtle 12px checkerboard in `paperAlt` on `paper`, bordered with `line`. Used everywhere a photo would appear in the real app.

---

## Files in this bundle

- `README.md` — this document
- `FleetAware.html` — the interactive prototype. Open in any browser; ~2500 lines of React + inline Babel. All state is client-side. Localstorage keys: `wdw_route`, `wdw_user_profile`, `wdw_site_config`.

## Open questions for the developer

1. **Units conversion.** Site config says "miles", per-user override says "kilometers" — does the API serve raw values (always integer miles) and the client converts, or does the API return pre-converted values? The prototype assumes client-side conversion.
2. **Attachment storage.** Inline binary on the row (bad idea at scale), S3-compatible blob, or local filesystem under a configured path? The prototype treats attachments as opaque URLs.
3. **Crew vs UserProfile.** The prototype treats these as separate concepts; if the product would prefer a single "User" model with `isLoginUser: bool`, that's equally valid — but the "Performed by" dropdown must include non-login names (e.g. "Outside shop").
4. **Multi-tenant?** Prototype assumes single-tenant (one site = one garage). If multi-tenant is in scope, `SiteConfig` scopes to a tenant key, users belong to a tenant, etc. Not in the current design.
