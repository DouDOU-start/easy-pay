# easy-pay

统一聚合微信支付 / 支付宝的支付网关，对下游服务提供一套签名认证的 REST API 与异步 HTTP 回调。

## 特性

- 统一抽象：一套 `PaymentChannel` 接口，微信 / 支付宝各自实现
- 多商户：每个商户独立 app_id / app_secret，渠道密钥 AES-256-GCM 加密存库
- 下游请求 HMAC 签名：`X-App-Id / X-Timestamp / X-Nonce / X-Signature`，5 分钟防重放
- 下游回调：收到渠道回调后异步通知下游，失败指数退避重试，最多 8 次
- 管理后台：React + Ant Design，商户、渠道、订单、回调日志、手动重推
- 一键起服：Docker Compose（PostgreSQL + Redis + Adminer + API）

## 目录

```
easy-pay/
├── backend/
│   ├── cmd/api/              # 服务入口
│   ├── config.example.yaml   # 配置模板
│   ├── internal/
│   │   ├── config/           # 配置加载（viper）
│   │   ├── model/            # GORM 模型
│   │   ├── repository/       # DB 访问
│   │   ├── channel/
│   │   │   ├── channel.go    # PaymentChannel 接口
│   │   │   ├── wechat/       # 微信实现（V3）
│   │   │   ├── alipay/       # 支付宝实现
│   │   │   └── registry/     # (merchant,channel) → 实例 缓存
│   │   ├── service/
│   │   │   ├── payment/      # 核心业务：下单/查询/关单/退款/回调
│   │   │   └── notify/       # 下游 HTTP 通知（队列 + 重试）
│   │   ├── handler/
│   │   │   ├── api/          # 下游支付 API
│   │   │   ├── callback/     # 渠道回调接收
│   │   │   ├── admin/        # 管理后台 API + 登录
│   │   │   ├── merchant/     # 商户自助门户
│   │   │   └── middleware/   # 商户签名鉴权
│   │   ├── pkg/              # crypto / sign / idgen / defaults
│   │   ├── server/           # Router 装配
│   │   └── setup/            # 首装向导
│   └── migrations/           # 嵌入式 SQL 迁移
├── web/                      # React + Vite + Ant Design 前端（管理员 + 商户）
├── deploy/
│   ├── Dockerfile            # 多阶段构建
│   └── docker-compose.yml    # Postgres + Redis + Adminer + API
├── Makefile                  # 开发 / 构建快捷命令
├── go.mod
└── go.sum
```

## 配置

所有配置统一在 `backend/config.yaml`（由首装向导自动生成，已 gitignore）。

参考模板：`backend/config.example.yaml`。

## 快速开始

```bash
# 1. 启动基础设施（Postgres + Redis + Adminer）
make infra

# 2. 一键启动 API + 前端开发服务器
make dev
```

- API：`http://localhost:8080`
- 管理后台（Vite dev）：`http://localhost:5173`
- Adminer（DB 可视化）：`http://localhost:8081`（server=postgres user=easypay pass=easypay db=easypay）

首次启动会自动进入「初始化向导」，在浏览器里设置管理员账号与连接信息后，向导会写出 `config.yaml` 并重启 API。

其他常用命令：

```bash
make run          # 仅运行 Go API
make web          # 仅运行前端 dev server
make build        # 生产构建（Go 二进制 + 内嵌 SPA）
make up           # Docker 全栈启动（含 API 容器）
make down         # 停止全栈
make infra-down   # 仅停止基础设施
```

## 下游接入

### 1. 签名算法

```
signature = hex(HMAC-SHA256(app_secret,
    method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + body))
```

请求头：`X-App-Id`、`X-Timestamp`（秒）、`X-Nonce`、`X-Signature`。时间戳偏差超过 5 分钟会被拒绝。

### 2. 下单

```http
POST /api/v1/pay/create
Content-Type: application/json

{
  "merchant_order_no": "SHOP_20260415_00001",
  "channel": "wechat",
  "trade_type": "native",
  "subject": "商品名称",
  "amount": 100,
  "expire_seconds": 900
}
```

响应：

```json
{
  "code": "OK",
  "data": {
    "order_no": "EP20260415103000...",
    "code_url": "weixin://wxpay/bizpayurl?pr=...",
    "h5_url": ""
  }
}
```

### 3. 查询 / 关单 / 退款

```
GET  /api/v1/pay/query?merchant_order_no=SHOP_20260415_00001
POST /api/v1/pay/close   { "merchant_order_no": "..." }
POST /api/v1/pay/refund  { "merchant_order_no": "...", "merchant_refund_no": "...", "amount": 100 }
```

### 4. 下游通知

支付成功后，easy-pay 会向商户 `notify_url` 发起签名 POST：

```http
POST {merchant.notify_url}
Content-Type: application/json
X-App-Id: ap_xxx
X-Timestamp: 1712000000
X-Nonce: ...
X-Signature: ...
X-Event-Type: payment.success

{
  "order_no": "EP...",
  "merchant_order_no": "SHOP_...",
  "channel": "wechat",
  "channel_order_no": "...",
  "amount": 100,
  "currency": "CNY",
  "status": "paid",
  "paid_at": "2026-04-15T10:30:00Z"
}
```

下游返回 HTTP 2xx 视为成功。失败按以下间隔重试：15s, 60s, 5m, 15m, 30m, 1h, 2h, 4h。

## 微信渠道配置

在管理后台的 **商户 → 配置渠道** 里填入 JSON（服务端 AES-256-GCM 加密落库）：

```json
{
  "mch_id": "1900000000",
  "app_id": "wxXXXXXXXXXXXXXXXX",
  "api_v3_key": "your-32-byte-api-v3-key-here---",
  "serial_no": "YOUR_CERT_SERIAL_NUMBER",
  "private_key_pem": "-----BEGIN PRIVATE KEY-----\\n...\\n-----END PRIVATE KEY-----"
}
```

SDK 会在首次 Resolve 时自动下载并定期刷新平台证书（用于回调验签），证书缓存由 `wechatpay-go/core/downloader` 全局管理，按 `mch_id` 索引。

回调地址：`https://{your-domain}/callback/wechat/{merchant_id}` —— 在 `prepay` 时由 `notify_url` 字段传给微信。

> **微信要求 `notify_url` 必须是公网 HTTPS**。本地联调请用 ngrok / cpolar 反代 `http://localhost:8080/callback/...`，并在 `config.yaml` 中设置 `server.platform_base: "https://xxx.ngrok.io"`。

## 当前状态

- ✅ 全链路骨架（下单 → 渠道 → DB → 回调 → 下游通知）
- ✅ 管理后台 API（商户、渠道、订单、日志）
- ✅ 微信支付 V3 真实 SDK 接入（Native / H5 下单、查询、关单、退款、回调验签）
- ⚠️ 支付宝仍为占位符，待接入 `smartwalle/alipay/v3`
- ⚠️ 管理前端仅含脚手架 + 登录 + 订单/商户基本页，视需补完
