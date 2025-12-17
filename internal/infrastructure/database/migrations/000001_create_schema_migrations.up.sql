-- migration: 000001_create_schema_migrations.up.sql
-- creates the schema_migrations table to track applied migrations
-- idempotent: uses IF NOT EXISTS

CREATE SCHEMA IF NOT EXISTS pulse;

CREATE TABLE IF NOT EXISTS pulse.schema_migrations (
    version VARCHAR(14) PRIMARY KEY,
    description VARCHAR(255) NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE pulse.schema_migrations IS 'tracks applied database migrations';
