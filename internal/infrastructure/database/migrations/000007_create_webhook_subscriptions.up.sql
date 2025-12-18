-- migration: 000007_create_webhook_subscriptions.up.sql
-- creates the webhook_subscriptions table for momentum spike notifications
-- idempotent: uses IF NOT EXISTS

CREATE TABLE IF NOT EXISTS pulse.webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES pulse.users_profile(id) ON DELETE CASCADE,
    community_id UUID NOT NULL REFERENCES pulse.communities(id) ON DELETE CASCADE,
    target_url TEXT NOT NULL,
    secret TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    -- users can only have one subscription per community
    CONSTRAINT unique_user_community_subscription UNIQUE(user_id, community_id)
);

COMMENT ON TABLE pulse.webhook_subscriptions IS 'webhook endpoints for momentum spike notifications';
COMMENT ON COLUMN pulse.webhook_subscriptions.target_url IS 'url to POST notifications to';
COMMENT ON COLUMN pulse.webhook_subscriptions.secret IS 'hmac secret for signing payloads';
COMMENT ON COLUMN pulse.webhook_subscriptions.is_active IS 'soft disable without deletion';

-- index for looking up subscriptions by community (used when broadcasting spikes)
CREATE INDEX IF NOT EXISTS idx_webhook_subscriptions_community 
    ON pulse.webhook_subscriptions(community_id) 
    WHERE is_active = true;

-- index for user management of their subscriptions
CREATE INDEX IF NOT EXISTS idx_webhook_subscriptions_user 
    ON pulse.webhook_subscriptions(user_id);
