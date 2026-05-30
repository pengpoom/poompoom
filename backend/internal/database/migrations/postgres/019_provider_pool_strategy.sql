ALTER TABLE business_provider_members
	ADD COLUMN IF NOT EXISTS weight INTEGER NOT NULL DEFAULT 1;

ALTER TABLE business_provider_members
	ADD COLUMN IF NOT EXISTS max_concurrent INTEGER NOT NULL DEFAULT 0;

ALTER TABLE business_provider_members
	ADD COLUMN IF NOT EXISTS cooldown_seconds INTEGER NOT NULL DEFAULT 120;

ALTER TABLE business_provider_members
	ADD COLUMN IF NOT EXISTS failure_threshold INTEGER NOT NULL DEFAULT 5;

ALTER TABLE business_provider_members
	ADD COLUMN IF NOT EXISTS consecutive_failures INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_business_provider_members_strategy
	ON business_provider_members(platform, enabled, status, max_concurrent, weight, priority, last_used_at);

CREATE INDEX IF NOT EXISTS idx_business_image_jobs_provider_running
	ON business_image_jobs(provider_id, status)
	WHERE status = 'running';
