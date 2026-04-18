package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/easypay/easy-pay/internal/channel/registry"
	"github.com/easypay/easy-pay/internal/config"
	"github.com/easypay/easy-pay/internal/handler/admin"
	"github.com/easypay/easy-pay/internal/handler/api"
	"github.com/easypay/easy-pay/internal/handler/callback"
	"github.com/easypay/easy-pay/internal/handler/merchant"
	testsinkh "github.com/easypay/easy-pay/internal/handler/testsink"
	"github.com/easypay/easy-pay/internal/pkg/crypto"
	"github.com/easypay/easy-pay/internal/repository"
	"github.com/easypay/easy-pay/internal/server"
	"github.com/easypay/easy-pay/internal/service/notify"
	"github.com/easypay/easy-pay/internal/service/payment"
	"github.com/easypay/easy-pay/internal/setup"
	"github.com/easypay/easy-pay/internal/testsink"
	webadmin "github.com/easypay/easy-pay/web/admin"
)

func main() {
	cfgPath := flag.String("config", "", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		if errors.Is(err, config.ErrConfigNotFound) {
			runSetupMode(*cfgPath)
			return
		}
		log.Fatalf("load config: %v", err)
	}

	runNormalMode(cfg)
}

// runSetupMode starts a minimal server that only serves the setup wizard.
func runSetupMode(cfgPath string) {
	log.Println("配置文件未找到，进入初始化向导模式...")

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	setupH := setup.New(cfgPath)

	sg := r.Group("/setup")
	{
		sg.GET("/status", setupH.Status)
		sg.POST("/test-db", setupH.TestDB)
		sg.POST("/test-redis", setupH.TestRedis)
		sg.POST("/install", setupH.Install)
	}

	// Serve the embedded admin SPA so the wizard UI works.
	staticFS, err := webadmin.Dist()
	if err == nil {
		r.NoRoute(server.SpaHandler(staticFS))
	}

	addr := ":8080"
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		log.Printf("初始化向导已启动: http://localhost%s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// runNormalMode starts the full application server.
func runNormalMode(cfg *config.Config) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// --- infra: db, redis, cipher ---
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		logger.Fatal("open db", zap.Error(err))
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Fatal("ping redis", zap.Error(err))
	}

	cipher, err := crypto.NewAESGCM(cfg.Crypto.Key)
	if err != nil {
		logger.Fatal("init cipher", zap.Error(err))
	}

	// --- repositories ---
	merchantRepo := repository.NewMerchantRepo(db)
	platformChRepo := repository.NewPlatformChannelRepo(db)
	orderRepo := repository.NewOrderRepo(db)
	refundRepo := repository.NewRefundRepo(db)
	notifyLogRepo := repository.NewNotifyLogRepo(db)

	// --- services ---
	reg := registry.New(platformChRepo, merchantRepo, cipher)

	notifySvc := notify.New(
		notifyLogRepo, merchantRepo,
		cfg.Notify.BackoffSeconds, cfg.Notify.MaxRetries, cfg.Notify.Timeout,
		logger,
	)
	notifySvc.Start(4)
	defer notifySvc.Stop()

	platformBase := getenvDefault("EASYPAY_PLATFORM_BASE", "http://localhost"+cfg.Server.Addr)
	paymentSvc := payment.NewService(orderRepo, refundRepo, reg, notifySvc, platformBase, logger)

	// --- handlers ---
	paymentH := api.NewPaymentHandler(paymentSvc)
	callbackH := callback.New(paymentSvc, reg, logger)
	adminH := admin.New(merchantRepo, platformChRepo, orderRepo, refundRepo, notifyLogRepo, cipher, reg, paymentSvc)
	adminAuthH := admin.NewAuthHandler(db, rdb)
	merchantH := merchant.New(merchantRepo, orderRepo, notifyLogRepo)
	merchantAuthH := merchant.NewAuthHandler(merchantRepo, rdb)
	sink := testsink.New(200)
	testSinkH := testsinkh.New(sink)

	// --- router / server ---
	gin.SetMode(cfg.Server.Mode)
	staticFS, err := webadmin.Dist()
	if err != nil {
		logger.Warn("embed admin dist", zap.Error(err))
	}
	r := server.NewRouter(server.Deps{
		MerchantRepo: merchantRepo,
		Payment:      paymentH,
		Callback:     callbackH,
		Admin:        adminH,
		AdminAuth:    adminAuthH,
		Merchant:     merchantH,
		MerchantAuth: merchantAuthH,
		TestSink:     testSinkH,
		StaticFS:     staticFS,
	})

	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		logger.Info("easy-pay listening", zap.String("addr", cfg.Server.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen", zap.Error(err))
		}
	}()

	// --- graceful shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
