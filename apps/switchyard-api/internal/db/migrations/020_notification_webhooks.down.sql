-- Migration 020: Notification Webhooks (Rollback)

DROP TRIGGER IF EXISTS trigger_update_webhook_destinations_updated_at ON webhook_destinations;
DROP FUNCTION IF EXISTS update_webhook_destinations_updated_at();

DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_destinations;
