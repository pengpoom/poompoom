ALTER TABLE business_credit_ledger
	ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '';

ALTER TABLE business_credit_ledger
	ADD COLUMN IF NOT EXISTS source_id TEXT NOT NULL DEFAULT '';

ALTER TABLE business_credit_ledger
	ADD COLUMN IF NOT EXISTS metadata_json BYTEA;

CREATE INDEX IF NOT EXISTS idx_business_credit_ledger_source
	ON business_credit_ledger(source_type, source_id)
	WHERE source_type <> '' AND source_id <> '';

CREATE TABLE IF NOT EXISTS business_payment_packages (
	id TEXT PRIMARY KEY,
	package_type TEXT NOT NULL DEFAULT 'balance',
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	amount_cents BIGINT NOT NULL,
	credits BIGINT NOT NULL,
	duration_days INTEGER NOT NULL DEFAULT 0,
	currency TEXT NOT NULL,
	enabled INTEGER NOT NULL,
	sort_order INTEGER NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_payment_packages_enabled_sort
	ON business_payment_packages(enabled, sort_order, created_at DESC);

CREATE TABLE IF NOT EXISTS business_payment_providers (
	id TEXT PRIMARY KEY,
	provider_key TEXT NOT NULL,
	name TEXT NOT NULL,
	enabled INTEGER NOT NULL,
	supported_methods TEXT NOT NULL,
	config_ciphertext BYTEA,
	limits_json BYTEA,
	sort_order INTEGER NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_payment_providers_key_enabled
	ON business_payment_providers(provider_key, enabled, sort_order);

CREATE TABLE IF NOT EXISTS business_payment_orders (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES business_users(id) ON DELETE CASCADE,
	user_email TEXT NOT NULL,
	username TEXT NOT NULL,
	package_id TEXT NOT NULL,
	package_type TEXT NOT NULL DEFAULT 'balance',
	package_snapshot_json BYTEA,
	amount_cents BIGINT NOT NULL,
	credits BIGINT NOT NULL,
	duration_days INTEGER NOT NULL DEFAULT 0,
	currency TEXT NOT NULL,
	payment_method TEXT NOT NULL DEFAULT 'manual',
	provider_key TEXT NOT NULL,
	provider_instance_id TEXT NOT NULL,
	provider_snapshot_json BYTEA,
	out_trade_no TEXT NOT NULL UNIQUE,
	provider_trade_no TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	pay_url TEXT NOT NULL DEFAULT '',
	qr_code TEXT NOT NULL DEFAULT '',
	expires_at TIMESTAMPTZ,
	paid_at TIMESTAMPTZ,
	completed_at TIMESTAMPTZ,
	failed_at TIMESTAMPTZ,
	refunded_at TIMESTAMPTZ,
	credit_ledger_id TEXT NOT NULL DEFAULT '',
	billing_action TEXT NOT NULL DEFAULT '',
	upgrade_from_subscription_id TEXT NOT NULL DEFAULT '',
	upgrade_credit_cents BIGINT NOT NULL DEFAULT 0,
	original_amount_cents BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_payment_orders_user_status
	ON business_payment_orders(user_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_payment_orders_status_created
	ON business_payment_orders(status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_payment_orders_expires
	ON business_payment_orders(expires_at)
	WHERE expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_business_payment_orders_billing_action
	ON business_payment_orders(billing_action, created_at DESC)
	WHERE billing_action <> '';

CREATE TABLE IF NOT EXISTS business_idempotency_records (
	scope TEXT NOT NULL,
	idempotency_key_hash TEXT NOT NULL,
	request_fingerprint TEXT NOT NULL,
	status TEXT NOT NULL,
	response_status INTEGER NOT NULL DEFAULT 0,
	response_body BYTEA,
	error_reason TEXT NOT NULL DEFAULT '',
	locked_until TIMESTAMPTZ,
	expires_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (scope, idempotency_key_hash)
);

CREATE INDEX IF NOT EXISTS idx_business_idempotency_records_expires
	ON business_idempotency_records(expires_at);

CREATE TABLE IF NOT EXISTS business_payment_audit_logs (
	id TEXT PRIMARY KEY,
	order_id TEXT NOT NULL REFERENCES business_payment_orders(id) ON DELETE CASCADE,
	action TEXT NOT NULL,
	detail_json BYTEA,
	operator TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_payment_audit_logs_order_created
	ON business_payment_audit_logs(order_id, created_at DESC);

CREATE TABLE IF NOT EXISTS business_affiliate_commissions (
	id TEXT PRIMARY KEY,
	order_id TEXT NOT NULL UNIQUE REFERENCES business_payment_orders(id) ON DELETE CASCADE,
	referrer_user_id TEXT NOT NULL REFERENCES business_users(id) ON DELETE CASCADE,
	referred_user_id TEXT NOT NULL REFERENCES business_users(id) ON DELETE CASCADE,
	base_amount_cents BIGINT NOT NULL,
	rate_bps INTEGER NOT NULL,
	credits BIGINT NOT NULL,
	ledger_id TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	settled_at TIMESTAMPTZ,
	reversed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_business_affiliate_commissions_referrer_created
	ON business_affiliate_commissions(referrer_user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS business_user_subscriptions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES business_users(id) ON DELETE CASCADE,
	order_id TEXT NOT NULL UNIQUE REFERENCES business_payment_orders(id) ON DELETE CASCADE,
	package_id TEXT NOT NULL,
	package_name TEXT NOT NULL,
	duration_days INTEGER NOT NULL DEFAULT 30,
	credits_total BIGINT NOT NULL DEFAULT 0,
	credits_used BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL,
	starts_at TIMESTAMPTZ NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	cancelled_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_user_subscriptions_user_status_expires
	ON business_user_subscriptions(user_id, status, expires_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_user_subscriptions_active_credits
	ON business_user_subscriptions(user_id, starts_at, expires_at)
	WHERE status = 'active' AND credits_total > credits_used;
