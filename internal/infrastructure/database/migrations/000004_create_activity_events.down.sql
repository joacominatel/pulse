-- migration: 000004_create_activity_events.down.sql
-- drops the activity_events table and related objects

DROP INDEX IF EXISTS pulse.idx_activity_events_type;
DROP INDEX IF EXISTS pulse.idx_activity_events_created_at;
DROP INDEX IF EXISTS pulse.idx_activity_events_user;
DROP INDEX IF EXISTS pulse.idx_activity_events_community_time;
DROP TABLE IF EXISTS pulse.activity_events;
DROP TYPE IF EXISTS pulse.activity_event_type;
