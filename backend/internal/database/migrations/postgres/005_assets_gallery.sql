CREATE INDEX IF NOT EXISTS idx_business_image_assets_user_created
	ON business_image_assets(user_id, created_at DESC, file_name DESC);
