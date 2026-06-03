CREATE TABLE IF NOT EXISTS business_risk_control_logs (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL DEFAULT '',
	job_id TEXT NOT NULL DEFAULT '',
	conversation_id TEXT NOT NULL DEFAULT '',
	turn_id TEXT NOT NULL DEFAULT '',
	platform TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	mode TEXT NOT NULL DEFAULT '',
	action TEXT NOT NULL DEFAULT '',
	flagged BOOLEAN NOT NULL DEFAULT FALSE,
	highest_category TEXT NOT NULL DEFAULT '',
	highest_score DOUBLE PRECISION NOT NULL DEFAULT 0,
	category_scores_json BYTEA NOT NULL,
	input_excerpt TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	latency_ms BIGINT NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_risk_control_logs_created
	ON business_risk_control_logs(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_risk_control_logs_flagged_created
	ON business_risk_control_logs(flagged, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_risk_control_logs_user_created
	ON business_risk_control_logs(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_risk_control_logs_action_created
	ON business_risk_control_logs(action, created_at DESC);
