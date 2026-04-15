package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/easypay/easy-pay/internal/model"
	"github.com/easypay/easy-pay/internal/pkg/sign"
	"github.com/easypay/easy-pay/internal/repository"
)

const (
	CtxMerchant = "ctx_merchant"
)

// MerchantAuth verifies X-App-Id + HMAC signature on every request and stashes
// the resolved merchant into the gin context for downstream handlers.
func MerchantAuth(repo repository.MerchantRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.GetHeader("X-App-Id")
		ts := c.GetHeader("X-Timestamp")
		nonce := c.GetHeader("X-Nonce")
		signature := c.GetHeader("X-Signature")
		if appID == "" || ts == "" || nonce == "" || signature == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "AUTH_MISSING", "msg": "missing auth headers"})
			return
		}

		m, err := repo.GetByAppID(c.Request.Context(), appID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "AUTH_INVALID", "msg": "unknown app_id"})
			return
		}
		if m.Status != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "MERCHANT_DISABLED", "msg": "merchant disabled"})
			return
		}

		body, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		if err := sign.Verify(m.AppSecret, c.Request.Method, c.Request.URL.Path, ts, nonce, signature, body); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "AUTH_INVALID", "msg": err.Error()})
			return
		}

		c.Set(CtxMerchant, m)
		c.Next()
	}
}

func GetMerchant(c *gin.Context) *model.Merchant {
	v, _ := c.Get(CtxMerchant)
	if m, ok := v.(*model.Merchant); ok {
		return m
	}
	return nil
}
