package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/easypay/easy-pay/backend/internal/model"
	"github.com/easypay/easy-pay/backend/internal/pkg/sign"
	"github.com/easypay/easy-pay/backend/internal/repository"
)

const (
	CtxMerchant = "ctx_merchant"
)

func MerchantAuth(repo repository.MerchantRepo, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.GetHeader("X-App-Id")
		ts := c.GetHeader("X-Timestamp")
		nonce := c.GetHeader("X-Nonce")
		signature := c.GetHeader("X-Signature")
		if appID == "" || ts == "" || nonce == "" || signature == "" {
			log.Warn("merchant auth missing headers",
				zap.String("client_ip", c.ClientIP()),
				zap.String("path", c.Request.URL.Path))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "AUTH_MISSING", "msg": "missing auth headers"})
			return
		}

		m, err := repo.GetByAppID(c.Request.Context(), appID)
		if err != nil {
			log.Warn("merchant auth unknown app_id",
				zap.String("app_id", appID),
				zap.String("client_ip", c.ClientIP()))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "AUTH_INVALID", "msg": "unknown app_id"})
			return
		}
		if m.Status != 1 {
			log.Warn("merchant auth disabled",
				zap.String("app_id", appID),
				zap.Int64("merchant_id", m.ID))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "MERCHANT_DISABLED", "msg": "merchant disabled"})
			return
		}

		body, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		if err := sign.Verify(m.AppSecret, c.Request.Method, c.Request.URL.Path, ts, nonce, signature, body); err != nil {
			log.Warn("merchant auth signature invalid",
				zap.String("app_id", appID),
				zap.Int64("merchant_id", m.ID),
				zap.String("client_ip", c.ClientIP()),
				zap.Error(err))
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
