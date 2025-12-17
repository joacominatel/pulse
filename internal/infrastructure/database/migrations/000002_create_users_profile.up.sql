-- migration: 000002_create_users_profile.up.sql
-- creates the users_profile table
-- idempotent: uses IF NOT EXISTS

CREATE TABLE IF NOT EXISTS pulse.users_profile (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(50) UNIQUE NOT NULL,
    display_name VARCHAR(100),
    avatar_url TEXT,
    bio TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE pulse.users_profile IS 'user profiles for pulse, linked to external auth system';
COMMENT ON COLUMN pulse.users_profile.external_id IS 'identifier from external auth provider';

-- index for looking up users by external_id (common auth flow)
CREATE INDEX IF NOT EXISTS idx_users_profile_external_id 
    ON pulse.users_profile(external_id);

-- index for username lookups
CREATE INDEX IF NOT EXISTS idx_users_profile_username 
    ON pulse.users_profile(username);
