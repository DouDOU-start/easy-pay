ALTER TABLE orders ADD COLUMN IF NOT EXISTS settlement_id BIGINT REFERENCES settlements(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_orders_settlement ON orders(settlement_id);
