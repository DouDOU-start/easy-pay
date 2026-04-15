package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/easypay/easy-pay/internal/model"
	"github.com/easypay/easy-pay/internal/pkg/idgen"
	"github.com/easypay/easy-pay/internal/pkg/sign"
	"github.com/easypay/easy-pay/internal/repository"
)

// Service is responsible for delivering payment events to downstream services
// over HTTP. It persists every attempt to notify_logs, signs outgoing requests
// with the merchant's app_secret, and retries failed deliveries with an
// exponential backoff schedule.
type Service struct {
	logs     repository.NotifyLogRepo
	merchant repository.MerchantRepo
	log      *zap.Logger
	client   *http.Client

	backoff    []time.Duration
	maxRetries int

	// in-process channel for immediate dispatch after enqueue; the scheduler
	// also sweeps the DB for due retries.
	queue chan int64

	wg     sync.WaitGroup
	stopCh chan struct{}
}

func New(
	logs repository.NotifyLogRepo,
	merchant repository.MerchantRepo,
	backoffSeconds []int,
	maxRetries int,
	timeout time.Duration,
	log *zap.Logger,
) *Service {
	backoff := make([]time.Duration, len(backoffSeconds))
	for i, s := range backoffSeconds {
		backoff[i] = time.Duration(s) * time.Second
	}
	return &Service{
		logs:       logs,
		merchant:   merchant,
		log:        log,
		client:     &http.Client{Timeout: timeout},
		backoff:    backoff,
		maxRetries: maxRetries,
		queue:      make(chan int64, 1024),
		stopCh:     make(chan struct{}),
	}
}

// Enqueue persists a new pending notify_log row and hands its ID to a worker.
func (s *Service) Enqueue(ctx context.Context, merchantID int64, orderNo, eventType string, payload any) error {
	m, err := s.merchant.GetByID(ctx, merchantID)
	if err != nil {
		return err
	}
	if m.NotifyURL == "" {
		s.log.Warn("merchant has no notify_url, dropping",
			zap.Int64("merchant_id", merchantID),
			zap.String("order_no", orderNo))
		return nil
	}

	body, _ := json.Marshal(payload)
	now := time.Now()
	entry := &model.NotifyLog{
		MerchantID:   merchantID,
		OrderNo:      orderNo,
		EventType:    eventType,
		NotifyURL:    m.NotifyURL,
		RequestBody:  string(body),
		Status:       model.NotifyPending,
		NextRetryAt:  &now,
	}
	if err := s.logs.Create(ctx, entry); err != nil {
		return err
	}
	select {
	case s.queue <- entry.ID:
	default:
		// queue is full; the scheduler will pick it up on the next sweep
	}
	return nil
}

// Start launches N worker goroutines plus a scheduler that sweeps pending rows
// whose next_retry_at has elapsed.
func (s *Service) Start(workers int) {
	for i := 0; i < workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}
	s.wg.Add(1)
	go s.scheduler()
}

func (s *Service) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

func (s *Service) scheduler() {
	defer s.wg.Done()
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			due, err := s.logs.ListPendingDue(ctx, time.Now(), 100)
			cancel()
			if err != nil {
				s.log.Error("notify scheduler list failed", zap.Error(err))
				continue
			}
			for _, n := range due {
				select {
				case s.queue <- n.ID:
				case <-s.stopCh:
					return
				}
			}
		}
	}
}

func (s *Service) worker() {
	defer s.wg.Done()
	for {
		select {
		case <-s.stopCh:
			return
		case id := <-s.queue:
			s.deliver(id)
		}
	}
}

func (s *Service) deliver(id int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	n, err := s.logs.GetByID(ctx, id)
	if err != nil {
		s.log.Error("load notify log failed", zap.Int64("id", id), zap.Error(err))
		return
	}
	if n.Status != model.NotifyPending {
		return
	}
	m, err := s.merchant.GetByID(ctx, n.MerchantID)
	if err != nil {
		s.log.Error("load merchant failed", zap.Error(err))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.NotifyURL, bytes.NewReader([]byte(n.RequestBody)))
	if err != nil {
		s.markFailed(ctx, n, err.Error())
		return
	}
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := idgen.Nonce()
	signature := sign.Compute(m.AppSecret, http.MethodPost, req.URL.Path, ts, nonce, []byte(n.RequestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Id", m.AppID)
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", signature)
	req.Header.Set("X-Event-Type", n.EventType)

	resp, err := s.client.Do(req)
	if err != nil {
		s.markFailed(ctx, n, err.Error())
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	n.HTTPStatus = resp.StatusCode
	n.ResponseBody = string(respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// downstream is expected to respond with body "success" or 2xx
		now := time.Now()
		n.Status = model.NotifySuccess
		n.NextRetryAt = nil
		n.UpdatedAt = now
		_ = s.logs.Update(ctx, n)
		return
	}
	s.markFailed(ctx, n, fmt.Sprintf("http %d", resp.StatusCode))
}

func (s *Service) markFailed(ctx context.Context, n *model.NotifyLog, reason string) {
	n.RetryCount++
	n.LastError = reason
	if n.RetryCount >= s.maxRetries {
		n.Status = model.NotifyDropped
		n.NextRetryAt = nil
	} else {
		idx := n.RetryCount - 1
		if idx >= len(s.backoff) {
			idx = len(s.backoff) - 1
		}
		next := time.Now().Add(s.backoff[idx])
		n.NextRetryAt = &next
	}
	_ = s.logs.Update(ctx, n)
}
