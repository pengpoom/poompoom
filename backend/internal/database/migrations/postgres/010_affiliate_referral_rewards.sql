ALTER TABLE business_affiliate_referrals
	ADD COLUMN IF NOT EXISTS reward_ledger_id TEXT NOT NULL DEFAULT '';

ALTER TABLE business_affiliate_referrals
	ADD COLUMN IF NOT EXISTS reward_credits BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_business_affiliate_referrals_reward_ledger
	ON business_affiliate_referrals(reward_ledger_id)
	WHERE reward_ledger_id <> '';
