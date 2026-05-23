CREATE TABLE IF NOT EXISTS business_user_credits (
	user_id TEXT PRIMARY KEY,
	balance BIGINT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS business_credit_ledger (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	delta BIGINT NOT NULL,
	balance_after BIGINT NOT NULL,
	reason TEXT NOT NULL,
	generation_id TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_credit_ledger_user_created
	ON business_credit_ledger(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_credit_ledger_generation
	ON business_credit_ledger(generation_id);

CREATE TABLE IF NOT EXISTS business_image_jobs (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	generation_id TEXT NOT NULL,
	turn_id TEXT NOT NULL,
	platform TEXT NOT NULL,
	provider_id TEXT NOT NULL,
	provider_name TEXT NOT NULL,
	model TEXT NOT NULL,
	prompt TEXT NOT NULL,
	size TEXT NOT NULL,
	quality TEXT NOT NULL,
	requested_count INTEGER NOT NULL,
	actual_count INTEGER NOT NULL,
	status TEXT NOT NULL,
	stage TEXT NOT NULL,
	upstream_sent INTEGER NOT NULL DEFAULT 0,
	error_code TEXT NOT NULL,
	error_message TEXT NOT NULL,
	queue_wait_ms BIGINT NOT NULL,
	upstream_duration_ms BIGINT NOT NULL,
	persist_duration_ms BIGINT NOT NULL,
	total_duration_ms BIGINT NOT NULL,
	storage_bytes BIGINT NOT NULL,
	credit_reserved BIGINT NOT NULL,
	credit_refunded BIGINT NOT NULL,
	payload_json BYTEA,
	created_at TEXT NOT NULL,
	queued_at TEXT NOT NULL,
	started_at TEXT NOT NULL,
	finished_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_image_jobs_user_updated
	ON business_image_jobs(user_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_image_jobs_conversation_created
	ON business_image_jobs(user_id, conversation_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_image_jobs_generation
	ON business_image_jobs(user_id, generation_id);

CREATE INDEX IF NOT EXISTS idx_business_image_jobs_status_updated
	ON business_image_jobs(status, updated_at DESC);

CREATE TABLE IF NOT EXISTS business_image_conversations (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	title TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_image_conversations_user_updated
	ON business_image_conversations(user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS business_image_generations (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	turn_id TEXT NOT NULL,
	prompt TEXT NOT NULL,
	model TEXT NOT NULL,
	size TEXT NOT NULL,
	quality TEXT NOT NULL,
	count INTEGER NOT NULL,
	status TEXT NOT NULL,
	response_json BYTEA,
	error TEXT NOT NULL,
	created_at TEXT NOT NULL,
	finished_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_image_generations_conversation_created
	ON business_image_generations(conversation_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_business_image_generations_user_created
	ON business_image_generations(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_image_generations_filter
	ON business_image_generations(user_id, status, model, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_image_generations_created
	ON business_image_generations(created_at DESC);

CREATE TABLE IF NOT EXISTS business_image_assets (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	generation_id TEXT NOT NULL,
	file_name TEXT NOT NULL,
	file_path TEXT NOT NULL,
	url TEXT NOT NULL,
	mime_type TEXT NOT NULL,
	size_bytes BIGINT NOT NULL,
	sha256 TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_business_image_assets_file_name
	ON business_image_assets(file_name);

CREATE INDEX IF NOT EXISTS idx_business_image_assets_user_conversation
	ON business_image_assets(user_id, conversation_id);

CREATE INDEX IF NOT EXISTS idx_business_image_assets_generation
	ON business_image_assets(generation_id);
