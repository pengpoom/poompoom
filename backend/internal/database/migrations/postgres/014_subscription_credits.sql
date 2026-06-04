ALTER TABLE business_payment_packages
	ADD COLUMN IF NOT EXISTS duration_days INTEGER NOT NULL DEFAULT 0;

ALTER TABLE business_payment_orders
	ADD COLUMN IF NOT EXISTS duration_days INTEGER NOT NULL DEFAULT 0;

ALTER TABLE business_user_subscriptions
	ADD COLUMN IF NOT EXISTS duration_days INTEGER NOT NULL DEFAULT 30;

ALTER TABLE business_user_subscriptions
	ADD COLUMN IF NOT EXISTS credits_total BIGINT NOT NULL DEFAULT 0;

ALTER TABLE business_user_subscriptions
	ADD COLUMN IF NOT EXISTS credits_used BIGINT NOT NULL DEFAULT 0;

UPDATE business_payment_packages
	SET package_type = 'subscription',
		duration_days = CASE WHEN duration_days > 0 THEN duration_days ELSE 30 END
	WHERE package_type = 'monthly';

UPDATE business_payment_orders
	SET package_type = 'subscription',
		duration_days = CASE WHEN duration_days > 0 THEN duration_days ELSE 30 END
	WHERE package_type = 'monthly';

UPDATE business_user_subscriptions
	SET duration_days = 30
	WHERE duration_days <= 0;

CREATE INDEX IF NOT EXISTS idx_business_user_subscriptions_active_credits
	ON business_user_subscriptions(user_id, starts_at, expires_at)
	WHERE status = 'active' AND credits_total > credits_used;
