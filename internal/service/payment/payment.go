package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/easypay/easy-pay/internal/channel"
	"github.com/easypay/easy-pay/internal/model"
	"github.com/easypay/easy-pay/internal/pkg/idgen"
	"github.com/easypay/easy-pay/internal/repository"
)

var (
	ErrOrderExists        = errors.New("payment: duplicate merchant_order_no")
	ErrOrderNotFound      = errors.New("payment: order not found")
	ErrInvalidStatus      = errors.New("payment: invalid status for operation")
	ErrRefundExceedAmount = errors.New("payment: refund amount exceeds remaining")
)

type Notifier interface {
	Enqueue(ctx context.Context, merchantID int64, orderNo, eventType string, payload any) error
}

type Service struct {
	orders   repository.OrderRepo
	refunds  repository.RefundRepo
	registry channel.Registry
	notifier Notifier
	log      *zap.Logger
	platformNotifyBase string // e.g. https://api.example.com
}

func NewService(
	orders repository.OrderRepo,
	refunds repository.RefundRepo,
	registry channel.Registry,
	notifier Notifier,
	platformNotifyBase string,
	log *zap.Logger,
) *Service {
	return &Service{
		orders:             orders,
		refunds:            refunds,
		registry:           registry,
		notifier:           notifier,
		platformNotifyBase: platformNotifyBase,
		log:                log,
	}
}

type CreateOrderInput struct {
	MerchantID      int64
	MerchantOrderNo string
	Channel         model.Channel
	TradeType       model.TradeType
	Subject         string
	Amount          int64
	Currency        string
	ClientIP        string
	Extra           map[string]any
	ExpireSeconds   int
}

type CreateOrderResult struct {
	OrderNo string
	CodeURL string
	H5URL   string
}

func (s *Service) CreateOrder(ctx context.Context, in CreateOrderInput) (*CreateOrderResult, error) {
	if existing, err := s.orders.GetByMerchantOrderNo(ctx, in.MerchantID, in.MerchantOrderNo); err == nil {
		// idempotent: return the existing order instead of creating a duplicate
		return &CreateOrderResult{
			OrderNo: existing.OrderNo,
			CodeURL: existing.CodeURL,
			H5URL:   existing.H5URL,
		}, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	ch, err := s.registry.Resolve(ctx, in.MerchantID, in.Channel)
	if err != nil {
		return nil, fmt.Errorf("resolve channel: %w", err)
	}

	orderNo := idgen.OrderNo("EP")
	if in.Currency == "" {
		in.Currency = "CNY"
	}

	var expireAt *time.Time
	if in.ExpireSeconds > 0 {
		t := time.Now().Add(time.Duration(in.ExpireSeconds) * time.Second)
		expireAt = &t
	}

	extraJSON, _ := json.Marshal(in.Extra)
	order := &model.Order{
		OrderNo:         orderNo,
		MerchantID:      in.MerchantID,
		MerchantOrderNo: in.MerchantOrderNo,
		Channel:         in.Channel,
		TradeType:       in.TradeType,
		Subject:         in.Subject,
		Amount:          in.Amount,
		Currency:        in.Currency,
		Status:          model.OrderPending,
		ClientIP:        in.ClientIP,
		Extra:           string(extraJSON),
		ExpireAt:        expireAt,
	}
	if err := s.orders.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	notifyURL := fmt.Sprintf("%s/callback/%s/%d", s.platformNotifyBase, in.Channel, in.MerchantID)
	res, err := ch.Prepay(ctx, channel.PrepayRequest{
		OrderNo:   orderNo,
		Subject:   in.Subject,
		Amount:    in.Amount,
		Currency:  in.Currency,
		TradeType: in.TradeType,
		ClientIP:  in.ClientIP,
		NotifyURL: notifyURL,
		ExpireAt:  expireAt,
	})
	if err != nil {
		order.Status = model.OrderFailed
		_ = s.orders.Update(ctx, order)
		return nil, fmt.Errorf("prepay: %w", err)
	}

	order.CodeURL = res.CodeURL
	order.H5URL = res.H5URL
	if err := s.orders.Update(ctx, order); err != nil {
		return nil, err
	}
	return &CreateOrderResult{OrderNo: orderNo, CodeURL: res.CodeURL, H5URL: res.H5URL}, nil
}

func (s *Service) QueryOrder(ctx context.Context, merchantID int64, merchantOrderNo string) (*model.Order, error) {
	o, err := s.orders.GetByMerchantOrderNo(ctx, merchantID, merchantOrderNo)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrOrderNotFound
	}
	return o, err
}

func (s *Service) CloseOrder(ctx context.Context, merchantID int64, merchantOrderNo string) error {
	o, err := s.orders.GetByMerchantOrderNo(ctx, merchantID, merchantOrderNo)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrOrderNotFound
	}
	if err != nil {
		return err
	}
	if o.Status != model.OrderPending {
		return ErrInvalidStatus
	}
	ch, err := s.registry.Resolve(ctx, merchantID, o.Channel)
	if err != nil {
		return err
	}
	if err := ch.Close(ctx, channel.CloseRequest{OrderNo: o.OrderNo}); err != nil {
		return err
	}
	now := time.Now()
	o.Status = model.OrderClosed
	o.ClosedAt = &now
	return s.orders.Update(ctx, o)
}

type RefundInput struct {
	MerchantID       int64
	MerchantOrderNo  string
	MerchantRefundNo string
	Amount           int64
	Reason           string
}

func (s *Service) Refund(ctx context.Context, in RefundInput) (*model.RefundOrder, error) {
	o, err := s.orders.GetByMerchantOrderNo(ctx, in.MerchantID, in.MerchantOrderNo)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	if o.Status != model.OrderPaid && o.Status != model.OrderPartialRefunded {
		return nil, ErrInvalidStatus
	}
	if in.Amount <= 0 || in.Amount > o.Amount {
		return nil, ErrRefundExceedAmount
	}

	if existing, err := s.refunds.GetByMerchantRefundNo(ctx, in.MerchantID, in.MerchantRefundNo); err == nil {
		return existing, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	ro := &model.RefundOrder{
		RefundNo:         idgen.OrderNo("RF"),
		MerchantID:       in.MerchantID,
		MerchantRefundNo: in.MerchantRefundNo,
		OrderNo:          o.OrderNo,
		Channel:          o.Channel,
		Amount:           in.Amount,
		Reason:           in.Reason,
		Status:           model.RefundPending,
	}
	if err := s.refunds.Create(ctx, ro); err != nil {
		return nil, err
	}

	ch, err := s.registry.Resolve(ctx, in.MerchantID, o.Channel)
	if err != nil {
		return nil, err
	}
	res, err := ch.Refund(ctx, channel.RefundRequest{
		OrderNo:      o.OrderNo,
		RefundNo:     ro.RefundNo,
		OriginAmount: o.Amount,
		RefundAmount: in.Amount,
		Reason:       in.Reason,
	})
	if err != nil {
		ro.Status = model.RefundFailed
		_ = s.refunds.Update(ctx, ro)
		return nil, err
	}
	ro.Status = res.Status
	ro.ChannelRefundNo = res.ChannelRefundNo
	_ = s.refunds.Update(ctx, ro)
	return ro, nil
}

// HandlePaymentNotify marks the order as paid (idempotent) and enqueues a
// downstream notification.
func (s *Service) HandlePaymentNotify(ctx context.Context, ev *channel.NotifyEvent) error {
	o, err := s.orders.GetByOrderNo(ctx, ev.OrderNo)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrOrderNotFound
	}
	if err != nil {
		return err
	}

	if o.Amount != ev.Amount {
		s.log.Warn("amount mismatch on callback",
			zap.String("order_no", ev.OrderNo),
			zap.Int64("expected", o.Amount),
			zap.Int64("got", ev.Amount))
	}

	paidAt := time.Now()
	if err := s.orders.MarkPaid(ctx, o.OrderNo, ev.ChannelOrderNo, paidAt); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// already paid → idempotent success
			return nil
		}
		return err
	}

	payload := map[string]any{
		"order_no":          o.OrderNo,
		"merchant_order_no": o.MerchantOrderNo,
		"channel":           o.Channel,
		"channel_order_no":  ev.ChannelOrderNo,
		"amount":            o.Amount,
		"currency":          o.Currency,
		"status":            string(model.OrderPaid),
		"paid_at":           paidAt.Format(time.RFC3339),
	}
	return s.notifier.Enqueue(ctx, o.MerchantID, o.OrderNo, string(channel.EventPaymentSuccess), payload)
}
