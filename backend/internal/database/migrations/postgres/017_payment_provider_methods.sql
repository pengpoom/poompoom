ALTER TABLE business_payment_orders
	ADD COLUMN IF NOT EXISTS payment_method TEXT NOT NULL DEFAULT 'manual';

CREATE INDEX IF NOT EXISTS idx_business_payment_orders_method_created
	ON business_payment_orders(payment_method, created_at DESC)
	WHERE payment_method <> '';
