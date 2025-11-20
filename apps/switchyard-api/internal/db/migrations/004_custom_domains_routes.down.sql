-- Migration 004 rollback: Remove custom domains and routes tables

DROP TABLE IF EXISTS routes;
DROP TABLE IF EXISTS custom_domains;

ALTER TABLE services DROP COLUMN IF EXISTS volumes;
