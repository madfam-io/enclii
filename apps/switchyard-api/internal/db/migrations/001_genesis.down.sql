-- Enclii Genesis Schema Rollback
-- WARNING: This will destroy ALL data in the database
-- Only use for complete database reset

DROP SCHEMA public CASCADE;
CREATE SCHEMA public;

-- Grant default privileges
GRANT ALL ON SCHEMA public TO PUBLIC;
