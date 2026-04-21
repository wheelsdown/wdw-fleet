-- FleetAware / wdw-fleet initial schema.
--
-- Conventions:
--   * UUIDs for all primary keys of user-facing entities.
--   * Money stored as integer cents. Column names end in `_cents` to make it
--     self-documenting at query time.
--   * Timestamps are `timestamptz` (never naive `timestamp`) so the IANA
--     timezone on site_config can drive display conversion.
--   * User-editable primary entities carry `deleted_at timestamptz` for soft
--     delete. Reference tables (sessions, task_parts, attachments,
--     webhook_deliveries) hard-delete.
--   * Enums are implemented as `text` + CHECK constraints. This stays
--     migration-friendly (no DROP TYPE dance to widen an enum later).

CREATE EXTENSION IF NOT EXISTS "pgcrypto"; -- gen_random_uuid
CREATE EXTENSION IF NOT EXISTS "citext";   -- case-insensitive email

-- ---------------------------------------------------------------------------
-- Shared trigger: bump updated_at on row mutation.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- site_config  (singleton row; id = 1 enforced)
-- ===========================================================================
CREATE TABLE site_config (
    id                      INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),

    -- Branding
    site_title              TEXT NOT NULL DEFAULT 'FleetAware',
    tagline                 TEXT NOT NULL DEFAULT 'Wheels Down Workshop',
    logo_url                TEXT,
    primary_color           TEXT CHECK (primary_color ~ '^#[0-9a-fA-F]{6}$'),
    accent_color            TEXT CHECK (accent_color  ~ '^#[0-9a-fA-F]{6}$'),
    address                 TEXT NOT NULL DEFAULT '',

    -- Regional
    currency                CHAR(3) NOT NULL DEFAULT 'USD',  -- ISO 4217
    timezone                TEXT    NOT NULL DEFAULT 'UTC',  -- IANA

    -- Units
    distance_unit           TEXT NOT NULL DEFAULT 'mi' CHECK (distance_unit IN ('mi','km')),
    volume_unit             TEXT NOT NULL DEFAULT 'gal' CHECK (volume_unit IN ('gal','L')),

    -- Pricing defaults
    labor_rate_cents        INT  NOT NULL DEFAULT 0,
    fuel_tracking_enabled   BOOL NOT NULL DEFAULT TRUE,
    parts_markup_enabled    BOOL NOT NULL DEFAULT FALSE,

    -- Avatar resolution. When a user has no uploaded avatar_url:
    --   * libravatar_enabled=TRUE  -> derive from libravatar (federated,
    --     open source, email-hash based; no Automattic involvement)
    --   * libravatar_gravatar_fallback=TRUE -> if libravatar has no record,
    --     tell libravatar to fall back to Gravatar (requires opt-in because
    --     Gravatar is proprietary and tracks IP addresses)
    --   * both FALSE -> render initials only (no email hash leaves the server)
    libravatar_enabled              BOOL NOT NULL DEFAULT TRUE,
    libravatar_gravatar_fallback    BOOL NOT NULL DEFAULT FALSE,

    -- IMAP receipt ingestion (all optional; only used if imap_enabled)
    imap_enabled            BOOL   NOT NULL DEFAULT FALSE,
    imap_host               TEXT   NOT NULL DEFAULT '',
    imap_port               INT    NOT NULL DEFAULT 993,
    imap_tls                BOOL   NOT NULL DEFAULT TRUE,
    imap_username           TEXT   NOT NULL DEFAULT '',
    imap_password_encrypted BYTEA,                       -- encrypted at rest
    imap_folder             TEXT   NOT NULL DEFAULT 'INBOX',

    -- API access
    api_token_hash          TEXT,                        -- sha256 of current token

    -- Setup completion marker (used by onboarding wizard)
    setup_completed_at      TIMESTAMPTZ,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed the singleton. Onboarding wizard updates; it does not insert.
INSERT INTO site_config (id) VALUES (1);

CREATE TRIGGER trg_site_config_updated_at
    BEFORE UPDATE ON site_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ===========================================================================
-- users  (login accounts)
-- ===========================================================================
CREATE TABLE users (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email                   CITEXT UNIQUE NOT NULL,
    password_hash           TEXT NOT NULL,               -- argon2id
    name                    TEXT NOT NULL,
    initials                TEXT NOT NULL CHECK (char_length(initials) BETWEEN 1 AND 2),
    avatar_url              TEXT,

    theme                   TEXT NOT NULL DEFAULT 'auto'
                                CHECK (theme IN ('light','dark','auto')),
    density                 TEXT NOT NULL DEFAULT 'comfortable'
                                CHECK (density IN ('comfortable','compact')),

    default_vehicle_id      UUID,                        -- FK added after vehicles table exists
    distance_unit_override  TEXT CHECK (distance_unit_override IN ('mi','km')),
    digest_email            BOOL NOT NULL DEFAULT TRUE,
    keyboard_shortcuts      BOOL NOT NULL DEFAULT TRUE,

    is_admin                BOOL NOT NULL DEFAULT FALSE,
    role                    TEXT NOT NULL DEFAULT 'owner'
                                CHECK (role IN ('owner','mechanic','driver','external')),

    last_sign_in_at         TIMESTAMPTZ,
    deleted_at              TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_users_email_live ON users (email) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ===========================================================================
-- sessions  (auth cookies; store sha256 of the raw token only)
-- ===========================================================================
CREATE TABLE sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT UNIQUE NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    user_agent  TEXT,
    ip          INET,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id    ON sessions (user_id);
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);

-- ===========================================================================
-- crew_members  (attribution labels; may or may not map to a user)
-- ===========================================================================
CREATE TABLE crew_members (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    role            TEXT NOT NULL DEFAULT 'owner'
                        CHECK (role IN ('owner','mechanic','driver','external')),
    linked_user_id  UUID REFERENCES users(id) ON DELETE SET NULL,
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_crew_members_linked_user ON crew_members (linked_user_id);

CREATE TRIGGER trg_crew_members_updated_at
    BEFORE UPDATE ON crew_members FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ===========================================================================
-- vehicles
-- ===========================================================================
CREATE TABLE vehicles (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                    TEXT NOT NULL,
    make                    TEXT NOT NULL DEFAULT '',
    model                   TEXT NOT NULL DEFAULT '',
    year                    INT,
    vin                     TEXT NOT NULL DEFAULT '',
    license_plate           TEXT NOT NULL DEFAULT '',

    -- UI decoration
    glyph                   TEXT NOT NULL DEFAULT 'car'
                                CHECK (glyph IN ('truck','tractor','atv','trailer','gen','car','track','race')),

    -- Primary counter. For mi/km/hr vehicles this is cumulative; for 'events'
    -- it's a monotonic count of track events.
    odometer                INT  NOT NULL DEFAULT 0,
    odometer_unit           TEXT NOT NULL DEFAULT 'mi'
                                CHECK (odometer_unit IN ('mi','km','hr','events')),

    -- Lifecycle
    status                  TEXT NOT NULL DEFAULT 'active'
                                CHECK (status IN ('active','inactive','sold')),
    acquisition_date        DATE,
    acquisition_cost_cents  INT,
    sale_date               DATE,
    sale_price_cents        INT,

    notes                   TEXT NOT NULL DEFAULT '',
    custom_fields           JSONB NOT NULL DEFAULT '{}'::jsonb,
    photo_url               TEXT NOT NULL DEFAULT '',
    tags                    TEXT[] NOT NULL DEFAULT '{}',

    deleted_at              TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vehicles_status_live ON vehicles (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_vehicles_tags_gin    ON vehicles USING GIN (tags);

CREATE TRIGGER trg_vehicles_updated_at
    BEFORE UPDATE ON vehicles FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Backfill FK: users.default_vehicle_id -> vehicles.id
ALTER TABLE users
    ADD CONSTRAINT fk_users_default_vehicle
    FOREIGN KEY (default_vehicle_id) REFERENCES vehicles(id) ON DELETE SET NULL;

-- ===========================================================================
-- track_events
--
-- First-class entity for race / HPDE / autocross / track-day sessions. Fuel
-- burn per event and task-to-event attribution are common queries, and track
-- events carry metadata tasks can't accommodate (venue, lap count, best lap,
-- tire compound, weather).
--
-- Reminders with kind='event' count rows in this table for the vehicle since
-- last_done_at.
-- ===========================================================================
CREATE TABLE track_events (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id          UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,

    event_date          DATE NOT NULL,
    end_date            DATE,                           -- multi-day events
    venue               TEXT NOT NULL,                  -- 'Road Atlanta','Sebring',...
    series              TEXT NOT NULL DEFAULT '',       -- 'NASA','SCCA','HPDE',...
    session_type        TEXT NOT NULL DEFAULT ''
                            CHECK (session_type IN ('','practice','qualifying','race','hpde','autocross','test','other')),
    run_group           TEXT NOT NULL DEFAULT '',       -- 'A','B','Advanced',...
    class               TEXT NOT NULL DEFAULT '',       -- car class / category

    odometer_start      INT,
    odometer_end        INT,
    session_count       INT NOT NULL DEFAULT 0,         -- sessions run (tire/brake/engine wear)
    lap_count           INT NOT NULL DEFAULT 0,         -- laps completed (wear metric)

    weather             TEXT NOT NULL DEFAULT '',
    tire_compound       TEXT NOT NULL DEFAULT '',
    tire_pressures      JSONB,                          -- free-form: {f:32,r:30} or per-corner
    conditions_notes    TEXT NOT NULL DEFAULT '',

    entry_fee_cents     INT,
    notes               TEXT NOT NULL DEFAULT '',

    deleted_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_track_events_vehicle_date
    ON track_events (vehicle_id, event_date DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_track_events_venue
    ON track_events (venue) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_track_events_updated_at
    BEFORE UPDATE ON track_events FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ===========================================================================
-- parts  (catalog + inventory)
-- ===========================================================================
CREATE TABLE parts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sku              TEXT,
    name             TEXT NOT NULL,
    manufacturer     TEXT NOT NULL DEFAULT '',
    vendor           TEXT NOT NULL DEFAULT '',
    category         TEXT NOT NULL DEFAULT '',

    on_hand          NUMERIC(12,3) NOT NULL DEFAULT 0,
    reorder_point    NUMERIC(12,3),
    unit_cost_cents  INT,
    last_used_at     TIMESTAMPTZ,

    notes            TEXT NOT NULL DEFAULT '',

    deleted_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_parts_sku_live ON parts (sku)
    WHERE sku IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_parts_vendor_live   ON parts (vendor)   WHERE deleted_at IS NULL;
CREATE INDEX idx_parts_category_live ON parts (category) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_parts_updated_at
    BEFORE UPDATE ON parts FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ===========================================================================
-- inbox_items  (IMAP / upload / webhook intake)  -- forward-declared so tasks
-- and expenses can reference it via FK.
-- ===========================================================================
CREATE TABLE inbox_items (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source               TEXT NOT NULL CHECK (source IN ('imap','upload','webhook')),
    sender               TEXT NOT NULL DEFAULT '',
    subject              TEXT NOT NULL DEFAULT '',
    received_at          TIMESTAMPTZ NOT NULL,

    raw_path             TEXT,                 -- blob-store path to raw message
    external_message_id  TEXT,                 -- IMAP Message-ID for dedup

    parsed               JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- Expected parsed keys: confidence, parser, vehicle_guess_id, vendor,
    -- total_cents, line_items[]

    status               TEXT NOT NULL DEFAULT 'unfiled'
                             CHECK (status IN ('unfiled','routed','rejected')),
    routed_task_id       UUID,                 -- FK added after tasks exists
    routed_expense_id    UUID,                 -- FK added after expenses exists
    rejected_at          TIMESTAMPTZ,

    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_inbox_external_msg_id
    ON inbox_items (external_message_id)
    WHERE external_message_id IS NOT NULL;
CREATE INDEX idx_inbox_status  ON inbox_items (status);
CREATE INDEX idx_inbox_source  ON inbox_items (source);

CREATE TRIGGER trg_inbox_items_updated_at
    BEFORE UPDATE ON inbox_items FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ===========================================================================
-- tasks  (unified work log: service, repair, inspection, modification, note,
--         other. NOT fuel -- fuel_logs. NOT track events -- track_events.)
-- ===========================================================================
CREATE TABLE tasks (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id              UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,

    task_type               TEXT NOT NULL
                                CHECK (task_type IN (
                                    'oil_change','service','repair','inspection',
                                    'modification','note','other'
                                )),
    title                   TEXT NOT NULL,
    notes                   TEXT NOT NULL DEFAULT '',   -- markdown

    logged_at               TIMESTAMPTZ NOT NULL,
    odometer_at             INT,

    -- Attribution: either a crew_member (structured) or a free-text label
    -- like "Outside shop". performed_by_label wins if both set.
    performed_by_crew_id    UUID REFERENCES crew_members(id) ON DELETE SET NULL,
    performed_by_label      TEXT NOT NULL DEFAULT '',

    labor_hours             NUMERIC(6,2) NOT NULL DEFAULT 0,
    labor_rate_cents        INT NOT NULL DEFAULT 0,     -- snapshot at log time

    -- Optional provenance / association
    inbox_item_id           UUID REFERENCES inbox_items(id)  ON DELETE SET NULL,
    track_event_id          UUID REFERENCES track_events(id) ON DELETE SET NULL,

    deleted_at              TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tasks_vehicle_logged ON tasks (vehicle_id, logged_at DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_tasks_vehicle_type_logged ON tasks (vehicle_id, task_type, logged_at DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_tasks_inbox_item   ON tasks (inbox_item_id)  WHERE inbox_item_id  IS NOT NULL;
CREATE INDEX idx_tasks_track_event  ON tasks (track_event_id) WHERE track_event_id IS NOT NULL;

CREATE TRIGGER trg_tasks_updated_at
    BEFORE UPDATE ON tasks FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Backfill FK on inbox_items.routed_task_id
ALTER TABLE inbox_items
    ADD CONSTRAINT fk_inbox_routed_task
    FOREIGN KEY (routed_task_id) REFERENCES tasks(id) ON DELETE SET NULL;

-- ===========================================================================
-- task_parts  (parts used on a task; captures unit cost at use time)
-- ===========================================================================
CREATE TABLE task_parts (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id           UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    part_id           UUID REFERENCES parts(id) ON DELETE SET NULL,

    -- Ad-hoc entries (part_id NULL) carry their own sku/name:
    sku_override      TEXT,
    name              TEXT NOT NULL,

    qty               NUMERIC(12,3) NOT NULL DEFAULT 1,
    unit_cost_cents   INT NOT NULL,

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_parts_task ON task_parts (task_id);
CREATE INDEX idx_task_parts_part ON task_parts (part_id) WHERE part_id IS NOT NULL;

-- ===========================================================================
-- fuel_logs  (Road Trip HD-compatible superset; single-currency, USD-normalized)
-- ===========================================================================
CREATE TABLE fuel_logs (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id              UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,

    filled_at               TIMESTAMPTZ NOT NULL,
    odometer                INT,                          -- primary counter
    trip_meter              INT,                          -- precedence over odometer delta

    volume                  NUMERIC(10,3) NOT NULL,
    volume_unit             TEXT NOT NULL DEFAULT 'gal'
                                CHECK (volume_unit IN ('gal','L','kWh')),

    price_per_unit_cents    INT,                          -- derivable from total/volume
    total_cents             INT NOT NULL,

    -- Economy-window controls (per Road Trip HD semantics)
    full_tank               BOOL NOT NULL DEFAULT TRUE,
    missed_fill             BOOL NOT NULL DEFAULT FALSE,  -- "Reset" flag

    -- Classification
    fuel_type               TEXT NOT NULL DEFAULT '',     -- '87','93','Diesel','E85',...
    conditions              TEXT[] NOT NULL DEFAULT '{}', -- 'city','highway','winter'
    payment                 TEXT NOT NULL DEFAULT '',
    categories              TEXT[] NOT NULL DEFAULT '{}', -- free-form tags

    -- Location
    location_name           TEXT NOT NULL DEFAULT '',
    latitude                NUMERIC(9,6),
    longitude               NUMERIC(9,6),

    -- Trip computer (optional OBD/dashboard readings)
    trip_comp_econ          NUMERIC(8,3),
    trip_comp_speed         NUMERIC(8,3),
    trip_comp_temp          NUMERIC(6,2),
    trip_comp_time_minutes  INT,

    -- Optional attribution: this fill was burned at / during a track event
    track_event_id          UUID REFERENCES track_events(id) ON DELETE SET NULL,

    photo_url               TEXT NOT NULL DEFAULT '',
    notes                   TEXT NOT NULL DEFAULT '',

    -- Import bookkeeping
    import_source           TEXT,                         -- 'roadtrip_csv','manual','ocr'
    import_row_hash         TEXT,                         -- sha256 of source row

    deleted_at              TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fuel_logs_vehicle_filled
    ON fuel_logs (vehicle_id, filled_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_fuel_logs_vehicle_odo
    ON fuel_logs (vehicle_id, odometer) WHERE deleted_at IS NULL;
CREATE INDEX idx_fuel_logs_track_event
    ON fuel_logs (track_event_id) WHERE track_event_id IS NOT NULL;
CREATE UNIQUE INDEX idx_fuel_logs_import_dedup
    ON fuel_logs (vehicle_id, import_source, import_row_hash)
    WHERE import_row_hash IS NOT NULL;

CREATE TRIGGER trg_fuel_logs_updated_at
    BEFORE UPDATE ON fuel_logs FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ===========================================================================
-- expenses  (non-fuel, non-service costs; service costs live on tasks)
-- ===========================================================================
CREATE TABLE expenses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id      UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    date            DATE NOT NULL,
    category        TEXT NOT NULL CHECK (category IN (
                        'fuel','maintenance','insurance','registration',
                        'tax','depreciation','financing','tolls','parking',
                        'track_fees','tires','other'
                    )),
    description     TEXT NOT NULL DEFAULT '',
    cost_cents      INT NOT NULL,
    vendor          TEXT NOT NULL DEFAULT '',
    recurring       BOOL NOT NULL DEFAULT FALSE,
    notes           TEXT NOT NULL DEFAULT '',

    inbox_item_id   UUID REFERENCES inbox_items(id) ON DELETE SET NULL,

    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_expenses_vehicle_date
    ON expenses (vehicle_id, date DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_expenses_vehicle_category
    ON expenses (vehicle_id, category) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_expenses_updated_at
    BEFORE UPDATE ON expenses FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Backfill FK on inbox_items.routed_expense_id
ALTER TABLE inbox_items
    ADD CONSTRAINT fk_inbox_routed_expense
    FOREIGN KEY (routed_expense_id) REFERENCES expenses(id) ON DELETE SET NULL;

-- ===========================================================================
-- reminders
--   kind='date'     -> interval_value is months
--   kind='odometer' -> interval_value is in vehicle's odometer_unit (mi/km/hr)
--   kind='event'    -> interval_value counts track_events rows for the vehicle
--                      since last_done_at
-- ===========================================================================
CREATE TABLE reminders (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id           UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,

    title                TEXT NOT NULL,
    kind                 TEXT NOT NULL CHECK (kind IN ('date','odometer','event')),
    interval_value       INT  NOT NULL CHECK (interval_value > 0),
    system               TEXT NOT NULL DEFAULT '',   -- 'Engine','Brakes',...

    last_done_at         TIMESTAMPTZ,
    last_done_odometer   INT,
    last_done_task_id    UUID REFERENCES tasks(id) ON DELETE SET NULL,

    next_due_at          TIMESTAMPTZ,
    next_due_odometer    INT,

    status               TEXT NOT NULL DEFAULT 'pending'
                             CHECK (status IN ('pending','due-soon','overdue','dismissed','completed')),
    notes                TEXT NOT NULL DEFAULT '',

    deleted_at           TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reminders_vehicle ON reminders (vehicle_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_reminders_status  ON reminders (status)     WHERE deleted_at IS NULL;

CREATE TRIGGER trg_reminders_updated_at
    BEFORE UPDATE ON reminders FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ===========================================================================
-- attachments  (polymorphic; parent_kind + parent_id)
-- ===========================================================================
CREATE TABLE attachments (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_kind            TEXT NOT NULL CHECK (parent_kind IN (
                               'vehicle','task','fuel_log','expense',
                               'inbox_item','track_event'
                           )),
    parent_id              UUID NOT NULL,

    filename               TEXT NOT NULL,
    mime_type              TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes             BIGINT NOT NULL,
    sha256                 TEXT NOT NULL,
    storage_path           TEXT NOT NULL,              -- relative to blob root

    uploaded_by_user_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_attachments_parent ON attachments (parent_kind, parent_id);
CREATE INDEX idx_attachments_sha256 ON attachments (sha256);

-- ===========================================================================
-- webhooks  (outbound notifications)
-- ===========================================================================
CREATE TABLE webhooks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url         TEXT NOT NULL,
    events      TEXT[] NOT NULL DEFAULT '{}',
    secret      TEXT NOT NULL DEFAULT '',               -- HMAC-SHA256 signing key
    active      BOOL NOT NULL DEFAULT TRUE,
    deleted_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trg_webhooks_updated_at
    BEFORE UPDATE ON webhooks FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ===========================================================================
-- webhook_deliveries  (outbound queue w/ retry)
-- ===========================================================================
CREATE TABLE webhook_deliveries (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id        UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type        TEXT NOT NULL,
    payload           JSONB NOT NULL,

    status            TEXT NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending','sent','failed')),
    response_status   INT,
    response_body     TEXT,
    attempt_count     INT NOT NULL DEFAULT 0,

    next_attempt_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_deliveries_pending
    ON webhook_deliveries (next_attempt_at)
    WHERE status = 'pending';
CREATE INDEX idx_webhook_deliveries_webhook
    ON webhook_deliveries (webhook_id, created_at DESC);
