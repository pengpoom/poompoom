CREATE TABLE IF NOT EXISTS business_codes (
	id TEXT PRIMARY KEY,
	code_hash TEXT NOT NULL UNIQUE,
	code_preview TEXT NOT NULL,
	type TEXT NOT NULL,
	title TEXT NOT NULL,
	credits BIGINT NOT NULL DEFAULT 0,
	max_uses INTEGER NOT NULL DEFAULT 1,
	used_count INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL,
	starts_at TIMESTAMPTZ,
	expires_at TIMESTAMPTZ,
	created_by TEXT NOT NULL,
	note TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_codes_type_status_created
	ON business_codes(type, status, created_at DESC);

CREATE TABLE IF NOT EXISTS business_code_usages (
	id TEXT PRIMARY KEY,
	code_id TEXT NOT NULL REFERENCES business_codes(id) ON DELETE CASCADE,
	user_id TEXT NOT NULL REFERENCES business_users(id) ON DELETE CASCADE,
	context TEXT NOT NULL,
	credits_granted BIGINT NOT NULL DEFAULT 0,
	ledger_id TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_business_code_usages_code_user_context
	ON business_code_usages(code_id, user_id, context);

CREATE INDEX IF NOT EXISTS idx_business_code_usages_user_created
	ON business_code_usages(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS business_affiliate_profiles (
	user_id TEXT PRIMARY KEY REFERENCES business_users(id) ON DELETE CASCADE,
	code_hash TEXT NOT NULL UNIQUE,
	code_preview TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS business_affiliate_referrals (
	id TEXT PRIMARY KEY,
	referrer_user_id TEXT NOT NULL REFERENCES business_users(id) ON DELETE CASCADE,
	referred_user_id TEXT NOT NULL UNIQUE REFERENCES business_users(id) ON DELETE CASCADE,
	affiliate_code_preview TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_affiliate_referrals_referrer_created
	ON business_affiliate_referrals(referrer_user_id, created_at DESC);

