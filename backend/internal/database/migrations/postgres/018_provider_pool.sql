CREATE TABLE IF NOT EXISTS business_provider_groups (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	platform TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	enabled INTEGER NOT NULL DEFAULT 1,
	is_default INTEGER NOT NULL DEFAULT 0,
	priority INTEGER NOT NULL DEFAULT 50,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_business_provider_groups_default
	ON business_provider_groups(platform)
	WHERE is_default = 1;

CREATE INDEX IF NOT EXISTS idx_business_provider_groups_platform
	ON business_provider_groups(platform, enabled, is_default, priority, created_at);

CREATE TABLE IF NOT EXISTS business_provider_members (
	id TEXT PRIMARY KEY,
	group_id TEXT NOT NULL REFERENCES business_provider_groups(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	platform TEXT NOT NULL,
	base_url TEXT NOT NULL,
	api_key TEXT NOT NULL,
	default_model TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	priority INTEGER NOT NULL DEFAULT 50,
	status TEXT NOT NULL DEFAULT 'active',
	cooldown_until TEXT NOT NULL DEFAULT '',
	success_count BIGINT NOT NULL DEFAULT 0,
	fail_count BIGINT NOT NULL DEFAULT 0,
	last_used_at TEXT NOT NULL DEFAULT '',
	last_error TEXT NOT NULL DEFAULT '',
	last_error_at TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_provider_members_group
	ON business_provider_members(group_id, enabled, status, priority, last_used_at);

CREATE INDEX IF NOT EXISTS idx_business_provider_members_platform
	ON business_provider_members(platform, enabled, status, priority, last_used_at);
