-- Merge admin_users into merchants with role-based access.

ALTER TABLE merchants
    ADD COLUMN IF NOT EXISTS role VARCHAR(16) NOT NULL DEFAULT 'merchant';

-- Migrate existing admin users into merchants table.
-- Generate mch_no / app_id / app_secret so they have valid merchant identities.
INSERT INTO merchants (name, email, password_hash, role, status, mch_no, app_id, app_secret)
SELECT
    username,
    username,
    password_hash,
    'admin',
    status,
    'M' || LPAD(nextval('merchants_id_seq')::TEXT, 14, '0'),
    'ap_' || LEFT(gen_random_uuid()::TEXT, 12),
    gen_random_uuid()::TEXT || gen_random_uuid()::TEXT
FROM admin_users
ON CONFLICT DO NOTHING;

DROP TABLE IF EXISTS admin_users;
