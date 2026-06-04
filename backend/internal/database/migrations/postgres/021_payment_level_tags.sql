ALTER TABLE business_payment_packages
	ADD COLUMN IF NOT EXISTS level_tag TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_business_payment_packages_level_tag
	ON business_payment_packages(package_type, level_tag)
	WHERE level_tag <> '';
