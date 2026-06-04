CREATE TABLE IF NOT EXISTS business_system_settings (
	key TEXT PRIMARY KEY,
	value_json BYTEA NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS business_api_providers (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	platform TEXT NOT NULL,
	base_url TEXT NOT NULL,
	api_key TEXT NOT NULL,
	default_model TEXT NOT NULL,
	enabled INTEGER NOT NULL,
	is_default INTEGER NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_api_providers_platform
	ON business_api_providers(platform, enabled, is_default);

CREATE TABLE IF NOT EXISTS business_image_tracker (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	generation_id TEXT NOT NULL,
	turn_id TEXT NOT NULL,
	platform TEXT NOT NULL,
	provider_id TEXT NOT NULL,
	provider_name TEXT NOT NULL,
	model TEXT NOT NULL,
	status TEXT NOT NULL,
	stage TEXT NOT NULL,
	error_code TEXT NOT NULL,
	error_message TEXT NOT NULL,
	requested_count INTEGER NOT NULL,
	actual_count INTEGER NOT NULL,
	queue_wait_ms BIGINT NOT NULL,
	upstream_duration_ms BIGINT NOT NULL,
	persist_duration_ms BIGINT NOT NULL,
	total_duration_ms BIGINT NOT NULL,
	credit_reserved BIGINT NOT NULL,
	credit_refunded BIGINT NOT NULL,
	storage_bytes BIGINT NOT NULL,
	created_at TEXT NOT NULL,
	admitted_at TEXT NOT NULL,
	upstream_started_at TEXT NOT NULL,
	upstream_finished_at TEXT NOT NULL,
	finished_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_image_tracker_created
	ON business_image_tracker(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_image_tracker_platform_created
	ON business_image_tracker(platform, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_image_tracker_status_created
	ON business_image_tracker(status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_image_tracker_user_created
	ON business_image_tracker(user_id, created_at DESC);
