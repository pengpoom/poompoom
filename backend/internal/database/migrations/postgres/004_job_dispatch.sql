ALTER TABLE business_image_jobs
	ADD COLUMN IF NOT EXISTS claimed_by TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS claimed_at TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS lease_until TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS attempts INTEGER NOT NULL DEFAULT 0,
	ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS next_run_at TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_business_image_jobs_runnable
	ON business_image_jobs(status, next_run_at, lease_until, created_at);
