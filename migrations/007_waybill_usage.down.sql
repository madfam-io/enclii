-- Rollback Waybill usage tracking tables

DROP VIEW IF EXISTS project_usage_summary;
DROP TRIGGER IF EXISTS pricing_plans_updated_at ON pricing_plans;
DROP TRIGGER IF EXISTS billing_records_updated_at ON billing_records;
DROP TRIGGER IF EXISTS subscriptions_updated_at ON subscriptions;
DROP FUNCTION IF EXISTS update_waybill_updated_at();
DROP TABLE IF EXISTS usage_alerts;
DROP TABLE IF EXISTS credits;
DROP TABLE IF EXISTS billing_records;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS pricing_plans;
DROP TABLE IF EXISTS daily_usage;
DROP TABLE IF EXISTS hourly_usage;
DROP TABLE IF EXISTS usage_events;
