ALTER TABLE business_users
	ADD COLUMN IF NOT EXISTS subscription_level_tag TEXT NOT NULL DEFAULT '';

ALTER TABLE business_users
	ADD COLUMN IF NOT EXISTS wallet_level_tag TEXT NOT NULL DEFAULT '';
