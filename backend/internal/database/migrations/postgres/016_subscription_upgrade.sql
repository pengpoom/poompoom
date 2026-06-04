ALTER TABLE business_payment_orders
	ADD COLUMN IF NOT EXISTS billing_action TEXT NOT NULL DEFAULT '';

ALTER TABLE business_payment_orders
	ADD COLUMN IF NOT EXISTS upgrade_from_subscription_id TEXT NOT NULL DEFAULT '';

ALTER TABLE business_payment_orders
	ADD COLUMN IF NOT EXISTS upgrade_credit_cents BIGINT NOT NULL DEFAULT 0;

ALTER TABLE business_payment_orders
	ADD COLUMN IF NOT EXISTS original_amount_cents BIGINT NOT NULL DEFAULT 0;

UPDATE business_payment_orders
SET original_amount_cents = amount_cents
WHERE original_amount_cents = 0;

CREATE INDEX IF NOT EXISTS idx_business_payment_orders_billing_action
	ON business_payment_orders(billing_action, created_at DESC)
	WHERE billing_action <> '';
