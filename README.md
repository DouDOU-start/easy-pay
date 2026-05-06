# Easy-Pay

统一支付网关，聚合微信支付 / 支付宝，提供签名认证的 REST API 与异步回调通知。

## 部署

### 一键安装

```bash
curl -sSL https://raw.githubusercontent.com/DouDOU-start/easy-pay/master/deploy/install.sh | sudo bash
```

要求：Linux (amd64/arm64) + systemd + 已运行的 PostgreSQL 和 Redis。

安装完成后访问 `http://YOUR_IP:8080` 进入初始化向导，配置数据库连接和管理员账户。

### 管理

```bash
systemctl status easypay        # 状态
systemctl restart easypay       # 重启
journalctl -u easypay -f        # 日志
```

### 更新

重新执行安装脚本即可覆盖升级，数据和配置不会丢失。

### 卸载

```bash
curl -sSL https://raw.githubusercontent.com/DouDOU-start/easy-pay/master/deploy/install.sh | sudo bash -s -- --uninstall
```

## 本地开发

```bash
make infra    # 启动 PostgreSQL + Redis (Docker)
make dev      # API (localhost:8080) + 前端 (localhost:5173)
```

首次启动自动进入初始化向导。其他命令：

```bash
make build    # 生产构建 (单二进制，内嵌 SPA)
make up       # Docker 全栈启动
make down     # 停止
```

## API 概览

所有 `/api/v1/pay/*` 接口使用 HMAC-SHA256 签名认证。

### 签名

```
signature = hex(HMAC-SHA256(app_secret, METHOD + "\n" + PATH + "\n" + TIMESTAMP + "\n" + NONCE + "\n" + BODY))
```

请求头：`X-App-Id` / `X-Timestamp` / `X-Nonce` / `X-Signature`

### 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/pay/create` | 创建订单 |
| GET | `/api/v1/pay/query?merchant_order_no=` | 查询订单 |
| POST | `/api/v1/pay/close` | 关闭订单 |
| POST | `/api/v1/pay/refund` | 申请退款 |

### 异步通知

支付成功后向商户 `notify_url` 发送签名 POST，返回 HTTP 2xx 为成功。失败重试 8 次（15s → 4h 递增）。

完整对接文档见管理后台「文档 > 对接文档」页面。

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go · Gin · GORM · PostgreSQL · Redis |
| 前端 | React · TypeScript · Vite · Ant Design |
| 部署 | systemd / Docker Compose |

## License

MIT
