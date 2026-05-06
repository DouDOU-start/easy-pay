# Easy-Pay 商户对接文档

## 概述

Easy-Pay 是统一支付网关，聚合微信支付和支付宝，商户通过 **HMAC-SHA256 签名认证**的 REST API 完成支付对接。

---

## 接入准备

商户需要从平台管理员获取以下凭证（创建商户时一次性返回，请妥善保管）：

| 参数 | 说明 |
|------|------|
| `app_id` | 商户应用 ID，用于标识身份 |
| `app_secret` | 密钥，用于签名计算 |

> **注意：** `app_secret` 仅在创建商户时返回一次，丢失需联系管理员重置。

---

## 请求签名

所有 `/api/v1/pay/*` 接口必须携带签名头，否则返回 `AUTH_MISSING` 错误。

### 签名请求头

| Header | 说明 |
|--------|------|
| `X-App-Id` | 商户 app_id |
| `X-Timestamp` | 当前 Unix 时间戳（秒），与服务器偏差不超过 ±5 分钟 |
| `X-Nonce` | 随机字符串（建议 UUID），防重放 |
| `X-Signature` | HMAC-SHA256 签名值（hex 编码） |

### 签名算法

**Step 1 — 构造待签名字符串：**

```
签名串 = HTTP方法 + "\n" + 请求路径 + "\n" + 时间戳 + "\n" + 随机串 + "\n" + 请求体
```

- GET 请求的「请求体」为空字符串
- 请求路径不含域名和查询参数，如 `/api/v1/pay/create`

**Step 2 — 计算签名：**

```
signature = hex(HMAC-SHA256(app_secret, 签名串))
```

### 示例代码

#### Python

```python
import hmac, hashlib, time, uuid, json, requests

app_id = "ap_xxx"
app_secret = "your_secret"

method = "POST"
path = "/api/v1/pay/create"
timestamp = str(int(time.time()))
nonce = uuid.uuid4().hex
body = json.dumps({
    "merchant_order_no": "ORDER_001",
    "channel": "wechat",
    "trade_type": "native",
    "subject": "测试商品",
    "amount": 100
})

sign_str = f"{method}\n{path}\n{timestamp}\n{nonce}\n{body}"
signature = hmac.new(
    app_secret.encode(),
    sign_str.encode(),
    hashlib.sha256
).hexdigest()

resp = requests.post(
    f"https://pay.example.com{path}",
    headers={
        "Content-Type": "application/json",
        "X-App-Id": app_id,
        "X-Timestamp": timestamp,
        "X-Nonce": nonce,
        "X-Signature": signature,
    },
    data=body
)
print(resp.json())
```

#### Java

```java
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

String method = "POST";
String path = "/api/v1/pay/create";
String timestamp = String.valueOf(System.currentTimeMillis() / 1000);
String nonce = UUID.randomUUID().toString().replace("-", "");
String body = "{\"merchant_order_no\":\"ORDER_001\",\"channel\":\"wechat\",\"trade_type\":\"native\",\"subject\":\"测试商品\",\"amount\":100}";

String signStr = method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + body;

Mac mac = Mac.getInstance("HmacSHA256");
mac.init(new SecretKeySpec(appSecret.getBytes("UTF-8"), "HmacSHA256"));
byte[] hash = mac.doFinal(signStr.getBytes("UTF-8"));
String signature = bytesToHex(hash);
```

#### Go

```go
import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "strings"
    "time"
)

method := "POST"
path := "/api/v1/pay/create"
timestamp := fmt.Sprintf("%d", time.Now().Unix())
nonce := generateNonce()
body := `{"merchant_order_no":"ORDER_001","channel":"wechat","trade_type":"native","subject":"测试商品","amount":100}`

signStr := strings.Join([]string{method, path, timestamp, nonce, body}, "\n")

h := hmac.New(sha256.New, []byte(appSecret))
h.Write([]byte(signStr))
signature := hex.EncodeToString(h.Sum(nil))
```

---

## 支付接口

### 创建订单

```
POST /api/v1/pay/create
```

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| `merchant_order_no` | string | 是 | 商户订单号（需唯一） |
| `channel` | string | 是 | 支付渠道：`wechat` / `alipay` |
| `trade_type` | string | 是 | 交易类型：`native`（扫码） / `h5`（H5跳转） |
| `subject` | string | 是 | 商品描述 |
| `amount` | int64 | 是 | 金额，单位：**分**（最小值 1） |
| `expire_seconds` | int | 否 | 过期时间（秒），默认 900 |
| `currency` | string | 否 | 币种，默认 `CNY` |
| `extra` | object | 否 | 额外参数（渠道特殊参数） |

**请求示例：**

```json
{
  "merchant_order_no": "SHOP_20260415_00001",
  "channel": "wechat",
  "trade_type": "native",
  "subject": "Premium会员月卡",
  "amount": 2990,
  "expire_seconds": 600
}
```

**响应示例：**

```json
{
  "code": "OK",
  "data": {
    "order_no": "EP20260415103000xxx",
    "code_url": "weixin://wxpay/bizpayurl?pr=...",
    "h5_url": ""
  }
}
```

| 响应字段 | 说明 |
|----------|------|
| `order_no` | 平台订单号 |
| `code_url` | Native 支付二维码链接（trade_type=native 时返回） |
| `h5_url` | H5 支付跳转链接（trade_type=h5 时返回） |

**幂等性：** 相同 `merchant_order_no` 重复请求会直接返回已有订单，不会重复创建。

---

### 查询订单

```
GET /api/v1/pay/query?merchant_order_no={订单号}
```

**响应示例：**

```json
{
  "code": "OK",
  "data": {
    "order_no": "EP20260415103000xxx",
    "merchant_order_no": "SHOP_20260415_00001",
    "channel": "wechat",
    "trade_type": "native",
    "subject": "Premium会员月卡",
    "amount": 2990,
    "currency": "CNY",
    "status": "paid",
    "paid_at": "2026-04-15T10:31:22Z",
    "created_at": "2026-04-15T10:30:00Z"
  }
}
```

**订单状态 `status` 枚举：**

| 值 | 含义 |
|----|------|
| `pending` | 待支付 |
| `paid` | 已支付 |
| `closed` | 已关闭 |
| `refunded` | 已全额退款 |
| `partial_refunded` | 部分退款 |
| `failed` | 支付失败 |

---

### 关闭订单

```
POST /api/v1/pay/close
```

**请求参数：**

```json
{
  "merchant_order_no": "SHOP_20260415_00001"
}
```

仅 `pending` 状态的订单可关闭。

**响应：**

```json
{
  "code": "OK"
}
```

---

### 申请退款

```
POST /api/v1/pay/refund
```

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| `merchant_order_no` | string | 是 | 原支付订单号 |
| `merchant_refund_no` | string | 是 | 商户退款单号（需唯一） |
| `amount` | int64 | 是 | 退款金额（分） |
| `reason` | string | 否 | 退款原因 |

**约束：**
- 仅 `paid` 或 `partial_refunded` 状态可退款
- 退款总额不得超过原订单金额

**响应示例：**

```json
{
  "code": "OK",
  "data": {
    "refund_no": "RF20260415110000xxx",
    "merchant_refund_no": "REFUND_001",
    "amount": 2990,
    "status": "pending"
  }
}
```

---

## 异步通知

支付成功后，平台会向商户注册的 `notify_url` 发送 **签名的 POST 请求**。

### 通知格式

```http
POST {商户notify_url}
Content-Type: application/json
X-App-Id: ap_xxx
X-Timestamp: 1712000000
X-Nonce: random-string
X-Signature: hmac-sha256-hex
X-Event-Type: payment.success
```

**通知体：**

```json
{
  "order_no": "EP20260415103000xxx",
  "merchant_order_no": "SHOP_20260415_00001",
  "channel": "wechat",
  "channel_order_no": "4200001234202604150001",
  "amount": 2990,
  "currency": "CNY",
  "status": "paid",
  "paid_at": "2026-04-15T10:31:22Z"
}
```

### 商户处理要求

1. **验签** — 使用相同的 HMAC-SHA256 算法验证 `X-Signature`，防止伪造
2. **返回 HTTP 2xx** — 平台以 HTTP 状态码判定是否接收成功
3. **幂等处理** — 同一笔订单可能收到多次通知，务必做去重

### 重试策略

未收到 2xx 响应时，平台按以下间隔重试（最多 8 次）：

| 次数 | 间隔 |
|:----:|------|
| 1 | 15 秒 |
| 2 | 1 分钟 |
| 3 | 5 分钟 |
| 4 | 15 分钟 |
| 5 | 30 分钟 |
| 6 | 1 小时 |
| 7 | 2 小时 |
| 8 | 4 小时 |

超过 8 次仍失败则标记为 `dropped`，可在商户后台手动重试。

---

## 响应格式

### 统一信封

所有接口返回统一 JSON 格式：

```json
{
  "code": "OK",
  "msg": "",
  "data": { ... }
}
```

### 错误码

| code | HTTP 状态码 | 说明 |
|------|:-----------:|------|
| `OK` | 200 | 成功 |
| `BAD_REQUEST` | 400 | 请求参数错误 |
| `AUTH_MISSING` | 401 | 缺少签名头 |
| `AUTH_INVALID` | 401 | 签名信息格式无效 |
| `AUTH_FAILED` | 401 | 签名验证失败 |
| `FORBIDDEN` | 403 | 无权限（商户被禁用 / 渠道未授权） |
| `NOT_FOUND` | 404 | 资源不存在 |
| `CONFLICT` | 409 | 状态冲突（如已支付的订单不可关闭） |

---

## 注意事项

1. **时钟同步** — 服务器校验时间戳偏差不超过 ±5 分钟，请确保服务器 NTP 同步
2. **密钥安全** — `app_secret` 不可暴露在前端代码或日志中
3. **金额单位** — 全部为**分**，1 元 = 100，请注意转换
4. **字符编码** — 请求体必须是 UTF-8 编码的 JSON，Content-Type 设为 `application/json`
5. **回调地址** — `notify_url` 必须是公网可达的 HTTPS 地址
6. **幂等设计** — 所有写入接口支持幂等：相同商户订单号 / 退款单号不会重复创建
7. **超时建议** — 建议设置 HTTP 客户端超时为 15 秒

---

## 对接检查清单

- [ ] 获取 `app_id` 和 `app_secret`
- [ ] 实现 HMAC-SHA256 签名算法并通过验证
- [ ] 调通「创建订单」接口，获取支付链接
- [ ] 实现支付回调 `notify_url` 接口
- [ ] 回调接口完成验签 + 幂等处理
- [ ] 测试订单查询、关闭、退款流程
- [ ] 确认生产环境 `notify_url` 使用 HTTPS 且公网可达
