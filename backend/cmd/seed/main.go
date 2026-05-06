package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/easypay/easy-pay/backend/internal/config"
	"github.com/easypay/easy-pay/backend/internal/model"
	"github.com/easypay/easy-pay/backend/internal/pkg/crypto"
)

func main() {
	cfgPath := flag.String("config", "backend/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	cipher, err := crypto.NewAESGCM(cfg.Crypto.Key)
	if err != nil {
		log.Fatalf("init cipher: %v", err)
	}

	seedPlatformChannels(db, cipher)
	merchantIDs := seedMerchants(db)
	seedMerchantChannels(db, merchantIDs)
	orderNos := seedOrders(db, merchantIDs)
	seedRefunds(db, merchantIDs, orderNos)
	seedNotifyLogs(db, merchantIDs, orderNos)

	log.Println("seed data inserted successfully")
}

func seedPlatformChannels(db *gorm.DB, cipher *crypto.AESGCM) {
	wxCfg, _ := json.Marshal(map[string]string{
		"MchID":         "1900000001",
		"AppID":         "wx_demo_appid_00001",
		"APIV3Key":      "demo-apiv3-key-32bytes-padding!",
		"SerialNo":      "ABCDEF1234567890",
		"PrivateKeyPEM": "-----BEGIN RSA PRIVATE KEY-----\nMIIBogIBAAJBALRiMLAH...demo...==\n-----END RSA PRIVATE KEY-----",
		"PublicKeyID":   "PUB_KEY_ID_001",
		"PublicKeyPEM":  "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0B...demo...==\n-----END PUBLIC KEY-----",
	})
	wxEnc, _ := cipher.Encrypt(wxCfg)

	aliCfg, _ := json.Marshal(map[string]string{
		"AppID":           "2021000000000001",
		"PrivateKey":      "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...demo...==\n-----END RSA PRIVATE KEY-----",
		"AlipayPublicKey": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0B...demo...==\n-----END PUBLIC KEY-----",
		"SignType":        "RSA2",
		"IsProduction":    "false",
	})
	aliEnc, _ := cipher.Encrypt(aliCfg)

	channels := []model.PlatformChannel{
		{Channel: model.ChannelWechat, Config: wxEnc, Status: 1},
		{Channel: model.ChannelAlipay, Config: aliEnc, Status: 1},
	}
	for _, ch := range channels {
		db.Where("channel = ?", ch.Channel).FirstOrCreate(&ch)
	}
	log.Println("  ✓ platform_channels")
}

func seedMerchants(db *gorm.DB) []int64 {
	type mchSeed struct {
		name   string
		email  string
		remark string
		status int16
	}

	seeds := []mchSeed{
		{"星巴克测试店", "starbucks@test.com", "咖啡连锁", 1},
		{"瑞幸咖啡", "luckin@test.com", "互联网咖啡", 1},
		{"美团外卖", "meituan@test.com", "外卖平台", 1},
		{"滴滴出行", "didi@test.com", "出行服务", 1},
		{"拼多多商城", "pdd@test.com", "电商平台", 0},
	}

	pwdHash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	var ids []int64

	for i, s := range seeds {
		m := model.Merchant{
			MchNo:        fmt.Sprintf("MCH%04d", i+1),
			Name:         s.name,
			Email:        s.email,
			PasswordHash: string(pwdHash),
			AppID:        fmt.Sprintf("app_%s", uuid.NewString()[:12]),
			AppSecret:    uuid.NewString() + uuid.NewString(),
			NotifyURL:    "",
			Status:       s.status,
			Remark:       s.remark,
		}
		result := db.Where("mch_no = ?", m.MchNo).FirstOrCreate(&m)
		if result.Error != nil {
			log.Fatalf("insert merchant %s: %v", s.name, result.Error)
		}
		ids = append(ids, m.ID)
	}
	log.Println("  ✓ merchants")
	return ids
}

func seedMerchantChannels(db *gorm.DB, merchantIDs []int64) {
	for _, mid := range merchantIDs {
		for _, ch := range []model.Channel{model.ChannelWechat, model.ChannelAlipay} {
			mc := model.MerchantChannel{
				MerchantID: mid,
				Channel:    ch,
				Status:     1,
			}
			db.Where("merchant_id = ? AND channel = ?", mid, ch).FirstOrCreate(&mc)
		}
	}
	log.Println("  ✓ merchant_channels")
}

func seedOrders(db *gorm.DB, merchantIDs []int64) []string {
	subjects := []string{
		"大杯拿铁", "摩卡星冰乐", "抹茶拿铁",
		"外卖订单-麻辣烫", "外卖订单-黄焖鸡", "外卖订单-沙县小吃",
		"快车订单", "专车订单", "顺风车订单",
		"iPhone 16 Pro Max", "AirPods Pro 2", "MacBook Air M4",
		"会员充值-月卡", "会员充值-年卡", "话费充值100元",
	}
	statuses := []model.OrderStatus{
		model.OrderPaid, model.OrderPaid, model.OrderPaid, model.OrderPaid, model.OrderPaid,
		model.OrderPending, model.OrderPending,
		model.OrderClosed,
		model.OrderRefunded,
		model.OrderPartialRefunded,
		model.OrderFailed,
	}
	channels := []model.Channel{model.ChannelWechat, model.ChannelAlipay}
	tradeTypes := []model.TradeType{model.TradeTypeNative, model.TradeTypeH5}
	clientIPs := []string{"192.168.1.100", "10.0.0.5", "172.16.0.88", "203.0.113.42", "100.64.0.1"}

	var orderNos []string
	now := time.Now()

	for i := 0; i < 30; i++ {
		mid := merchantIDs[i%len(merchantIDs)]
		status := statuses[i%len(statuses)]
		ch := channels[i%len(channels)]
		tt := tradeTypes[i%len(tradeTypes)]
		subject := subjects[i%len(subjects)]
		amount := int64((rand.Intn(9990) + 10)) // 0.10 ~ 99.99 元

		orderNo := fmt.Sprintf("EP%s%04d", now.Format("20060102"), i+1)
		mchOrderNo := fmt.Sprintf("OUT%s%04d", now.Format("20060102"), i+1)
		createdAt := now.Add(-time.Duration(30-i) * time.Hour)

		o := model.Order{
			OrderNo:         orderNo,
			MerchantID:      mid,
			MerchantOrderNo: mchOrderNo,
			Channel:         ch,
			TradeType:       tt,
			Subject:         subject,
			Amount:          amount,
			Currency:        "CNY",
			Status:          status,
			ClientIP:        clientIPs[i%len(clientIPs)],
			Extra:           "{}",
			CreatedAt:       createdAt,
			UpdatedAt:       createdAt,
		}

		if status == model.OrderPaid || status == model.OrderRefunded || status == model.OrderPartialRefunded {
			paidAt := createdAt.Add(time.Duration(rand.Intn(300)+10) * time.Second)
			o.PaidAt = &paidAt
			o.ChannelOrderNo = fmt.Sprintf("4200%010d", rand.Int63n(9999999999))
		}
		if status == model.OrderClosed {
			closedAt := createdAt.Add(30 * time.Minute)
			o.ClosedAt = &closedAt
		}
		if tt == model.TradeTypeNative {
			o.CodeURL = fmt.Sprintf("weixin://wxpay/bizpayurl?pr=%08x", rand.Int31())
		}

		expireAt := createdAt.Add(30 * time.Minute)
		o.ExpireAt = &expireAt

		result := db.Where("order_no = ?", o.OrderNo).FirstOrCreate(&o)
		if result.Error != nil {
			log.Fatalf("insert order %s: %v", orderNo, result.Error)
		}
		orderNos = append(orderNos, orderNo)
	}
	log.Println("  ✓ orders (30)")
	return orderNos
}

func seedRefunds(db *gorm.DB, merchantIDs []int64, orderNos []string) {
	now := time.Now()
	refundReasons := []string{
		"商品质量问题", "用户取消订单", "重复支付", "配送超时", "商品缺货",
	}

	paidOrders := []string{}
	var orders []model.Order
	db.Where("status IN ?", []string{"paid", "refunded", "partial_refunded"}).Find(&orders)
	for _, o := range orders {
		paidOrders = append(paidOrders, o.OrderNo)
	}
	if len(paidOrders) == 0 {
		paidOrders = orderNos[:5]
	}

	for i := 0; i < 8; i++ {
		orderNo := paidOrders[i%len(paidOrders)]
		var order model.Order
		db.Where("order_no = ?", orderNo).First(&order)

		refundNo := fmt.Sprintf("RF%s%04d", now.Format("20060102"), i+1)
		mchRefundNo := fmt.Sprintf("MRF%s%04d", now.Format("20060102"), i+1)
		amount := order.Amount / 2
		if amount == 0 {
			amount = 100
		}
		createdAt := now.Add(-time.Duration(8-i) * time.Hour)

		status := model.RefundSuccess
		if i%4 == 1 {
			status = model.RefundPending
		}
		if i%4 == 3 {
			status = model.RefundFailed
		}

		r := model.RefundOrder{
			RefundNo:         refundNo,
			MerchantID:       order.MerchantID,
			MerchantRefundNo: mchRefundNo,
			OrderNo:          orderNo,
			Channel:          order.Channel,
			Amount:           amount,
			Reason:           refundReasons[i%len(refundReasons)],
			Status:           status,
			CreatedAt:        createdAt,
			UpdatedAt:        createdAt,
		}
		if status == model.RefundSuccess {
			refundedAt := createdAt.Add(5 * time.Minute)
			r.RefundedAt = &refundedAt
			r.ChannelRefundNo = fmt.Sprintf("5000%010d", rand.Int63n(9999999999))
		}

		db.Where("refund_no = ?", r.RefundNo).FirstOrCreate(&r)
	}
	log.Println("  ✓ refund_orders (8)")
}

func seedNotifyLogs(db *gorm.DB, merchantIDs []int64, orderNos []string) {
	now := time.Now()

	for i := 0; i < 15; i++ {
		orderNo := orderNos[i%len(orderNos)]
		mid := merchantIDs[i%len(merchantIDs)]
		createdAt := now.Add(-time.Duration(15-i) * time.Hour)

		eventType := "payment.success"
		if i%5 == 0 {
			eventType = "refund.success"
		}

		status := model.NotifySuccess
		httpStatus := 200
		respBody := `{"code":"SUCCESS"}`
		retryCount := 0
		lastError := ""

		switch i % 5 {
		case 1:
			status = model.NotifyFailed
			httpStatus = 500
			respBody = "Internal Server Error"
			retryCount = 8
			lastError = "max retries reached"
		case 2:
			status = model.NotifyPending
			httpStatus = 0
			respBody = ""
			retryCount = 2
			lastError = "connection refused"
		case 3:
			status = model.NotifyDropped
			httpStatus = 0
			respBody = ""
			retryCount = 0
			lastError = "notify_url is empty"
		}

		reqBody := fmt.Sprintf(`{"order_no":"%s","event":"%s","amount":1990}`, orderNo, eventType)

		nl := model.NotifyLog{
			MerchantID:   mid,
			OrderNo:      orderNo,
			EventType:    eventType,
			NotifyURL:    "",
			RequestBody:  reqBody,
			ResponseBody: respBody,
			HTTPStatus:   httpStatus,
			Status:       status,
			RetryCount:   retryCount,
			LastError:    lastError,
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		}
		db.Create(&nl)
	}
	log.Println("  ✓ notify_logs (15)")
}
