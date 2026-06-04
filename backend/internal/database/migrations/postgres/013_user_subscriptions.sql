CREATE TABLE IF NOT EXISTS business_user_subscriptions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES business_users(id) ON DELETE CASCADE,
	order_id TEXT NOT NULL UNIQUE REFERENCES business_payment_orders(id) ON DELETE CASCADE,
	package_id TEXT NOT NULL,
	package_name TEXT NOT NULL,
	status TEXT NOT NULL,
	starts_at TIMESTAMPTZ NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	cancelled_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_user_subscriptions_user_status_expires
	ON business_user_subscriptions(user_id, status, expires_at DESC);
