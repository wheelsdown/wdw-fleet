DROP TRIGGER IF EXISTS trg_webhooks_updated_at ON webhooks;
DROP TRIGGER IF EXISTS trg_reminders_updated_at ON reminders;
DROP TRIGGER IF EXISTS trg_expenses_updated_at ON expenses;
DROP TRIGGER IF EXISTS trg_parts_updated_at ON parts;
DROP TRIGGER IF EXISTS trg_service_records_updated_at ON service_records;
DROP TRIGGER IF EXISTS trg_fuel_logs_updated_at ON fuel_logs;
DROP TRIGGER IF EXISTS trg_vehicles_updated_at ON vehicles;

DROP FUNCTION IF EXISTS update_updated_at();

DROP TABLE IF EXISTS webhooks;
DROP TABLE IF EXISTS reminders;
DROP TABLE IF EXISTS expenses;
DROP TABLE IF EXISTS service_record_parts;
DROP TABLE IF EXISTS parts;
DROP TABLE IF EXISTS service_records;
DROP TABLE IF EXISTS fuel_logs;
DROP TABLE IF EXISTS vehicles;
