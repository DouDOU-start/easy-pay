package channel

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/easypay/easy-pay/internal/model"
)

// PaymentChannel is the unified abstraction implemented by each payment provider
// (wechat, alipay, ...). The service layer is the only caller; handlers stay
// free of provider-specific concerns.
type PaymentChannel interface {
	Name() model.Channel

	// Prepay creates a transaction on the provider side and returns either
	// a code_url (Native) or an h5 redirect url.
	Prepay(ctx context.Context, req PrepayRequest) (*PrepayResult, error)

	// Query fetches the current state of a transaction from the provider.
	Query(ctx context.Context, req QueryRequest) (*QueryResult, error)

	// Close asks the provider to mark the order as closed (cannot be paid anymore).
	Close(ctx context.Context, req CloseRequest) error

	// Refund requests a refund against a paid transaction.
	Refund(ctx context.Context, req RefundRequest) (*RefundResult, error)

	// ParseNotify verifies the callback signature and returns a normalised event.
	ParseNotify(ctx context.Context, r *http.Request) (*NotifyEvent, error)

	// NotifyAck returns the provider-expected body for a successful callback ack.
	NotifyAck() (contentType string, body []byte)
}

type PrepayRequest struct {
	OrderNo   string
	Subject   string
	Amount    int64  // cents
	Currency  string // CNY
	TradeType model.TradeType
	ClientIP  string
	NotifyURL string     // platform callback url, provider → us
	ReturnURL string     // optional (h5)
	ExpireAt  *time.Time // forwarded as time_expire to wechat / it_b_pay to alipay
	Extra     map[string]string
}

type PrepayResult struct {
	CodeURL string
	H5URL   string
	Raw     map[string]any
}

type QueryRequest struct {
	OrderNo        string
	ChannelOrderNo string
}

type QueryResult struct {
	Status         model.OrderStatus
	ChannelOrderNo string
	Amount         int64
	Raw            map[string]any
}

type CloseRequest struct {
	OrderNo string
}

type RefundRequest struct {
	OrderNo       string
	RefundNo      string
	OriginAmount  int64
	RefundAmount  int64
	Reason        string
	NotifyURL     string
}

type RefundResult struct {
	Status          model.RefundStatus
	ChannelRefundNo string
	Raw             map[string]any
}

type NotifyEventType string

const (
	EventPaymentSuccess NotifyEventType = "payment.success"
	EventRefundSuccess  NotifyEventType = "refund.success"
)

type NotifyEvent struct {
	Type           NotifyEventType
	OrderNo        string
	ChannelOrderNo string
	Amount         int64
	PaidAt         string
	Raw            map[string]any
}

var (
	ErrUnsupported = errors.New("channel: operation not supported")
	ErrNotFound    = errors.New("channel: not found")
)

// Registry is a lookup from (merchantID, channel) to a configured PaymentChannel.
// The payment service resolves channels through the registry so that business
// logic stays unaware of provider wiring.
type Registry interface {
	Resolve(ctx context.Context, merchantID int64, ch model.Channel) (PaymentChannel, error)
}
