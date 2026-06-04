ALTER TABLE business_image_jobs
	ADD COLUMN IF NOT EXISTS compare_batch_id TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS compare_model_index INTEGER NOT NULL DEFAULT 0,
	ADD COLUMN IF NOT EXISTS compare_model_count INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_business_image_jobs_compare_batch
	ON business_image_jobs(compare_batch_id, updated_at DESC)
	WHERE compare_batch_id <> '';
