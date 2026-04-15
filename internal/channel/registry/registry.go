package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/easypay/easy-pay/internal/channel"
	"github.com/easypay/easy-pay/internal/channel/alipay"
	"github.com/easypay/easy-pay/internal/channel/wechat"
	"github.com/easypay/easy-pay/internal/model"
	"github.com/easypay/easy-pay/internal/pkg/crypto"
	"github.com/easypay/easy-pay/internal/repository"
)

// Registry resolves (merchantID, channel) to a PaymentChannel.
// Results are cached so we don't decrypt/parse on every request.
// TODO: add TTL or invalidate on merchant_channel updates.
type Registry struct {
	repo   repository.MerchantRepo
	cipher *crypto.AESGCM

	mu    sync.RWMutex
	cache map[string]channel.PaymentChannel
}

func New(repo repository.MerchantRepo, cipher *crypto.AESGCM) *Registry {
	return &Registry{
		repo:   repo,
		cipher: cipher,
		cache:  make(map[string]channel.PaymentChannel),
	}
}

func (r *Registry) Resolve(ctx context.Context, merchantID int64, ch model.Channel) (channel.PaymentChannel, error) {
	key := fmt.Sprintf("%d:%s", merchantID, ch)

	r.mu.RLock()
	if c, ok := r.cache[key]; ok {
		r.mu.RUnlock()
		return c, nil
	}
	r.mu.RUnlock()

	mc, err := r.repo.GetChannelConfig(ctx, merchantID, ch)
	if err != nil {
		return nil, err
	}
	plain, err := r.cipher.Decrypt(mc.Config)
	if err != nil {
		return nil, fmt.Errorf("decrypt channel config: %w", err)
	}

	var impl channel.PaymentChannel
	switch ch {
	case model.ChannelWechat:
		impl, err = wechat.New(ctx, json.RawMessage(plain))
	case model.ChannelAlipay:
		impl, err = alipay.New(ctx, json.RawMessage(plain))
	default:
		return nil, fmt.Errorf("unknown channel: %s", ch)
	}
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.cache[key] = impl
	r.mu.Unlock()
	return impl, nil
}

func (r *Registry) Invalidate(merchantID int64, ch model.Channel) {
	r.mu.Lock()
	delete(r.cache, fmt.Sprintf("%d:%s", merchantID, ch))
	r.mu.Unlock()
}
