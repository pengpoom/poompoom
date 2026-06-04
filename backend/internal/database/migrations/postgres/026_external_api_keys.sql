CREATE TABLE IF NOT EXISTS business_api_keys (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES business_users(id),
	name TEXT NOT NULL DEFAULT '',
	key_prefix TEXT NOT NULL DEFAULT '',
	key_last4 TEXT NOT NULL DEFAULT '',
	key_hash TEXT NOT NULL UNIQUE,
	status TEXT NOT NULL DEFAULT 'active',
	credit_limit BIGINT NOT NULL DEFAULT 0,
	used_credits BIGINT NOT NULL DEFAULT 0,
	rate_limit_per_minute INTEGER NOT NULL DEFAULT 60,
	concurrency_limit INTEGER NOT NULL DEFAULT 2,
	allowed_models TEXT NOT NULL DEFAULT '[]',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	last_used_at TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	revoked_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_business_api_keys_user
	ON business_api_keys(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_api_keys_prefix
	ON business_api_keys(key_prefix, status);
