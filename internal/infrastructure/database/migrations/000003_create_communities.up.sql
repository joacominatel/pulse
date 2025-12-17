-- migration: 000003_create_communities.up.sql
-- creates the communities table
-- idempotent: uses IF NOT EXISTS

CREATE TABLE IF NOT EXISTS pulse.communities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    creator_id UUID NOT NULL REFERENCES pulse.users_profile(id),
    avatar_url TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    current_momentum NUMERIC(12, 4) NOT NULL DEFAULT 0,
    momentum_updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE pulse.communities IS 'thematic groupings for content and discussion';
COMMENT ON COLUMN pulse.communities.slug IS 'url-friendly unique identifier';
COMMENT ON COLUMN pulse.communities.current_momentum IS 'precomputed momentum score, updated by background job';

-- index for slug lookups (primary access pattern)
CREATE INDEX IF NOT EXISTS idx_communities_slug 
    ON pulse.communities(slug);

-- index for ranking by momentum
CREATE INDEX IF NOT EXISTS idx_communities_momentum 
    ON pulse.communities(current_momentum DESC) 
    WHERE is_active = true;

-- index for finding communities by creator
CREATE INDEX IF NOT EXISTS idx_communities_creator 
    ON pulse.communities(creator_id);
