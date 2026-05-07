package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/easypay/easy-pay/backend/internal/handler/admin"
	"github.com/easypay/easy-pay/backend/internal/handler/api"
	"github.com/easypay/easy-pay/backend/internal/handler/auth"
	"github.com/easypay/easy-pay/backend/internal/handler/callback"
	"github.com/easypay/easy-pay/backend/internal/handler/merchant"
	"github.com/easypay/easy-pay/backend/internal/handler/middleware"
	"github.com/easypay/easy-pay/backend/internal/repository"
	"github.com/easypay/easy-pay/backend/internal/setup"
)

type Deps struct {
	MerchantRepo repository.MerchantRepo
	Payment      *api.PaymentHandler
	Callback     *callback.Handler
	Auth         *auth.Handler
	Admin        *admin.Handler
	Merchant     *merchant.Handler
	StaticFS     fs.FS
	Logger       *zap.Logger
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.AccessLog(d.Logger))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	r.GET("/setup/status", setup.StatusCompleted)

	// ---------- provider callbacks ----------
	r.POST("/callback/:channel/:merchant_id", d.Callback.Receive)

	// ---------- all API under /api ----------

	// Auth (public)
	authGrp := r.Group("/api/auth")
	{
		authGrp.POST("/login", d.Auth.Login)
	}

	// Downstream payment API (merchant HMAC auth)
	payGrp := r.Group("/api/v1/pay")
	payGrp.Use(middleware.MerchantAuth(d.MerchantRepo, d.Logger))
	{
		payGrp.POST("/create", d.Payment.Create)
		payGrp.GET("/query", d.Payment.Query)
		payGrp.POST("/close", d.Payment.Close)
		payGrp.POST("/refund", d.Payment.Refund)
	}

	// Authenticated (any role)
	authed := r.Group("/api")
	authed.Use(d.Auth.Middleware())
	{
		authed.POST("/auth/logout", d.Auth.Logout)
		authed.GET("/auth/me", d.Auth.Me)

		// Merchant self-service
		authed.GET("/merchant/me", d.Merchant.Me)
		authed.PUT("/merchant/me", d.Merchant.UpdateProfile)
		authed.PUT("/merchant/me/password", d.Merchant.ChangePassword)
		authed.GET("/merchant/orders", d.Merchant.Orders)
		authed.GET("/merchant/orders/:order_no", d.Merchant.OrderDetail)
		authed.GET("/merchant/notify_logs", d.Merchant.NotifyLogs)
	}

	// Admin only
	adminGrp := r.Group("/api/admin")
	adminGrp.Use(d.Auth.Middleware(), auth.RequireAdmin())
	{
		adminGrp.GET("/merchants", d.Admin.ListMerchants)
		adminGrp.POST("/merchants", d.Admin.CreateMerchant)
		adminGrp.PUT("/merchants/:id", d.Admin.UpdateMerchant)
		adminGrp.DELETE("/merchants/:id", d.Admin.DeleteMerchant)
		adminGrp.POST("/merchants/:id/reset-password", d.Admin.ResetMerchantPassword)

		adminGrp.GET("/merchants/:id/channels", d.Admin.ListMerchantChannels)
		adminGrp.PUT("/merchants/:id/channels/:channel", d.Admin.UpsertMerchantChannel)

		adminGrp.GET("/platform/channels", d.Admin.ListPlatformChannels)
		adminGrp.GET("/platform/channels/:channel", d.Admin.GetPlatformChannel)
		adminGrp.PUT("/platform/channels/:channel", d.Admin.UpsertPlatformChannel)

		adminGrp.GET("/orders", d.Admin.ListOrders)
		adminGrp.POST("/orders/test", d.Admin.TestCreateOrder)
		adminGrp.POST("/orders/:order_no/refund", d.Admin.RefundOrder)
		adminGrp.POST("/wechat/parse-cert", d.Admin.ParseWechatCert)

		adminGrp.GET("/notify_logs", d.Admin.ListNotifyLogs)
		adminGrp.POST("/notify_logs/:id/retry", d.Admin.RetryNotify)

		adminGrp.GET("/settings", d.Admin.GetSettings)
		adminGrp.PUT("/settings", d.Admin.UpdateSettings)
	}

	// ---------- embedded SPA ----------
	if d.StaticFS != nil {
		r.NoRoute(SpaHandler(d.StaticFS))
	}

	return r
}

func SpaHandler(static fs.FS) gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(static))
	indexBytes, _ := fs.ReadFile(static, "index.html")

	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") ||
			strings.HasPrefix(p, "/callback/") ||
			strings.HasPrefix(p, "/setup/") {
			c.Status(http.StatusNotFound)
			return
		}

		if p != "/" {
			if f, err := static.Open(strings.TrimPrefix(p, "/")); err == nil {
				f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", indexBytes)
	}
}
