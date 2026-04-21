-- Core domain tables for wdw-fleet

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Vehicles
CREATE TABLE vehicles (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL,
    make          TEXT NOT NULL DEFAULT '',
    model         TEXT NOT NULL DEFAULT '',
    year          INT,
    vin           TEXT NOT NULL DEFAULT '',
    license_plate TEXT NOT NULL DEFAULT '',
    odometer      INT NOT NULL DEFAULT 0,
    odometer_unit TEXT NOT NULL DEFAULT 'mi' CHECK (odometer_unit IN ('mi', 'km')),
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'sold')),
    acquisition_date DATE,
    acquisition_cost INT,  -- cents
    sale_date    DATE,
    sale_price   INT,      -- cents
    notes        TEXT NOT NULL DEFAULT '',
    custom_fields JSONB NOT NULL DEFAULT '{}',
    photo_url    TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vehicles_status ON vehicles (status);

-- Fuel logs
CREATE TABLE fuel_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id  UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    date        DATE NOT NULL,
    odometer    INT NOT NULL,
    volume      NUMERIC(10,3) NOT NULL,
    volume_unit TEXT NOT NULL DEFAULT 'gal' CHECK (volume_unit IN ('gal', 'L')),
    cost        INT NOT NULL,  -- cents
    full_tank   BOOLEAN NOT NULL DEFAULT true,
    missed_fill BOOLEAN NOT NULL DEFAULT false,
    octane      TEXT NOT NULL DEFAULT '',
    station     TEXT NOT NULL DEFAULT '',
    notes       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fuel_logs_vehicle_id ON fuel_logs (vehicle_id);
CREATE INDEX idx_fuel_logs_date ON fuel_logs (vehicle_id, date);
CREATE INDEX idx_fuel_logs_odometer ON fuel_logs (vehicle_id, odometer);

-- Service records
CREATE TABLE service_records (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id  UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    date        DATE NOT NULL,
    odometer    INT,
    description TEXT NOT NULL,
    category    TEXT NOT NULL DEFAULT '',
    cost        INT NOT NULL DEFAULT 0,  -- cents
    vendor      TEXT NOT NULL DEFAULT '',
    notes       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_service_records_vehicle_id ON service_records (vehicle_id);
CREATE INDEX idx_service_records_date ON service_records (vehicle_id, date);

-- Parts
CREATE TABLE parts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT NOT NULL,
    part_number  TEXT NOT NULL DEFAULT '',
    manufacturer TEXT NOT NULL DEFAULT '',
    cost         INT,  -- cents
    vendor       TEXT NOT NULL DEFAULT '',
    notes        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Join table: service records <-> parts
CREATE TABLE service_record_parts (
    service_record_id UUID NOT NULL REFERENCES service_records(id) ON DELETE CASCADE,
    part_id           UUID NOT NULL REFERENCES parts(id) ON DELETE CASCADE,
    quantity          INT NOT NULL DEFAULT 1,
    PRIMARY KEY (service_record_id, part_id)
);

-- Expenses (non-fuel, non-service costs)
CREATE TABLE expenses (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id  UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    date        DATE NOT NULL,
    category    TEXT NOT NULL CHECK (category IN (
        'fuel', 'maintenance', 'insurance', 'registration',
        'tax', 'depreciation', 'financing', 'tolls', 'parking', 'other'
    )),
    description TEXT NOT NULL DEFAULT '',
    cost        INT NOT NULL,  -- cents
    vendor      TEXT NOT NULL DEFAULT '',
    recurring   BOOLEAN NOT NULL DEFAULT false,
    notes       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_expenses_vehicle_id ON expenses (vehicle_id);
CREATE INDEX idx_expenses_date ON expenses (vehicle_id, date);
CREATE INDEX idx_expenses_category ON expenses (vehicle_id, category);

-- Reminders
CREATE TABLE reminders (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id               UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    name                     TEXT NOT NULL,
    type                     TEXT NOT NULL CHECK (type IN ('date', 'odometer', 'both')),
    due_date                 DATE,
    due_odometer             INT,
    repeat_interval_days     INT,
    repeat_interval_distance INT,
    status                   TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'overdue', 'dismissed', 'completed')),
    notes                    TEXT NOT NULL DEFAULT '',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reminders_vehicle_id ON reminders (vehicle_id);
CREATE INDEX idx_reminders_status ON reminders (status);

-- Webhooks
CREATE TABLE webhooks (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url        TEXT NOT NULL,
    events     TEXT[] NOT NULL DEFAULT '{}',
    secret     TEXT NOT NULL DEFAULT '',
    active     BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply updated_at triggers
CREATE TRIGGER trg_vehicles_updated_at BEFORE UPDATE ON vehicles FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_fuel_logs_updated_at BEFORE UPDATE ON fuel_logs FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_service_records_updated_at BEFORE UPDATE ON service_records FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_parts_updated_at BEFORE UPDATE ON parts FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_expenses_updated_at BEFORE UPDATE ON expenses FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_reminders_updated_at BEFORE UPDATE ON reminders FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_webhooks_updated_at BEFORE UPDATE ON webhooks FOR EACH ROW EXECUTE FUNCTION update_updated_at();
