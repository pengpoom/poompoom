CREATE TABLE IF NOT EXISTS business_risk_control_policies (
	id TEXT PRIMARY KEY,
	scope TEXT NOT NULL,
	target_id TEXT NOT NULL DEFAULT '',
	enabled BOOLEAN NOT NULL DEFAULT TRUE,
	mode TEXT NOT NULL DEFAULT '',
	risk_level TEXT NOT NULL DEFAULT '',
	block_message TEXT NOT NULL DEFAULT '',
	thresholds_json BYTEA NOT NULL DEFAULT '{}'::bytea,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(scope, target_id)
);

CREATE INDEX IF NOT EXISTS idx_business_risk_control_policies_scope
	ON business_risk_control_policies(scope, target_id);
