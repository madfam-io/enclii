-- Migration 018: Database Add-ons (Rollback)

-- Drop triggers first
DROP TRIGGER IF EXISTS trigger_update_database_addon_bindings_updated_at ON database_addon_bindings;
DROP TRIGGER IF EXISTS trigger_update_database_addons_updated_at ON database_addons;
DROP FUNCTION IF EXISTS update_database_addons_updated_at();

-- Drop tables in reverse order (respecting foreign keys)
DROP TABLE IF EXISTS database_addon_backups;
DROP TABLE IF EXISTS database_addon_bindings;
DROP TABLE IF EXISTS database_addons;
