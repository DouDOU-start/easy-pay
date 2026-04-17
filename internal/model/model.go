package model

import (
	"time"
)

type OrderStatus string

const (
	OrderPending         OrderStatus = "pending"
	OrderPaid            OrderStatus = "paid"
	OrderClosed          OrderStatus = "closed"
	OrderRefunded        OrderStatus = "refunded"
	OrderPartialRefunded OrderStatus = "partial_refunded"
	OrderFailed          OrderStatus = "failed"
)

type RefundStatus string

const (
	RefundPending RefundStatus = "pending"
	RefundSuccess RefundStatus = "success"
	RefundFailed  RefundStatus = "failed"
)

type NotifyStatus string

const (
	NotifyPending NotifyStatus = "pending"
	NotifySuccess NotifyStatus = "success"
	NotifyFailed  NotifyStatus = "failed"
	NotifyDropped NotifyStatus = "dropped"
)

type Channel string

const (
	ChannelWechat Channel = "wechat"
	ChannelAlipay Channel = "alipay"
)

type TradeType string

const (
	TradeTypeNative TradeType = "native"
	TradeTypeH5     TradeType = "h5"
)

type Merchant struct {
	ID                int64      `gorm:"primaryKey" json:"id"`
	MchNo             string     `gorm:"column:mch_no;uniqueIndex;size:32" json:"mch_no"`
	Name              string     `gorm:"size:128" json:"name"`
	Email             string     `gorm:"size:128" json:"email"`
	PasswordHash      string     `gorm:"column:password_hash;size:128" json:"-"`
	PasswordChangedAt *time.Time `gorm:"column:password_changed_at" json:"password_changed_at"`
	AppID             string     `gorm:"column:app_id;uniqueIndex;size:64" json:"app_id"`
	AppSecret         string     `gorm:"column:app_secret;size:128" json:"-"`
	NotifyURL         string     `gorm:"column:notify_url;size:512" json:"notify_url"`
	Status            int16      `json:"status"`
	Remark            string     `gorm:"size:256" json:"remark"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (Merchant) TableName() string { return "merchants" }

// PlatformChannel holds the platform-level credentials for a payment provider.
// There is one row per channel (wechat, alipay). All downstream merchants share
// these credentials; per-merchant authorisation is in MerchantChannel.
type PlatformChannel struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Channel   Channel   `gorm:"size:16;uniqueIndex" json:"channel"`
	Config    string    `json:"-"` // AES-GCM encrypted JSON — same schema as before
	Status    int16     `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (PlatformChannel) TableName() string { return "platform_channels" }

// MerchantChannel records which channels a merchant is authorised to use.
// No credentials are stored here; the actual keys live in PlatformChannel.
type MerchantChannel struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	MerchantID int64     `gorm:"column:merchant_id;index" json:"merchant_id"`
	Channel    Channel   `gorm:"size:16" json:"channel"`
	Status     int16     `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (MerchantChannel) TableName() string { return "merchant_channels" }

type Order struct {
	ID              int64       `gorm:"primaryKey" json:"id"`
	OrderNo         string      `gorm:"column:order_no;uniqueIndex;size:40" json:"order_no"`
	MerchantID      int64       `gorm:"column:merchant_id;index" json:"merchant_id"`
	MerchantOrderNo string      `gorm:"column:merchant_order_no;size:64" json:"merchant_order_no"`
	Channel         Channel     `gorm:"size:16" json:"channel"`
	ChannelOrderNo  string      `gorm:"column:channel_order_no;size:64" json:"channel_order_no"`
	TradeType       TradeType   `gorm:"column:trade_type;size:16" json:"trade_type"`
	Subject         string      `gorm:"size:256" json:"subject"`
	Amount          int64       `json:"amount"`
	Currency        string      `gorm:"size:8" json:"currency"`
	Status          OrderStatus `gorm:"size:16;index" json:"status"`
	ClientIP        string      `gorm:"column:client_ip;size:64" json:"client_ip"`
	Extra           string      `gorm:"type:jsonb" json:"extra"`
	CodeURL         string      `gorm:"column:code_url;size:512" json:"code_url"`
	H5URL           string      `gorm:"column:h5_url;size:512" json:"h5_url"`
	ExpireAt        *time.Time  `json:"expire_at"`
	PaidAt          *time.Time  `json:"paid_at"`
	ClosedAt        *time.Time  `json:"closed_at"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

func (Order) TableName() string { return "orders" }

type RefundOrder struct {
	ID               int64        `gorm:"primaryKey" json:"id"`
	RefundNo         string       `gorm:"column:refund_no;uniqueIndex;size:40" json:"refund_no"`
	MerchantID       int64        `gorm:"column:merchant_id;index" json:"merchant_id"`
	MerchantRefundNo string       `gorm:"column:merchant_refund_no;size:64" json:"merchant_refund_no"`
	OrderNo          string       `gorm:"column:order_no;size:40;index" json:"order_no"`
	Channel          Channel      `gorm:"size:16" json:"channel"`
	ChannelRefundNo  string       `gorm:"column:channel_refund_no;size:64" json:"channel_refund_no"`
	Amount           int64        `json:"amount"`
	Reason           string       `gorm:"size:256" json:"reason"`
	Status           RefundStatus `gorm:"size:16" json:"status"`
	RefundedAt       *time.Time   `json:"refunded_at"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}

func (RefundOrder) TableName() string { return "refund_orders" }

type NotifyLog struct {
	ID           int64        `gorm:"primaryKey" json:"id"`
	MerchantID   int64        `gorm:"column:merchant_id" json:"merchant_id"`
	OrderNo      string       `gorm:"column:order_no;size:40;index" json:"order_no"`
	EventType    string       `gorm:"column:event_type;size:32" json:"event_type"`
	NotifyURL    string       `gorm:"column:notify_url;size:512" json:"notify_url"`
	RequestBody  string       `gorm:"column:request_body" json:"request_body"`
	ResponseBody string       `gorm:"column:response_body" json:"response_body"`
	HTTPStatus   int          `gorm:"column:http_status" json:"http_status"`
	Status       NotifyStatus `gorm:"size:16;index" json:"status"`
	RetryCount   int          `gorm:"column:retry_count" json:"retry_count"`
	NextRetryAt  *time.Time   `gorm:"column:next_retry_at" json:"next_retry_at"`
	LastError    string       `gorm:"column:last_error;size:512" json:"last_error"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

func (NotifyLog) TableName() string { return "notify_logs" }

type AdminUser struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64" json:"username"`
	PasswordHash string    `gorm:"column:password_hash;size:128" json:"-"`
	Role         string    `gorm:"size:16" json:"role"`
	Status       int16     `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (AdminUser) TableName() string { return "admin_users" }
