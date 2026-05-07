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

func AccessLog(logger *zap.Logger) gin.HandlerFunc {
	al := logger.WithOptions(zap.WithCaller(false), zap.AddStacktrace(zap.DPanicLevel))
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
			al.Error("request", fields...)
		} else if status >= 400 {
			al.Warn("request", fields...)
		} else {
			al.Info("request", fields...)
		}
	}
}
