package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/easypay/easy-pay/internal/model"
)

var ErrNotFound = errors.New("repository: not found")

// ---------- PlatformChannel ----------

// PlatformChannelRepo manages the platform-level credentials for each payment
// provider. There is one row per channel; all merchants share these credentials.
type PlatformChannelRepo interface {
	Upsert(ctx context.Context, pc *model.PlatformChannel) error
	Get(ctx context.Context, ch model.Channel) (*model.PlatformChannel, error)
	List(ctx context.Context) ([]*model.PlatformChannel, error)
}

type platformChannelRepo struct{ db *gorm.DB }

func NewPlatformChannelRepo(db *gorm.DB) PlatformChannelRepo {
	return &platformChannelRepo{db: db}
}

func (r *platformChannelRepo) Upsert(ctx context.Context, pc *model.PlatformChannel) error {
	var existing model.PlatformChannel
	err := r.db.WithContext(ctx).Where("channel = ?", pc.Channel).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.WithContext(ctx).Create(pc).Error
	}
	if err != nil {
		return err
	}
	existing.Config = pc.Config
	existing.Status = pc.Status
	existing.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(&existing).Error
}

func (r *platformChannelRepo) Get(ctx context.Context, ch model.Channel) (*model.PlatformChannel, error) {
	var pc model.PlatformChannel
	err := r.db.WithContext(ctx).Where("channel = ? AND status = 1", ch).First(&pc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &pc, err
}

func (r *platformChannelRepo) List(ctx context.Context) ([]*model.PlatformChannel, error) {
	var list []*model.PlatformChannel
	err := r.db.WithContext(ctx).Order("channel ASC").Find(&list).Error
	return list, err
}

// ---------- Merchant ----------

type MerchantRepo interface {
	Create(ctx context.Context, m *model.Merchant) error
	GetByID(ctx context.Context, id int64) (*model.Merchant, error)
	GetByAppID(ctx context.Context, appID string) (*model.Merchant, error)
	GetByEmail(ctx context.Context, email string) (*model.Merchant, error)
	List(ctx context.Context, f MerchantFilter) ([]*model.Merchant, int64, error)
	Update(ctx context.Context, m *model.Merchant) error

	// Channel authorisation — no credentials stored here.
	UpsertMerchantChannel(ctx context.Context, mc *model.MerchantChannel) error
	GetMerchantChannel(ctx context.Context, merchantID int64, ch model.Channel) (*model.MerchantChannel, error)
	ListChannels(ctx context.Context, merchantID int64) ([]*model.MerchantChannel, error)
}

// MerchantFilter drives the admin list endpoint. Keyword matches mch_no / name
// / app_id case-insensitively; Status is a pointer so we can distinguish "any"
// (nil) from the valid zero value 0 (disabled).
type MerchantFilter struct {
	Keyword string
	Status  *int16
	Offset  int
	Limit   int
}

type merchantRepo struct{ db *gorm.DB }

func NewMerchantRepo(db *gorm.DB) MerchantRepo { return &merchantRepo{db: db} }

func (r *merchantRepo) Create(ctx context.Context, m *model.Merchant) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *merchantRepo) GetByID(ctx context.Context, id int64) (*model.Merchant, error) {
	var m model.Merchant
	err := r.db.WithContext(ctx).First(&m, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &m, err
}

func (r *merchantRepo) GetByAppID(ctx context.Context, appID string) (*model.Merchant, error) {
	var m model.Merchant
	err := r.db.WithContext(ctx).Where("app_id = ?", appID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &m, err
}

func (r *merchantRepo) GetByEmail(ctx context.Context, email string) (*model.Merchant, error) {
	var m model.Merchant
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &m, err
}

func (r *merchantRepo) List(ctx context.Context, f MerchantFilter) ([]*model.Merchant, int64, error) {
	db := r.db.WithContext(ctx).Model(&model.Merchant{})
	if f.Keyword != "" {
		like := "%" + f.Keyword + "%"
		db = db.Where("mch_no ILIKE ? OR name ILIKE ? OR app_id ILIKE ?", like, like, like)
	}
	if f.Status != nil {
		db = db.Where("status = ?", *f.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*model.Merchant
	if err := db.Order("id DESC").Offset(f.Offset).Limit(f.Limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *merchantRepo) Update(ctx context.Context, m *model.Merchant) error {
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *merchantRepo) UpsertMerchantChannel(ctx context.Context, mc *model.MerchantChannel) error {
	var existing model.MerchantChannel
	err := r.db.WithContext(ctx).
		Where("merchant_id = ? AND channel = ?", mc.MerchantID, mc.Channel).
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.WithContext(ctx).Create(mc).Error
	}
	if err != nil {
		return err
	}
	existing.Status = mc.Status
	existing.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(&existing).Error
}

func (r *merchantRepo) GetMerchantChannel(ctx context.Context, merchantID int64, ch model.Channel) (*model.MerchantChannel, error) {
	var mc model.MerchantChannel
	err := r.db.WithContext(ctx).
		Where("merchant_id = ? AND channel = ? AND status = 1", merchantID, ch).
		First(&mc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &mc, err
}

func (r *merchantRepo) ListChannels(ctx context.Context, merchantID int64) ([]*model.MerchantChannel, error) {
	var list []*model.MerchantChannel
	err := r.db.WithContext(ctx).Where("merchant_id = ?", merchantID).Find(&list).Error
	return list, err
}

// ---------- Order ----------

type OrderRepo interface {
	Create(ctx context.Context, o *model.Order) error
	GetByOrderNo(ctx context.Context, orderNo string) (*model.Order, error)
	GetByMerchantOrderNo(ctx context.Context, merchantID int64, merchantOrderNo string) (*model.Order, error)
	Update(ctx context.Context, o *model.Order) error
	MarkPaid(ctx context.Context, orderNo, channelOrderNo string, paidAt time.Time) error
	List(ctx context.Context, filter OrderFilter) ([]*model.Order, int64, error)
}

type OrderFilter struct {
	MerchantID int64
	Status     model.OrderStatus
	Channel    model.Channel
	From       *time.Time
	To         *time.Time
	Offset     int
	Limit      int
}

type orderRepo struct{ db *gorm.DB }

func NewOrderRepo(db *gorm.DB) OrderRepo { return &orderRepo{db: db} }

func (r *orderRepo) Create(ctx context.Context, o *model.Order) error {
	return r.db.WithContext(ctx).Create(o).Error
}

func (r *orderRepo) GetByOrderNo(ctx context.Context, orderNo string) (*model.Order, error) {
	var o model.Order
	err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &o, err
}

func (r *orderRepo) GetByMerchantOrderNo(ctx context.Context, merchantID int64, merchantOrderNo string) (*model.Order, error) {
	var o model.Order
	err := r.db.WithContext(ctx).
		Where("merchant_id = ? AND merchant_order_no = ?", merchantID, merchantOrderNo).
		First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &o, err
}

func (r *orderRepo) Update(ctx context.Context, o *model.Order) error {
	return r.db.WithContext(ctx).Save(o).Error
}

// MarkPaid transitions pending → paid atomically. Returns ErrNotFound if the
// row is already paid or missing, which lets callback handlers stay idempotent.
func (r *orderRepo) MarkPaid(ctx context.Context, orderNo, channelOrderNo string, paidAt time.Time) error {
	res := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("order_no = ? AND status = ?", orderNo, model.OrderPending).
		Updates(map[string]any{
			"status":           model.OrderPaid,
			"channel_order_no": channelOrderNo,
			"paid_at":          paidAt,
			"updated_at":       time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *orderRepo) List(ctx context.Context, f OrderFilter) ([]*model.Order, int64, error) {
	db := r.db.WithContext(ctx).Model(&model.Order{})
	if f.MerchantID > 0 {
		db = db.Where("merchant_id = ?", f.MerchantID)
	}
	if f.Status != "" {
		db = db.Where("status = ?", f.Status)
	}
	if f.Channel != "" {
		db = db.Where("channel = ?", f.Channel)
	}
	if f.From != nil {
		db = db.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		db = db.Where("created_at < ?", *f.To)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*model.Order
	if err := db.Order("id DESC").Offset(f.Offset).Limit(f.Limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// ---------- Refund ----------

type RefundRepo interface {
	Create(ctx context.Context, r *model.RefundOrder) error
	Update(ctx context.Context, r *model.RefundOrder) error
	GetByRefundNo(ctx context.Context, refundNo string) (*model.RefundOrder, error)
	GetByMerchantRefundNo(ctx context.Context, merchantID int64, mrNo string) (*model.RefundOrder, error)
}

type refundRepo struct{ db *gorm.DB }

func NewRefundRepo(db *gorm.DB) RefundRepo { return &refundRepo{db: db} }

func (r *refundRepo) Create(ctx context.Context, ro *model.RefundOrder) error {
	return r.db.WithContext(ctx).Create(ro).Error
}
func (r *refundRepo) Update(ctx context.Context, ro *model.RefundOrder) error {
	return r.db.WithContext(ctx).Save(ro).Error
}
func (r *refundRepo) GetByRefundNo(ctx context.Context, refundNo string) (*model.RefundOrder, error) {
	var ro model.RefundOrder
	err := r.db.WithContext(ctx).Where("refund_no = ?", refundNo).First(&ro).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &ro, err
}
func (r *refundRepo) GetByMerchantRefundNo(ctx context.Context, merchantID int64, mrNo string) (*model.RefundOrder, error) {
	var ro model.RefundOrder
	err := r.db.WithContext(ctx).
		Where("merchant_id = ? AND merchant_refund_no = ?", merchantID, mrNo).
		First(&ro).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &ro, err
}

// ---------- NotifyLog ----------

type NotifyLogRepo interface {
	Create(ctx context.Context, n *model.NotifyLog) error
	Update(ctx context.Context, n *model.NotifyLog) error
	GetByID(ctx context.Context, id int64) (*model.NotifyLog, error)
	ListPendingDue(ctx context.Context, now time.Time, limit int) ([]*model.NotifyLog, error)
	ListByOrder(ctx context.Context, orderNo string) ([]*model.NotifyLog, error)
	List(ctx context.Context, f NotifyLogFilter) ([]*model.NotifyLog, int64, error)
}

// NotifyLogFilter drives the admin list endpoint. OrderNo and Status are
// optional; an empty filter returns every log in descending-id order.
// MerchantID, when non-zero, restricts results to one merchant (used by the
// self-service endpoint).
type NotifyLogFilter struct {
	MerchantID int64
	OrderNo    string
	Status     model.NotifyStatus
	Offset     int
	Limit      int
}

type notifyLogRepo struct{ db *gorm.DB }

func NewNotifyLogRepo(db *gorm.DB) NotifyLogRepo { return &notifyLogRepo{db: db} }

func (r *notifyLogRepo) Create(ctx context.Context, n *model.NotifyLog) error {
	return r.db.WithContext(ctx).Create(n).Error
}
func (r *notifyLogRepo) Update(ctx context.Context, n *model.NotifyLog) error {
	return r.db.WithContext(ctx).Save(n).Error
}
func (r *notifyLogRepo) GetByID(ctx context.Context, id int64) (*model.NotifyLog, error) {
	var n model.NotifyLog
	err := r.db.WithContext(ctx).First(&n, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &n, err
}
func (r *notifyLogRepo) ListPendingDue(ctx context.Context, now time.Time, limit int) ([]*model.NotifyLog, error) {
	var list []*model.NotifyLog
	err := r.db.WithContext(ctx).
		Where("status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", model.NotifyPending, now).
		Order("id ASC").Limit(limit).Find(&list).Error
	return list, err
}
func (r *notifyLogRepo) ListByOrder(ctx context.Context, orderNo string) ([]*model.NotifyLog, error) {
	var list []*model.NotifyLog
	err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).
		Order("id DESC").Find(&list).Error
	return list, err
}
func (r *notifyLogRepo) List(ctx context.Context, f NotifyLogFilter) ([]*model.NotifyLog, int64, error) {
	db := r.db.WithContext(ctx).Model(&model.NotifyLog{})
	if f.MerchantID > 0 {
		db = db.Where("merchant_id = ?", f.MerchantID)
	}
	if f.OrderNo != "" {
		db = db.Where("order_no = ?", f.OrderNo)
	}
	if f.Status != "" {
		db = db.Where("status = ?", f.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*model.NotifyLog
	if err := db.Order("id DESC").Offset(f.Offset).Limit(f.Limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
