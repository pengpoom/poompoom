ALTER TABLE business_risk_control_logs
	ADD COLUMN IF NOT EXISTS moderation_provider TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS risk_level TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS provider_reason TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS provider_latency_ms BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_business_risk_control_logs_provider_created
	ON business_risk_control_logs(moderation_provider, created_at DESC);
