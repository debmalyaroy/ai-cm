-- =============================================================================
-- AI-CM: Migration — Add priority/expected_impact to action_log, add user_preferences
-- Run this on existing installations that already have 01-schema.sql applied.
-- =============================================================================

ALTER TABLE action_log ADD COLUMN IF NOT EXISTS priority VARCHAR(10) DEFAULT 'medium';
ALTER TABLE action_log ADD COLUMN IF NOT EXISTS expected_impact TEXT DEFAULT '';

CREATE TABLE IF NOT EXISTS user_preferences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(100) NOT NULL DEFAULT 'demo_user',
    key VARCHAR(100) NOT NULL,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, key)
);

-- Backfill priorities for existing action_log rows based on action_type
UPDATE action_log SET priority = 'high'   WHERE action_type IN ('restock', 'price_match') AND priority = 'medium';
UPDATE action_log SET priority = 'medium' WHERE action_type = 'promotion' AND priority = 'medium';
UPDATE action_log SET priority = 'low'    WHERE action_type = 'delist' AND priority = 'medium';
