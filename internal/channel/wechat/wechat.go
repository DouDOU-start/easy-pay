package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/h5"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/services/refunddomestic"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"

	"github.com/easypay/easy-pay/internal/channel"
	"github.com/easypay/easy-pay/internal/model"
)

// Config is the per-merchant configuration persisted (AES-encrypted) in
// merchant_channels.config.
//
// All fields are required. easy-pay uses the WeChat Pay public-key verification
// path exclusively — the legacy platform-certificate download path was dropped
// because WeChat returns 404 RESOURCE_NOT_EXISTS for merchants registered in
// 2024+. Download the public key from 商户平台 → 账户中心 → API 安全 →
// 验证微信支付身份 → 微信支付公钥.
type Config struct {
	MchID         string `json:"mch_id"`
	AppID         string `json:"app_id"`
	APIV3Key      string `json:"api_v3_key"`
	SerialNo      string `json:"serial_no"`
	PrivateKeyPEM string `json:"private_key_pem"`
	PublicKeyID   string `json:"public_key_id"`
	PublicKeyPEM  string `json:"public_key_pem"`
}

type Channel struct {
	cfg     Config
	client  *core.Client
	native  *native.NativeApiService
	h5      *h5.H5ApiService
	refund  *refunddomestic.RefundsApiService
	handler *notify.Handler
}

// New constructs a WeChat Pay v3 client using the public-key verification
// path. Called lazily by the registry the first time a merchant requests this
// channel; the result is cached.
func New(ctx context.Context, raw json.RawMessage) (*Channel, error) {
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("wechat: unmarshal config: %w", err)
	}
	if c.MchID == "" || c.AppID == "" || c.APIV3Key == "" || c.SerialNo == "" ||
		c.PrivateKeyPEM == "" || c.PublicKeyID == "" || c.PublicKeyPEM == "" {
		return nil, errors.New("wechat: incomplete config (mch_id/app_id/api_v3_key/serial_no/private_key_pem/public_key_id/public_key_pem required)")
	}

	privateKey, err := utils.LoadPrivateKey(c.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("wechat: load private key: %w", err)
	}
	publicKey, err := utils.LoadPublicKey(c.PublicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("wechat: load wechat-pay public key: %w", err)
	}

	client, err := core.NewClient(ctx, option.WithWechatPayPublicKeyAuthCipher(
		c.MchID, c.SerialNo, privateKey, c.PublicKeyID, publicKey,
	))
	if err != nil {
		return nil, fmt.Errorf("wechat: new client: %w", err)
	}

	handler, err := notify.NewRSANotifyHandler(
		c.APIV3Key,
		verifiers.NewSHA256WithRSAPubkeyVerifier(c.PublicKeyID, *publicKey),
	)
	if err != nil {
		return nil, fmt.Errorf("wechat: new notify handler: %w", err)
	}

	return &Channel{
		cfg:     c,
		client:  client,
		native:  &native.NativeApiService{Client: client},
		h5:      &h5.H5ApiService{Client: client},
		refund:  &refunddomestic.RefundsApiService{Client: client},
		handler: handler,
	}, nil
}

func (c *Channel) Name() model.Channel { return model.ChannelWechat }

func (c *Channel) Prepay(ctx context.Context, req channel.PrepayRequest) (*channel.PrepayResult, error) {
	currency := req.Currency
	if currency == "" {
		currency = "CNY"
	}

	switch req.TradeType {
	case model.TradeTypeNative:
		nativeReq := native.PrepayRequest{
			Appid:       core.String(c.cfg.AppID),
			Mchid:       core.String(c.cfg.MchID),
			Description: core.String(req.Subject),
			OutTradeNo:  core.String(req.OrderNo),
			NotifyUrl:   core.String(req.NotifyURL),
			Amount: &native.Amount{
				Total:    core.Int64(req.Amount),
				Currency: core.String(currency),
			},
		}
		if req.ExpireAt != nil {
			nativeReq.TimeExpire = req.ExpireAt
		}
		resp, _, err := c.native.Prepay(ctx, nativeReq)
		if err != nil {
			return nil, fmt.Errorf("wechat native prepay: %w", err)
		}
		return &channel.PrepayResult{CodeURL: strVal(resp.CodeUrl)}, nil

	case model.TradeTypeH5:
		h5Req := h5.PrepayRequest{
			Appid:       core.String(c.cfg.AppID),
			Mchid:       core.String(c.cfg.MchID),
			Description: core.String(req.Subject),
			OutTradeNo:  core.String(req.OrderNo),
			NotifyUrl:   core.String(req.NotifyURL),
			Amount: &h5.Amount{
				Total:    core.Int64(req.Amount),
				Currency: core.String(currency),
			},
			SceneInfo: &h5.SceneInfo{
				PayerClientIp: core.String(req.ClientIP),
				H5Info: &h5.H5Info{
					Type: core.String("Wap"),
				},
			},
		}
		if req.ExpireAt != nil {
			h5Req.TimeExpire = req.ExpireAt
		}
		resp, _, err := c.h5.Prepay(ctx, h5Req)
		if err != nil {
			return nil, fmt.Errorf("wechat h5 prepay: %w", err)
		}
		return &channel.PrepayResult{H5URL: strVal(resp.H5Url)}, nil
	}
	return nil, channel.ErrUnsupported
}

func (c *Channel) Query(ctx context.Context, req channel.QueryRequest) (*channel.QueryResult, error) {
	resp, _, err := c.native.QueryOrderByOutTradeNo(ctx, native.QueryOrderByOutTradeNoRequest{
		OutTradeNo: core.String(req.OrderNo),
		Mchid:      core.String(c.cfg.MchID),
	})
	if err != nil {
		return nil, fmt.Errorf("wechat query: %w", err)
	}
	out := &channel.QueryResult{
		Status:         mapTradeState(strVal(resp.TradeState)),
		ChannelOrderNo: strVal(resp.TransactionId),
	}
	if resp.Amount != nil && resp.Amount.Total != nil {
		out.Amount = *resp.Amount.Total
	}
	return out, nil
}

func (c *Channel) Close(ctx context.Context, req channel.CloseRequest) error {
	if _, err := c.native.CloseOrder(ctx, native.CloseOrderRequest{
		OutTradeNo: core.String(req.OrderNo),
		Mchid:      core.String(c.cfg.MchID),
	}); err != nil {
		return fmt.Errorf("wechat close: %w", err)
	}
	return nil
}

func (c *Channel) Refund(ctx context.Context, req channel.RefundRequest) (*channel.RefundResult, error) {
	createReq := refunddomestic.CreateRequest{
		OutTradeNo:  core.String(req.OrderNo),
		OutRefundNo: core.String(req.RefundNo),
		Reason:      core.String(req.Reason),
		Amount: &refunddomestic.AmountReq{
			Refund:   core.Int64(req.RefundAmount),
			Total:    core.Int64(req.OriginAmount),
			Currency: core.String("CNY"),
		},
	}
	if req.NotifyURL != "" {
		createReq.NotifyUrl = core.String(req.NotifyURL)
	}
	resp, _, err := c.refund.Create(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("wechat refund: %w", err)
	}
	return &channel.RefundResult{
		Status:          mapRefundStatus(string(*resp.Status)),
		ChannelRefundNo: strVal(resp.RefundId),
	}, nil
}

// ParseNotify verifies the V3 signature, decrypts the resource envelope, and
// returns a normalised payment event. The caller (callback handler) is
// responsible for short-circuiting on duplicate orders.
func (c *Channel) ParseNotify(ctx context.Context, r *http.Request) (*channel.NotifyEvent, error) {
	transaction := new(payments.Transaction)
	if _, err := c.handler.ParseNotifyRequest(ctx, r, transaction); err != nil {
		return nil, fmt.Errorf("wechat parse notify: %w", err)
	}
	ev := &channel.NotifyEvent{
		Type:           channel.EventPaymentSuccess,
		OrderNo:        strVal(transaction.OutTradeNo),
		ChannelOrderNo: strVal(transaction.TransactionId),
	}
	if transaction.Amount != nil && transaction.Amount.Total != nil {
		ev.Amount = *transaction.Amount.Total
	}
	if transaction.SuccessTime != nil {
		ev.PaidAt = *transaction.SuccessTime
	}
	return ev, nil
}

func (c *Channel) NotifyAck() (string, []byte) {
	return "application/json", []byte(`{"code":"SUCCESS","message":"OK"}`)
}

func mapTradeState(s string) model.OrderStatus {
	switch s {
	case "SUCCESS":
		return model.OrderPaid
	case "CLOSED", "REVOKED":
		return model.OrderClosed
	case "NOTPAY", "USERPAYING", "ACCEPT":
		return model.OrderPending
	case "REFUND":
		return model.OrderRefunded
	case "PAYERROR":
		return model.OrderFailed
	default:
		return model.OrderPending
	}
}

func mapRefundStatus(s string) model.RefundStatus {
	switch s {
	case "SUCCESS":
		return model.RefundSuccess
	case "ABNORMAL", "CLOSED":
		return model.RefundFailed
	default:
		return model.RefundPending
	}
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
