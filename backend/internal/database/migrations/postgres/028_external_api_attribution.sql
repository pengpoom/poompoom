ALTER TABLE business_image_jobs
	ADD COLUMN IF NOT EXISTS api_key_id TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS api_metadata TEXT NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_business_image_jobs_api_key
	ON business_image_jobs(api_key_id, created_at DESC);
