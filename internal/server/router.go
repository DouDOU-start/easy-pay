package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/easypay/easy-pay/internal/handler/admin"
	"github.com/easypay/easy-pay/internal/handler/api"
	"github.com/easypay/easy-pay/internal/handler/callback"
	"github.com/easypay/easy-pay/internal/handler/merchant"
	"github.com/easypay/easy-pay/internal/handler/middleware"
	testsinkh "github.com/easypay/easy-pay/internal/handler/testsink"
	"github.com/easypay/easy-pay/internal/repository"
	"github.com/easypay/easy-pay/internal/setup"
)

type Deps struct {
	MerchantRepo repository.MerchantRepo
	Payment      *api.PaymentHandler
	Callback     *callback.Handler
	Admin        *admin.Handler
	AdminAuth    *admin.AuthHandler
	Merchant     *merchant.Handler
	MerchantAuth *merchant.AuthHandler
	TestSink     *testsinkh.Handler
	// StaticFS serves the embedded admin SPA. When nil, no UI is served.
	StaticFS fs.FS
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Setup status — in normal mode, setup is already complete.
	r.GET("/setup/status", setup.StatusCompleted)

	// ---------- downstream payment API ----------
	apiGrp := r.Group("/api/v1/pay")
	apiGrp.Use(middleware.MerchantAuth(d.MerchantRepo))
	{
		apiGrp.POST("/create", d.Payment.Create)
		apiGrp.GET("/query", d.Payment.Query)
		apiGrp.POST("/close", d.Payment.Close)
		apiGrp.POST("/refund", d.Payment.Refund)
	}

	// ---------- provider callbacks ----------
	r.POST("/callback/:channel/:merchant_id", d.Callback.Receive)

	// ---------- dev-only test callback sink (public, any method) ----------
	if d.TestSink != nil {
		r.Any("/test/notify", d.TestSink.Receive)
		r.Any("/test/notify/*slot", d.TestSink.Receive)
	}

	// ---------- admin: auth (public) ----------
	adminPub := r.Group("/admin")
	{
		adminPub.POST("/login", d.AdminAuth.Login)
	}

	// ---------- merchant self-service: auth (public) ----------
	if d.MerchantAuth != nil && d.Merchant != nil {
		merchantPub := r.Group("/merchant")
		{
			merchantPub.POST("/login", d.MerchantAuth.Login)
		}

		// ---------- merchant self-service: authenticated ----------
		merchantGrp := r.Group("/merchant")
		merchantGrp.Use(d.MerchantAuth.Middleware())
		{
			merchantGrp.POST("/logout", d.MerchantAuth.Logout)
			merchantGrp.GET("/me", d.Merchant.Me)
			merchantGrp.PUT("/me", d.Merchant.UpdateProfile)
			merchantGrp.PUT("/me/password", d.Merchant.ChangePassword)
			merchantGrp.GET("/orders", d.Merchant.Orders)
			merchantGrp.GET("/orders/:order_no", d.Merchant.OrderDetail)
			merchantGrp.GET("/notify_logs", d.Merchant.NotifyLogs)
		}
	}

	// ---------- admin: authenticated ----------
	adminGrp := r.Group("/admin")
	adminGrp.Use(d.AdminAuth.Middleware())
	{
		adminGrp.POST("/logout", d.AdminAuth.Logout)
		adminGrp.GET("/me", d.AdminAuth.Me)

		// Merchants
		adminGrp.GET("/merchants", d.Admin.ListMerchants)
		adminGrp.POST("/merchants", d.Admin.CreateMerchant)
		adminGrp.PUT("/merchants/:id", d.Admin.UpdateMerchant)
		adminGrp.POST("/merchants/:id/reset-password", d.Admin.ResetMerchantPassword)

		// Merchant channel authorisation (no credentials — just enable/disable)
		adminGrp.GET("/merchants/:id/channels", d.Admin.ListMerchantChannels)
		adminGrp.PUT("/merchants/:id/channels/:channel", d.Admin.UpsertMerchantChannel)

		// Platform channel credentials (system-level, shared by all merchants)
		adminGrp.GET("/platform/channels", d.Admin.ListPlatformChannels)
		adminGrp.GET("/platform/channels/:channel", d.Admin.GetPlatformChannel)
		adminGrp.PUT("/platform/channels/:channel", d.Admin.UpsertPlatformChannel)

		// Orders & utilities
		adminGrp.GET("/orders", d.Admin.ListOrders)
		adminGrp.POST("/orders/test", d.Admin.TestCreateOrder)
		adminGrp.POST("/wechat/parse-cert", d.Admin.ParseWechatCert)

		// Notify logs
		adminGrp.GET("/notify_logs", d.Admin.ListNotifyLogs)
		adminGrp.POST("/notify_logs/:id/retry", d.Admin.RetryNotify)

		// Test notify sink (dev helper — reads in-memory ring buffer)
		if d.TestSink != nil {
			adminGrp.GET("/test_notify", d.TestSink.List)
			adminGrp.DELETE("/test_notify", d.TestSink.Clear)
		}
	}

	// ---------- embedded admin SPA ----------
	if d.StaticFS != nil {
		r.NoRoute(SpaHandler(d.StaticFS))
	}

	return r
}

// SpaHandler serves the embedded admin bundle. Static assets that exist in the
// FS are returned as-is; unknown paths fall through to index.html so that
// client-side routing works on deep links.
func SpaHandler(static fs.FS) gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(static))
	indexBytes, _ := fs.ReadFile(static, "index.html")

	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") ||
			strings.HasPrefix(p, "/admin/") ||
			strings.HasPrefix(p, "/merchant/") ||
			strings.HasPrefix(p, "/callback/") ||
			strings.HasPrefix(p, "/test/") ||
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
