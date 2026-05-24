ALTER TABLE business_notifications
	ADD COLUMN IF NOT EXISTS targeting TEXT NOT NULL DEFAULT '{"mode":"all"}';

