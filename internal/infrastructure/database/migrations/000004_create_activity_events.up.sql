-- migration: 000004_create_activity_events.up.sql
-- creates the activity_events table for tracking user signals
-- idempotent: uses IF NOT EXISTS and DO blocks

-- create enum type for event types (if not exists pattern for enums)
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'activity_event_type' AND typnamespace = (SELECT oid FROM pg_namespace WHERE nspname = 'pulse')) THEN
        CREATE TYPE pulse.activity_event_type AS ENUM (
            'view',
            'join',
            'leave',
            'post',
            'comment',
            'reaction',
            'share'
        );
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS pulse.activity_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    community_id UUID NOT NULL REFERENCES pulse.communities(id),
    user_id UUID REFERENCES pulse.users_profile(id),
    event_type pulse.activity_event_type NOT NULL,
    weight NUMERIC(5, 2) NOT NULL DEFAULT 1.0,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE pulse.activity_events IS 'append-only event log for all user activity signals';
COMMENT ON COLUMN pulse.activity_events.weight IS 'relative importance of this event type for momentum';
COMMENT ON COLUMN pulse.activity_events.metadata IS 'event-specific data, schema depends on event_type';

-- index for querying events by community (momentum calculation)
CREATE INDEX IF NOT EXISTS idx_activity_events_community_time 
    ON pulse.activity_events(community_id, created_at DESC);

-- index for querying events by user
CREATE INDEX IF NOT EXISTS idx_activity_events_user 
    ON pulse.activity_events(user_id) 
    WHERE user_id IS NOT NULL;

-- index for time-windowed queries (sliding window momentum)
CREATE INDEX IF NOT EXISTS idx_activity_events_created_at 
    ON pulse.activity_events(created_at DESC);

-- index for filtering by event type
CREATE INDEX IF NOT EXISTS idx_activity_events_type 
    ON pulse.activity_events(event_type, created_at DESC);
