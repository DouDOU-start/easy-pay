package main

import (
	"context"
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
	"github.com/easypay/easy-pay/internal/pkg/crypto"
	"github.com/easypay/easy-pay/internal/repository"
	"github.com/easypay/easy-pay/internal/server"
	"github.com/easypay/easy-pay/internal/service/notify"
	"github.com/easypay/easy-pay/internal/service/payment"
)

func main() {
	cfgPath := flag.String("config", "", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// --- infra: db, redis, cipher ---
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Info),
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
	orderRepo := repository.NewOrderRepo(db)
	refundRepo := repository.NewRefundRepo(db)
	notifyLogRepo := repository.NewNotifyLogRepo(db)

	// --- services ---
	reg := registry.New(merchantRepo, cipher)

	notifySvc := notify.New(
		notifyLogRepo, merchantRepo,
		cfg.Notify.BackoffSeconds, cfg.Notify.MaxRetries, cfg.Notify.Timeout,
		logger,
	)
	notifySvc.Start(4)
	defer notifySvc.Stop()

	platformBase := getenvDefault("EASYPAY_PLATFORM_BASE", "http://localhost"+cfg.Server.Addr)
	paymentSvc := payment.NewService(orderRepo, refundRepo, reg, notifySvc, platformBase, logger)

	// --- seed default admin if empty ---
	defaultUser := getenvDefault("EASYPAY_ADMIN_USER", "admin")
	defaultPass := getenvDefault("EASYPAY_ADMIN_PASS", "admin123")
	if err := admin.SeedAdmin(context.Background(), db, defaultUser, defaultPass); err != nil {
		logger.Warn("seed admin failed", zap.Error(err))
	}

	// --- handlers ---
	paymentH := api.NewPaymentHandler(paymentSvc)
	callbackH := callback.New(paymentSvc, reg, logger)
	adminH := admin.New(merchantRepo, orderRepo, refundRepo, notifyLogRepo, cipher, reg, paymentSvc)
	adminAuthH := admin.NewAuthHandler(db, rdb)

	// --- router / server ---
	gin.SetMode(cfg.Server.Mode)
	r := server.NewRouter(server.Deps{
		MerchantRepo: merchantRepo,
		Payment:      paymentH,
		Callback:     callbackH,
		Admin:        adminH,
		AdminAuth:    adminAuthH,
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
