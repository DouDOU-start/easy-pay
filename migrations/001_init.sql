-- easy-pay initial schema

CREATE TABLE IF NOT EXISTS merchants (
    id           BIGSERIAL PRIMARY KEY,
    mch_no       VARCHAR(32)  NOT NULL UNIQUE,
    name         VARCHAR(128) NOT NULL,
    app_id       VARCHAR(64)  NOT NULL UNIQUE,
    app_secret   VARCHAR(128) NOT NULL,
    notify_url   VARCHAR(512) NOT NULL DEFAULT '',
    status       SMALLINT     NOT NULL DEFAULT 1, -- 1=active, 0=disabled
    remark       VARCHAR(256) NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS merchant_channels (
    id           BIGSERIAL PRIMARY KEY,
    merchant_id  BIGINT       NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    channel      VARCHAR(16)  NOT NULL, -- wechat | alipay
    config       TEXT         NOT NULL, -- AES-GCM encrypted JSON
    status       SMALLINT     NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (merchant_id, channel)
);

CREATE TABLE IF NOT EXISTS orders (
    id                 BIGSERIAL PRIMARY KEY,
    order_no           VARCHAR(40)  NOT NULL UNIQUE, -- platform order no
    merchant_id        BIGINT       NOT NULL REFERENCES merchants(id),
    merchant_order_no  VARCHAR(64)  NOT NULL, -- downstream order no (idempotency)
    channel            VARCHAR(16)  NOT NULL, -- wechat | alipay
    channel_order_no   VARCHAR(64)  NOT NULL DEFAULT '', -- transaction_id / trade_no
    trade_type         VARCHAR(16)  NOT NULL, -- native | h5
    subject            VARCHAR(256) NOT NULL,
    amount             BIGINT       NOT NULL, -- in cents
    currency           VARCHAR(8)   NOT NULL DEFAULT 'CNY',
    status             VARCHAR(16)  NOT NULL, -- pending | paid | closed | refunded | partial_refunded | failed
    client_ip          VARCHAR(64)  NOT NULL DEFAULT '',
    extra              JSONB        NOT NULL DEFAULT '{}'::jsonb,
    code_url           VARCHAR(512) NOT NULL DEFAULT '', -- QR code url for native
    h5_url             VARCHAR(512) NOT NULL DEFAULT '',
    expire_at          TIMESTAMPTZ,
    paid_at            TIMESTAMPTZ,
    closed_at          TIMESTAMPTZ,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (merchant_id, merchant_order_no)
);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at DESC);

CREATE TABLE IF NOT EXISTS refund_orders (
    id                  BIGSERIAL PRIMARY KEY,
    refund_no           VARCHAR(40)  NOT NULL UNIQUE,
    merchant_id         BIGINT       NOT NULL REFERENCES merchants(id),
    merchant_refund_no  VARCHAR(64)  NOT NULL,
    order_no            VARCHAR(40)  NOT NULL,
    channel             VARCHAR(16)  NOT NULL,
    channel_refund_no   VARCHAR(64)  NOT NULL DEFAULT '',
    amount              BIGINT       NOT NULL,
    reason              VARCHAR(256) NOT NULL DEFAULT '',
    status              VARCHAR(16)  NOT NULL, -- pending | success | failed
    refunded_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (merchant_id, merchant_refund_no)
);
CREATE INDEX IF NOT EXISTS idx_refund_order_no ON refund_orders(order_no);

CREATE TABLE IF NOT EXISTS notify_logs (
    id            BIGSERIAL PRIMARY KEY,
    merchant_id   BIGINT       NOT NULL,
    order_no      VARCHAR(40)  NOT NULL,
    event_type    VARCHAR(32)  NOT NULL, -- payment.success | refund.success | ...
    notify_url    VARCHAR(512) NOT NULL,
    request_body  TEXT         NOT NULL,
    response_body TEXT         NOT NULL DEFAULT '',
    http_status   INT          NOT NULL DEFAULT 0,
    status        VARCHAR(16)  NOT NULL, -- pending | success | failed | dropped
    retry_count   INT          NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    last_error    VARCHAR(512) NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_notify_logs_order ON notify_logs(order_no);
CREATE INDEX IF NOT EXISTS idx_notify_logs_retry ON notify_logs(status, next_retry_at);

CREATE TABLE IF NOT EXISTS admin_users (
    id            BIGSERIAL PRIMARY KEY,
    username      VARCHAR(64)  NOT NULL UNIQUE,
    password_hash VARCHAR(128) NOT NULL,
    role          VARCHAR(16)  NOT NULL DEFAULT 'admin',
    status        SMALLINT     NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
