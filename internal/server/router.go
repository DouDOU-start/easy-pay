package server

import (
	"github.com/gin-gonic/gin"

	"github.com/easypay/easy-pay/internal/handler/admin"
	"github.com/easypay/easy-pay/internal/handler/api"
	"github.com/easypay/easy-pay/internal/handler/callback"
	"github.com/easypay/easy-pay/internal/handler/middleware"
	"github.com/easypay/easy-pay/internal/repository"
)

type Deps struct {
	MerchantRepo repository.MerchantRepo
	Payment      *api.PaymentHandler
	Callback     *callback.Handler
	Admin        *admin.Handler
	AdminAuth    *admin.AuthHandler
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

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

	// ---------- admin ----------
	adminPub := r.Group("/admin")
	{
		adminPub.POST("/login", d.AdminAuth.Login)
	}
	adminGrp := r.Group("/admin")
	adminGrp.Use(d.AdminAuth.Middleware())
	{
		adminGrp.POST("/logout", d.AdminAuth.Logout)
		adminGrp.GET("/me", d.AdminAuth.Me)

		adminGrp.GET("/merchants", d.Admin.ListMerchants)
		adminGrp.POST("/merchants", d.Admin.CreateMerchant)
		adminGrp.PUT("/merchants/:id", d.Admin.UpdateMerchant)
		adminGrp.GET("/merchants/:id/channels", d.Admin.ListMerchantChannels)
		adminGrp.GET("/merchants/:id/channels/:channel", d.Admin.GetMerchantChannel)
		adminGrp.PUT("/merchants/:id/channels", d.Admin.UpsertMerchantChannel)

		adminGrp.GET("/orders", d.Admin.ListOrders)
		adminGrp.POST("/orders/test", d.Admin.TestCreateOrder)
		adminGrp.POST("/wechat/parse-cert", d.Admin.ParseWechatCert)

		adminGrp.GET("/notify_logs", d.Admin.ListNotifyLogs)
		adminGrp.POST("/notify_logs/:id/retry", d.Admin.RetryNotify)
	}

	return r
}
