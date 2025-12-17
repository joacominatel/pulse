-- migration: 000002_create_users_profile.down.sql
-- drops the users_profile table and related objects

DROP INDEX IF EXISTS pulse.idx_users_profile_username;
DROP INDEX IF EXISTS pulse.idx_users_profile_external_id;
DROP TABLE IF EXISTS pulse.users_profile;
