-- Aggregator model refactor.
--
-- Before: each merchant stored their own WeChat/Alipay credentials in
--         merchant_channels.config (encrypted JSON).
-- After:  the platform holds one credential set per channel in
--         platform_channels; merchant_channels only tracks which channels
--         a merchant is authorised to use (+ optional sub_mch_id for future
--         WeChat Pay service-provider / sub-merchant split-settlement).

CREATE TABLE IF NOT EXISTS platform_channels (
    id         BIGSERIAL   PRIMARY KEY,
    channel    VARCHAR(16) NOT NULL UNIQUE, -- wechat | alipay
    config     TEXT        NOT NULL,        -- AES-GCM encrypted JSON (same schema as before)
    status     SMALLINT    NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Drop per-merchant credentials; keep the row as an authorisation record.
ALTER TABLE merchant_channels DROP COLUMN IF EXISTS config;
