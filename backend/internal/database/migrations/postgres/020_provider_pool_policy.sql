ALTER TABLE business_provider_groups
	ADD COLUMN IF NOT EXISTS tags TEXT NOT NULL DEFAULT '';

ALTER TABLE business_provider_groups
	ADD COLUMN IF NOT EXISTS match_mode TEXT NOT NULL DEFAULT 'fallback';

CREATE INDEX IF NOT EXISTS idx_business_provider_groups_policy
	ON business_provider_groups(platform, enabled, match_mode, priority, created_at);
