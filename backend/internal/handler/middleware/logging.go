package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/easypay/easy-pay/backend/internal/handler/httputil"
)

const CtxRequestID = "request_id"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = httputil.RandomToken()
		}
		c.Set(CtxRequestID, rid)
		c.Header("X-Request-ID", rid)
		c.Next()
	}
}

func AccessLog(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		latency := time.Since(start)
		rid, _ := c.Get(CtxRequestID)

		fields := []zap.Field{
			zap.String("request_id", rid.(string)),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
		}

		if status >= 500 {
			log.Error("request", fields...)
		} else if status >= 400 {
			log.Warn("request", fields...)
		} else {
			log.Info("request", fields...)
		}
	}
}
