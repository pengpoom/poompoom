ALTER TABLE business_payment_packages
	ADD COLUMN IF NOT EXISTS package_type TEXT NOT NULL DEFAULT 'balance';

ALTER TABLE business_payment_orders
	ADD COLUMN IF NOT EXISTS package_type TEXT NOT NULL DEFAULT 'balance';

UPDATE business_payment_packages
	SET package_type = 'subscription'
	WHERE package_type = 'monthly';

UPDATE business_payment_orders
	SET package_type = 'subscription'
	WHERE package_type = 'monthly';
