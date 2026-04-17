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

// Registry resolves a merchant's authorised channel to the shared platform
// PaymentChannel. The platform credentials (loaded from platform_channels) are
// cached so we only decrypt once per server lifetime; call Invalidate whenever
// a platform channel config is updated via the admin UI.
type Registry struct {
	platformRepo repository.PlatformChannelRepo
	merchantRepo repository.MerchantRepo
	cipher       *crypto.AESGCM

	mu    sync.RWMutex
	cache map[model.Channel]channel.PaymentChannel
}

func New(
	platformRepo repository.PlatformChannelRepo,
	merchantRepo repository.MerchantRepo,
	cipher *crypto.AESGCM,
) *Registry {
	return &Registry{
		platformRepo: platformRepo,
		merchantRepo: merchantRepo,
		cipher:       cipher,
		cache:        make(map[model.Channel]channel.PaymentChannel),
	}
}

// Resolve checks that the merchant is authorised for ch, then returns the
// platform-level PaymentChannel (creating and caching it if necessary).
func (r *Registry) Resolve(ctx context.Context, merchantID int64, ch model.Channel) (channel.PaymentChannel, error) {
	// Verify merchant authorisation.
	if _, err := r.merchantRepo.GetMerchantChannel(ctx, merchantID, ch); err != nil {
		return nil, fmt.Errorf("merchant %d not authorised for channel %s: %w", merchantID, ch, err)
	}

	// Fast path: platform channel already initialised.
	r.mu.RLock()
	if c, ok := r.cache[ch]; ok {
		r.mu.RUnlock()
		return c, nil
	}
	r.mu.RUnlock()

	// Slow path: decrypt platform config and build the channel client.
	pc, err := r.platformRepo.Get(ctx, ch)
	if err != nil {
		return nil, fmt.Errorf("platform channel %s not configured: %w", ch, err)
	}
	plain, err := r.cipher.Decrypt(pc.Config)
	if err != nil {
		return nil, fmt.Errorf("decrypt platform channel config: %w", err)
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
	r.cache[ch] = impl
	r.mu.Unlock()
	return impl, nil
}

// Invalidate drops the cached platform channel client, forcing a re-decrypt on
// the next request. Call this after updating a platform channel config.
func (r *Registry) Invalidate(ch model.Channel) {
	r.mu.Lock()
	delete(r.cache, ch)
	r.mu.Unlock()
}
