CREATE SEQUENCE IF NOT EXISTS business_user_uid_seq START 100001;

CREATE TABLE IF NOT EXISTS business_users (
	id TEXT PRIMARY KEY,
	uid BIGINT NOT NULL UNIQUE DEFAULT nextval('business_user_uid_seq'),
	username TEXT NOT NULL UNIQUE,
	email TEXT NOT NULL UNIQUE DEFAULT '',
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL,
	status TEXT NOT NULL,
	deleted_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_users_status_deleted
	ON business_users(status, deleted_at);

CREATE INDEX IF NOT EXISTS idx_business_users_created_uid
	ON business_users(created_at, uid);

CREATE TABLE IF NOT EXISTS user_sessions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES business_users(id) ON DELETE CASCADE,
	token_hash TEXT NOT NULL UNIQUE,
	expires_at TIMESTAMPTZ NOT NULL,
	revoked_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL,
	last_seen_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_expires
	ON user_sessions(user_id, expires_at DESC);

CREATE TABLE IF NOT EXISTS email_verification_codes (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL,
	purpose TEXT NOT NULL,
	code_hash TEXT NOT NULL,
	attempts INTEGER NOT NULL DEFAULT 0,
	consumed_at TIMESTAMPTZ,
	expires_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_email_verification_lookup
	ON email_verification_codes(email, purpose, created_at DESC);
