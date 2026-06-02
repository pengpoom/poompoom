CREATE TABLE IF NOT EXISTS business_image_models (
	id TEXT PRIMARY KEY,
	vendor TEXT NOT NULL,
	vendor_label TEXT NOT NULL,
	display_name TEXT NOT NULL,
	adapter TEXT NOT NULL,
	platform TEXT NOT NULL,
	upstream_model TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	preview INTEGER NOT NULL DEFAULT 0,
	compare_enabled INTEGER NOT NULL DEFAULT 1,
	generate_enabled INTEGER NOT NULL DEFAULT 1,
	edit_enabled INTEGER NOT NULL DEFAULT 1,
	reference_image_enabled INTEGER NOT NULL DEFAULT 1,
	mask_enabled INTEGER NOT NULL DEFAULT 1,
	sizes TEXT NOT NULL DEFAULT '',
	qualities TEXT NOT NULL DEFAULT '',
	max_images INTEGER NOT NULL DEFAULT 1,
	max_reference_images INTEGER NOT NULL DEFAULT 8,
	credit_cost BIGINT NOT NULL DEFAULT 1,
	is_default INTEGER NOT NULL DEFAULT 0,
	sort_order INTEGER NOT NULL DEFAULT 100,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_business_image_models_enabled_sort
	ON business_image_models(enabled, sort_order, vendor, display_name);

CREATE INDEX IF NOT EXISTS idx_business_image_models_platform
	ON business_image_models(platform, enabled, sort_order);

CREATE UNIQUE INDEX IF NOT EXISTS idx_business_image_models_default
	ON business_image_models(is_default)
	WHERE is_default = 1;
