-- Drop in reverse dependency order. CASCADE is needed because inbox_items has
-- a circular FK relationship with tasks and expenses (each side references
-- the other), and a few tables share polymorphic-style references that
-- resist a clean topological drop order.

DROP TABLE IF EXISTS webhook_deliveries CASCADE;
DROP TABLE IF EXISTS webhooks           CASCADE;
DROP TABLE IF EXISTS attachments        CASCADE;
DROP TABLE IF EXISTS reminders          CASCADE;
DROP TABLE IF EXISTS expenses           CASCADE;
DROP TABLE IF EXISTS fuel_logs          CASCADE;
DROP TABLE IF EXISTS task_parts         CASCADE;
DROP TABLE IF EXISTS tasks              CASCADE;
DROP TABLE IF EXISTS inbox_items        CASCADE;
DROP TABLE IF EXISTS parts              CASCADE;
DROP TABLE IF EXISTS track_events       CASCADE;
DROP TABLE IF EXISTS vehicles           CASCADE;
DROP TABLE IF EXISTS crew_members       CASCADE;
DROP TABLE IF EXISTS sessions           CASCADE;
DROP TABLE IF EXISTS users              CASCADE;
DROP TABLE IF EXISTS site_config        CASCADE;

DROP FUNCTION IF EXISTS update_updated_at();

-- Extensions left in place; they're cheap and may be used elsewhere.
