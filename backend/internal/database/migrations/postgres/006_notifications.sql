CREATE TABLE IF NOT EXISTS business_notifications (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	body TEXT NOT NULL,
	level TEXT NOT NULL,
	audience TEXT NOT NULL,
	status TEXT NOT NULL,
	created_by TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	published_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_notifications_status_created
	ON business_notifications(status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_business_notifications_audience_created
	ON business_notifications(audience, created_at DESC);

CREATE TABLE IF NOT EXISTS business_notification_reads (
	notification_id TEXT NOT NULL REFERENCES business_notifications(id) ON DELETE CASCADE,
	user_id TEXT NOT NULL,
	read_at TEXT NOT NULL,
	PRIMARY KEY (notification_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_business_notification_reads_user
	ON business_notification_reads(user_id, read_at DESC);
