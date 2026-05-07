CREATE TABLE IF NOT EXISTS settlements (
    id              BIGSERIAL PRIMARY KEY,
    settlement_no   VARCHAR(40)  NOT NULL UNIQUE,
    merchant_id     BIGINT       NOT NULL REFERENCES merchants(id),
    amount          BIGINT       NOT NULL, -- gross amount (cents)
    fee             BIGINT       NOT NULL DEFAULT 0, -- platform fee (cents)
    net_amount      BIGINT       NOT NULL, -- amount - fee
    period_start    TIMESTAMPTZ  NOT NULL,
    period_end      TIMESTAMPTZ  NOT NULL,
    status          VARCHAR(16)  NOT NULL DEFAULT 'pending', -- pending | paid | cancelled
    remark          VARCHAR(256) NOT NULL DEFAULT '',
    paid_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_settlements_merchant ON settlements(merchant_id);
CREATE INDEX IF NOT EXISTS idx_settlements_status ON settlements(status);
