ALTER TABLE business_notifications
	ADD COLUMN IF NOT EXISTS notify_mode TEXT NOT NULL DEFAULT 'silent';

ALTER TABLE business_notifications
	ADD COLUMN IF NOT EXISTS starts_at TEXT NOT NULL DEFAULT '';

ALTER TABLE business_notifications
	ADD COLUMN IF NOT EXISTS ends_at TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_business_notifications_status_window
	ON business_notifications(status, starts_at, ends_at);
