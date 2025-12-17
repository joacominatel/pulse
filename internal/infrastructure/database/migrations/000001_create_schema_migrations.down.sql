-- migration: 000001_create_schema_migrations.down.sql
-- drops the schema_migrations table
-- caution: this loses all migration history

DROP TABLE IF EXISTS pulse.schema_migrations;
