-- Add self-service auth fields to merchants.

ALTER TABLE merchants
    ADD COLUMN IF NOT EXISTS email               VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS password_hash       VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ;

-- Only enforce uniqueness on non-empty emails so existing rows (which get '')
-- don't collide.
CREATE UNIQUE INDEX IF NOT EXISTS idx_merchants_email
    ON merchants(email)
    WHERE email <> '';
