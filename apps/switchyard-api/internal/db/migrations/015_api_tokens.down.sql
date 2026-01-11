-- Migration 015: API Tokens (Rollback)

DROP TRIGGER IF EXISTS trigger_update_api_tokens_updated_at ON api_tokens;
DROP FUNCTION IF EXISTS update_api_tokens_updated_at();
DROP TABLE IF EXISTS api_tokens;
