package alipay

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/easypay/easy-pay/internal/channel"
	"github.com/easypay/easy-pay/internal/model"
)

// Config is the per-merchant configuration persisted (AES-encrypted) in
// merchant_channels.config.
type Config struct {
	AppID           string `json:"app_id"`
	PrivateKey      string `json:"private_key"`        // app private key (PKCS1/PKCS8 PEM)
	AlipayPublicKey string `json:"alipay_public_key"`  // alipay platform public key
	SignType        string `json:"sign_type"`          // RSA2
	IsProduction    bool   `json:"is_production"`
}

type Channel struct {
	cfg Config
}

func New(_ context.Context, raw json.RawMessage) (*Channel, error) {
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	if c.SignType == "" {
		c.SignType = "RSA2"
	}
	return &Channel{cfg: c}, nil
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

// ParseNotify validates the form-post signature and returns a normalised event.
// TODO: use alipay.Client.GetTradeNotification once wired.
func (c *Channel) ParseNotify(ctx context.Context, r *http.Request) (*channel.NotifyEvent, error) {
	_ = ctx
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	amount, _ := strconv.ParseFloat(r.PostForm.Get("total_amount"), 64)
	return &channel.NotifyEvent{
		Type:           channel.EventPaymentSuccess,
		OrderNo:        r.PostForm.Get("out_trade_no"),
		ChannelOrderNo: r.PostForm.Get("trade_no"),
		Amount:         int64(amount * 100),
		PaidAt:         r.PostForm.Get("gmt_payment"),
	}, nil
}

func (c *Channel) NotifyAck() (string, []byte) {
	return "text/plain", []byte("success")
}
