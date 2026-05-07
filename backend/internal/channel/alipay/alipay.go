package alipay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/smartwalle/alipay/v3"

	"github.com/easypay/easy-pay/backend/internal/channel"
	"github.com/easypay/easy-pay/backend/internal/model"
)

// Config is the per-merchant configuration persisted (AES-encrypted) in
// merchant_channels.config.
type Config struct {
	AppID           string `json:"app_id"`
	PrivateKey      string `json:"private_key"`       // app private key (PKCS1/PKCS8 PEM)
	AlipayPublicKey string `json:"alipay_public_key"` // alipay platform public key
	SignType        string `json:"sign_type"`         // RSA2
	IsProduction    bool   `json:"is_production"`
}

type Channel struct {
	cfg    Config
	client *alipay.Client
}

func New(_ context.Context, raw json.RawMessage) (*Channel, error) {
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	if c.SignType == "" {
		c.SignType = "RSA2"
	}
	if c.AppID == "" || c.PrivateKey == "" || c.AlipayPublicKey == "" {
		return nil, errors.New("alipay: incomplete config (app_id/private_key/alipay_public_key required)")
	}

	client, err := alipay.New(c.AppID, c.PrivateKey, c.IsProduction)
	if err != nil {
		return nil, fmt.Errorf("alipay: new client: %w", err)
	}
	if err := client.LoadAliPayPublicKey(c.AlipayPublicKey); err != nil {
		return nil, fmt.Errorf("alipay: load public key: %w", err)
	}

	return &Channel{cfg: c, client: client}, nil
}

func (c *Channel) Name() model.Channel { return model.ChannelAlipay }

// Prepay is the integration point for smartwalle/alipay.
// Native → alipay.TradePreCreate; H5 → alipay.TradeWapPay URL.
// TODO: replace the placeholder once the SDK client is wired.
func (c *Channel) Prepay(ctx context.Context, req channel.PrepayRequest) (*channel.PrepayResult, error) {
	_ = ctx
	switch req.TradeType {
	case model.TradeTypeNative:
		return &channel.PrepayResult{
			CodeURL: "https://qr.alipay.com/PLACEHOLDER_" + req.OrderNo,
		}, nil
	case model.TradeTypeH5:
		return &channel.PrepayResult{
			H5URL: "https://openapi.alipay.com/gateway.do?PLACEHOLDER_" + req.OrderNo,
		}, nil
	default:
		return nil, channel.ErrUnsupported
	}
}

func (c *Channel) Query(ctx context.Context, req channel.QueryRequest) (*channel.QueryResult, error) {
	_ = ctx
	_ = req
	return &channel.QueryResult{Status: model.OrderPending}, nil
}

func (c *Channel) Close(ctx context.Context, req channel.CloseRequest) error {
	_ = ctx
	_ = req
	return nil
}

func (c *Channel) Refund(ctx context.Context, req channel.RefundRequest) (*channel.RefundResult, error) {
	_ = ctx
	return &channel.RefundResult{
		Status:          model.RefundPending,
		ChannelRefundNo: "PLACEHOLDER_" + req.RefundNo,
	}, nil
}

func (c *Channel) ParseNotify(ctx context.Context, r *http.Request) (*channel.NotifyEvent, error) {
	_ = ctx
	noti, err := c.client.GetTradeNotification(r)
	if err != nil {
		return nil, fmt.Errorf("alipay: signature verification failed: %w", err)
	}
	if noti == nil {
		return nil, errors.New("alipay: empty notification")
	}
	if noti.TradeStatus != alipay.TradeStatusSuccess && noti.TradeStatus != alipay.TradeStatusFinished {
		return nil, fmt.Errorf("alipay: unexpected trade_status %s", noti.TradeStatus)
	}
	amount, _ := strconv.ParseFloat(noti.TotalAmount, 64)
	return &channel.NotifyEvent{
		Type:           channel.EventPaymentSuccess,
		OrderNo:        noti.OutTradeNo,
		ChannelOrderNo: noti.TradeNo,
		Amount:         int64(amount * 100),
		PaidAt:         noti.GmtPayment,
	}, nil
}

func (c *Channel) NotifyAck() (string, []byte) {
	return "text/plain", []byte("success")
}
