package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/easypay/easy-pay/backend/internal/channel"
	"github.com/easypay/easy-pay/backend/internal/model"
	"github.com/easypay/easy-pay/backend/internal/pkg/idgen"
	"github.com/easypay/easy-pay/backend/internal/repository"
)

var (
	ErrOrderExists        = errors.New("payment: duplicate merchant_order_no")
	ErrOrderNotFound      = errors.New("payment: order not found")
	ErrInvalidStatus      = errors.New("payment: invalid status for operation")
	ErrRefundExceedAmount = errors.New("payment: refund amount exceeds remaining")
	ErrAmountMismatch     = errors.New("payment: callback amount does not match order")
)

type Notifier interface {
	Enqueue(ctx context.Context, merchantID int64, orderNo, eventType, notifyURL string, payload any) error
}

type PlatformBaseGetter func(ctx context.Context) string

type Service struct {
	orders             repository.OrderRepo
	refunds            repository.RefundRepo
	registry           channel.Registry
	notifier           Notifier
	log                *zap.Logger
	platformBaseGetter PlatformBaseGetter

	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewService(
	orders repository.OrderRepo,
	refunds repository.RefundRepo,
	registry channel.Registry,
	notifier Notifier,
	platformBaseGetter PlatformBaseGetter,
	log *zap.Logger,
) *Service {
	return &Service{
		orders:             orders,
		refunds:            refunds,
		registry:           registry,
		notifier:           notifier,
		platformBaseGetter: platformBaseGetter,
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
	NotifyURL       string
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
		s.log.Debug("order idempotent hit",
			zap.Int64("merchant_id", in.MerchantID),
			zap.String("merchant_order_no", in.MerchantOrderNo),
			zap.String("order_no", existing.OrderNo))
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
		s.log.Error("resolve channel failed",
			zap.Int64("merchant_id", in.MerchantID),
			zap.String("channel", string(in.Channel)),
			zap.Error(err))
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
		NotifyURL:       in.NotifyURL,
		Extra:           string(extraJSON),
		ExpireAt:        expireAt,
	}
	if err := s.orders.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	notifyURL := fmt.Sprintf("%s/callback/%s/%d", s.platformBaseGetter(ctx), in.Channel, in.MerchantID)
	s.log.Info("calling channel prepay",
		zap.String("order_no", orderNo),
		zap.String("channel", string(in.Channel)),
		zap.String("trade_type", string(in.TradeType)),
		zap.Int64("amount", in.Amount))
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
		s.log.Error("channel prepay failed",
			zap.String("order_no", orderNo),
			zap.String("channel", string(in.Channel)),
			zap.Error(err))
		order.Status = model.OrderFailed
		_ = s.orders.Update(ctx, order)
		return nil, fmt.Errorf("prepay: %w", err)
	}

	order.CodeURL = res.CodeURL
	order.H5URL = res.H5URL
	if err := s.orders.Update(ctx, order); err != nil {
		return nil, err
	}
	s.log.Info("order created successfully",
		zap.String("order_no", orderNo),
		zap.Int64("merchant_id", in.MerchantID),
		zap.String("channel", string(in.Channel)))
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
	refunded, err := s.refunds.SumRefundedAmount(ctx, o.OrderNo)
	if err != nil {
		return nil, err
	}
	if in.Amount <= 0 || refunded+in.Amount > o.Amount {
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
		s.log.Error("refund resolve channel failed",
			zap.String("refund_no", ro.RefundNo),
			zap.Error(err))
		return nil, err
	}
	s.log.Info("calling channel refund",
		zap.String("refund_no", ro.RefundNo),
		zap.String("order_no", o.OrderNo),
		zap.String("channel", string(o.Channel)),
		zap.Int64("amount", in.Amount),
		zap.Int64("already_refunded", refunded))
	refundNotifyURL := fmt.Sprintf("%s/callback/refund/%s/%d", s.platformBaseGetter(ctx), o.Channel, in.MerchantID)
	res, err := ch.Refund(ctx, channel.RefundRequest{
		OrderNo:      o.OrderNo,
		RefundNo:     ro.RefundNo,
		OriginAmount: o.Amount,
		RefundAmount: in.Amount,
		Reason:       in.Reason,
		NotifyURL:    refundNotifyURL,
	})
	if err != nil {
		s.log.Error("channel refund failed",
			zap.String("refund_no", ro.RefundNo),
			zap.String("order_no", o.OrderNo),
			zap.Error(err))
		ro.Status = model.RefundFailed
		_ = s.refunds.Update(ctx, ro)
		return nil, err
	}
	ro.Status = res.Status
	ro.ChannelRefundNo = res.ChannelRefundNo
	_ = s.refunds.Update(ctx, ro)
	s.log.Info("refund result",
		zap.String("refund_no", ro.RefundNo),
		zap.String("order_no", o.OrderNo),
		zap.String("status", string(ro.Status)),
		zap.String("channel_refund_no", ro.ChannelRefundNo))

	if ro.Status == model.RefundSuccess {
		if refunded+in.Amount >= o.Amount {
			o.Status = model.OrderRefunded
		} else {
			o.Status = model.OrderPartialRefunded
		}
		_ = s.orders.Update(ctx, o)
		s.log.Info("order status updated after refund",
			zap.String("order_no", o.OrderNo),
			zap.String("status", string(o.Status)))
	}

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
		s.log.Error("amount mismatch on callback, rejecting",
			zap.String("order_no", ev.OrderNo),
			zap.Int64("expected", o.Amount),
			zap.Int64("got", ev.Amount))
		return ErrAmountMismatch
	}

	paidAt := time.Now()
	if err := s.orders.MarkPaid(ctx, o.OrderNo, ev.ChannelOrderNo, paidAt); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.log.Debug("payment notify idempotent",
				zap.String("order_no", ev.OrderNo))
			return nil
		}
		return err
	}
	s.log.Info("order paid",
		zap.String("order_no", o.OrderNo),
		zap.Int64("merchant_id", o.MerchantID),
		zap.String("channel", string(o.Channel)),
		zap.String("channel_order_no", ev.ChannelOrderNo),
		zap.Int64("amount", o.Amount))

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
	return s.notifier.Enqueue(ctx, o.MerchantID, o.OrderNo, string(channel.EventPaymentSuccess), o.NotifyURL, payload)
}

func (s *Service) HandleRefundNotify(ctx context.Context, ev *channel.RefundNotifyEvent) error {
	ro, err := s.refunds.GetByRefundNo(ctx, ev.RefundNo)
	if err != nil {
		return fmt.Errorf("refund not found: %s: %w", ev.RefundNo, err)
	}
	if ro.Status == ev.Status {
		return nil
	}

	ro.Status = ev.Status
	ro.ChannelRefundNo = ev.ChannelRefundNo
	if err := s.refunds.Update(ctx, ro); err != nil {
		return err
	}
	s.log.Info("refund notify processed",
		zap.String("refund_no", ro.RefundNo),
		zap.String("order_no", ro.OrderNo),
		zap.String("status", string(ro.Status)))

	if ro.Status == model.RefundSuccess {
		o, err := s.orders.GetByOrderNo(ctx, ro.OrderNo)
		if err != nil {
			return err
		}
		refunded, err := s.refunds.SumRefundedAmount(ctx, o.OrderNo)
		if err != nil {
			return err
		}
		if refunded >= o.Amount {
			o.Status = model.OrderRefunded
		} else {
			o.Status = model.OrderPartialRefunded
		}
		_ = s.orders.Update(ctx, o)
		s.log.Info("order status updated after refund notify",
			zap.String("order_no", o.OrderNo),
			zap.String("status", string(o.Status)))
	}
	return nil
}

func (s *Service) StartExpireScheduler() {
	s.stopCh = make(chan struct{})
	s.wg.Add(1)
	go s.expireLoop()
}

func (s *Service) StopExpireScheduler() {
	if s.stopCh != nil {
		close(s.stopCh)
		s.wg.Wait()
	}
}

func (s *Service) expireLoop() {
	defer s.wg.Done()
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			n, err := s.orders.CloseExpired(ctx, time.Now(), 200)
			cancel()
			if err != nil {
				s.log.Error("close expired orders failed", zap.Error(err))
			} else if n > 0 {
				s.log.Info("closed expired orders", zap.Int64("count", n))
			}
		}
	}
}
