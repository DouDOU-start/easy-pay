package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/easypay/easy-pay/backend/internal/model"
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
	// Delete removes the merchant and every record that references it
	// (notify_logs, refund_orders, orders, merchant_channels) in a single
	// transaction. There is no soft-delete column on merchants, so this is
	// physical deletion — only the admin DeleteMerchant handler should call it.
	Delete(ctx context.Context, id int64) error

	// Channel authorisation — no credentials stored here.
	UpsertMerchantChannel(ctx context.Context, mc *model.MerchantChannel) error
	GetMerchantChannel(ctx context.Context, merchantID int64, ch model.Channel) (*model.MerchantChannel, error)
	ListChannels(ctx context.Context, merchantID int64) ([]*model.MerchantChannel, error)
	// DisableChannelForAll bulk-disables every merchant_channels row for the
	// given channel. Used to cascade a platform-level disable so the UI doesn't
	// keep advertising "已启用" for a channel that can no longer route traffic.
	// Returns the number of rows actually flipped (was status=1, now =0).
	DisableChannelForAll(ctx context.Context, ch model.Channel) (int64, error)
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

func (r *merchantRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// notify_logs and merchant_channels have no FK constraint blocking us,
		// but orders / refund_orders do (REFERENCES merchants(id) without
		// ON DELETE CASCADE), so children must go first.
		if err := tx.Where("merchant_id = ?", id).Delete(&model.NotifyLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("merchant_id = ?", id).Delete(&model.RefundOrder{}).Error; err != nil {
			return err
		}
		if err := tx.Where("merchant_id = ?", id).Delete(&model.Order{}).Error; err != nil {
			return err
		}
		if err := tx.Where("merchant_id = ?", id).Delete(&model.MerchantChannel{}).Error; err != nil {
			return err
		}
		res := tx.Delete(&model.Merchant{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
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

func (r *merchantRepo) DisableChannelForAll(ctx context.Context, ch model.Channel) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.MerchantChannel{}).
		Where("channel = ? AND status = 1", ch).
		Updates(map[string]any{"status": 0, "updated_at": time.Now()})
	return res.RowsAffected, res.Error
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
	CloseExpired(ctx context.Context, now time.Time, limit int) (int64, error)
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

func (r *orderRepo) CloseExpired(ctx context.Context, now time.Time, limit int) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("status = ? AND expire_at IS NOT NULL AND expire_at <= ?", model.OrderPending, now).
		Limit(limit).
		Updates(map[string]any{
			"status":     model.OrderClosed,
			"closed_at":  now,
			"updated_at": now,
		})
	return res.RowsAffected, res.Error
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
	SumRefundedAmount(ctx context.Context, orderNo string) (int64, error)
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
func (r *refundRepo) SumRefundedAmount(ctx context.Context, orderNo string) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&model.RefundOrder{}).
		Where("order_no = ? AND status = ?", orderNo, model.RefundSuccess).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	return total, err
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

// ---------- SystemSetting ----------

type SystemSettingRepo interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetAll(ctx context.Context) ([]*model.SystemSetting, error)
}

type systemSettingRepo struct{ db *gorm.DB }

func NewSystemSettingRepo(db *gorm.DB) SystemSettingRepo {
	return &systemSettingRepo{db: db}
}

func (r *systemSettingRepo) Get(ctx context.Context, key string) (string, error) {
	var s model.SystemSetting
	err := r.db.WithContext(ctx).Where(`"key" = ?`, key).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrNotFound
	}
	return s.Value, err
}

func (r *systemSettingRepo) Set(ctx context.Context, key, value string) error {
	return r.db.WithContext(ctx).
		Where(`"key" = ?`, key).
		Assign(map[string]any{"value": value, "updated_at": time.Now()}).
		FirstOrCreate(&model.SystemSetting{Key: key}).Error
}

func (r *systemSettingRepo) GetAll(ctx context.Context) ([]*model.SystemSetting, error) {
	var list []*model.SystemSetting
	err := r.db.WithContext(ctx).Order(`"key" ASC`).Find(&list).Error
	return list, err
}

// ---------- Settlement ----------

type SettlementRepo interface {
	Create(ctx context.Context, s *model.Settlement) error
	Update(ctx context.Context, s *model.Settlement) error
	GetByID(ctx context.Context, id int64) (*model.Settlement, error)
	List(ctx context.Context, f SettlementFilter) ([]*model.Settlement, int64, error)
	SumSettled(ctx context.Context, merchantID int64) (int64, error)
	LastPaidEndTime(ctx context.Context, merchantID int64) (*time.Time, error)
}

type SettlementFilter struct {
	MerchantID int64
	Status     model.SettlementStatus
	Offset     int
	Limit      int
}

type settlementRepo struct{ db *gorm.DB }

func NewSettlementRepo(db *gorm.DB) SettlementRepo { return &settlementRepo{db: db} }

func (r *settlementRepo) Create(ctx context.Context, s *model.Settlement) error {
	return r.db.WithContext(ctx).Create(s).Error
}
func (r *settlementRepo) Update(ctx context.Context, s *model.Settlement) error {
	return r.db.WithContext(ctx).Save(s).Error
}
func (r *settlementRepo) GetByID(ctx context.Context, id int64) (*model.Settlement, error) {
	var s model.Settlement
	err := r.db.WithContext(ctx).First(&s, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &s, err
}
func (r *settlementRepo) List(ctx context.Context, f SettlementFilter) ([]*model.Settlement, int64, error) {
	db := r.db.WithContext(ctx).Model(&model.Settlement{})
	if f.MerchantID > 0 {
		db = db.Where("merchant_id = ?", f.MerchantID)
	}
	if f.Status != "" {
		db = db.Where("status = ?", f.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*model.Settlement
	if err := db.Order("id DESC").Offset(f.Offset).Limit(f.Limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
func (r *settlementRepo) SumSettled(ctx context.Context, merchantID int64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&model.Settlement{}).
		Where("merchant_id = ? AND status = ?", merchantID, model.SettlementPaid).
		Select("COALESCE(SUM(net_amount), 0)").
		Scan(&total).Error
	return total, err
}
func (r *settlementRepo) LastPaidEndTime(ctx context.Context, merchantID int64) (*time.Time, error) {
	var s model.Settlement
	err := r.db.WithContext(ctx).
		Where("merchant_id = ? AND status IN ?", merchantID, []model.SettlementStatus{model.SettlementPaid, model.SettlementPending}).
		Order("period_end DESC").
		First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s.PeriodEnd, nil
}

// MerchantBalance holds the computed financial summary for a merchant.
type MerchantBalance struct {
	MerchantID   int64 `json:"merchant_id"`
	TotalIncome  int64 `json:"total_income"`
	TotalRefund  int64 `json:"total_refund"`
	TotalSettled int64 `json:"total_settled"`
	Available    int64 `json:"available"`
}

type PeriodBalance struct {
	Income int64 `json:"income"`
	Refund int64 `json:"refund"`
	Amount int64 `json:"amount"`
}

type BalanceRepo interface {
	GetBalance(ctx context.Context, merchantID int64) (*MerchantBalance, error)
	GetPeriodBalance(ctx context.Context, merchantID int64, start, end time.Time) (*PeriodBalance, error)
}

type balanceRepo struct{ db *gorm.DB }

func NewBalanceRepo(db *gorm.DB) BalanceRepo { return &balanceRepo{db: db} }

func (r *balanceRepo) GetBalance(ctx context.Context, merchantID int64) (*MerchantBalance, error) {
	var income int64
	if err := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("merchant_id = ? AND status IN ?", merchantID, []model.OrderStatus{model.OrderPaid, model.OrderRefunded, model.OrderPartialRefunded}).
		Select("COALESCE(SUM(amount), 0)").Scan(&income).Error; err != nil {
		return nil, err
	}
	var refund int64
	if err := r.db.WithContext(ctx).Model(&model.RefundOrder{}).
		Where("merchant_id = ? AND status = ?", merchantID, model.RefundSuccess).
		Select("COALESCE(SUM(amount), 0)").Scan(&refund).Error; err != nil {
		return nil, err
	}
	var settled int64
	if err := r.db.WithContext(ctx).Model(&model.Settlement{}).
		Where("merchant_id = ? AND status = ?", merchantID, model.SettlementPaid).
		Select("COALESCE(SUM(net_amount), 0)").Scan(&settled).Error; err != nil {
		return nil, err
	}
	return &MerchantBalance{
		MerchantID:   merchantID,
		TotalIncome:  income,
		TotalRefund:  refund,
		TotalSettled: settled,
		Available:    income - refund - settled,
	}, nil
}

func (r *balanceRepo) GetPeriodBalance(ctx context.Context, merchantID int64, start, end time.Time) (*PeriodBalance, error) {
	var income int64
	if err := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("merchant_id = ? AND status IN ? AND paid_at >= ? AND paid_at < ?",
			merchantID, []model.OrderStatus{model.OrderPaid, model.OrderRefunded, model.OrderPartialRefunded}, start, end).
		Select("COALESCE(SUM(amount), 0)").Scan(&income).Error; err != nil {
		return nil, err
	}
	var refund int64
	if err := r.db.WithContext(ctx).Model(&model.RefundOrder{}).
		Where("merchant_id = ? AND status = ? AND created_at >= ? AND created_at < ?",
			merchantID, model.RefundSuccess, start, end).
		Select("COALESCE(SUM(amount), 0)").Scan(&refund).Error; err != nil {
		return nil, err
	}
	amount := income - refund
	if amount < 0 {
		amount = 0
	}
	return &PeriodBalance{Income: income, Refund: refund, Amount: amount}, nil
}

// ---------- Dashboard ----------

type DailyPayment struct {
	Date   string `json:"date"`
	Amount int64  `json:"amount"`
	Count  int64  `json:"count"`
}

type DashboardRepo interface {
	TodayOrderCount(ctx context.Context, merchantID int64) (int64, error)
	TodayPaidAmount(ctx context.Context, merchantID int64) (int64, error)
	TodayRefundAmount(ctx context.Context, merchantID int64) (int64, error)
	TotalMerchantCount(ctx context.Context) (int64, error)
	TotalOrderCount(ctx context.Context, merchantID int64) (int64, error)
	TotalRevenue(ctx context.Context, merchantID int64) (int64, error)
	TotalRefund(ctx context.Context, merchantID int64) (int64, error)
	PendingSettlementAmount(ctx context.Context) (int64, error)
	Last7DaysPayments(ctx context.Context, merchantID int64) ([]DailyPayment, error)
	RecentOrders(ctx context.Context, merchantID int64, limit int) ([]*model.Order, error)
}

type dashboardRepo struct{ db *gorm.DB }

func NewDashboardRepo(db *gorm.DB) DashboardRepo { return &dashboardRepo{db: db} }

var paidStatuses = []model.OrderStatus{model.OrderPaid, model.OrderRefunded, model.OrderPartialRefunded}

func (r *dashboardRepo) scopeMerchant(db *gorm.DB, merchantID int64) *gorm.DB {
	if merchantID > 0 {
		return db.Where("merchant_id = ?", merchantID)
	}
	return db
}

func (r *dashboardRepo) TodayOrderCount(ctx context.Context, merchantID int64) (int64, error) {
	var n int64
	db := r.scopeMerchant(r.db.WithContext(ctx).Model(&model.Order{}).
		Where("paid_at >= CURRENT_DATE AND status IN ?", paidStatuses), merchantID)
	return n, db.Count(&n).Error
}
func (r *dashboardRepo) TodayPaidAmount(ctx context.Context, merchantID int64) (int64, error) {
	var n int64
	db := r.scopeMerchant(r.db.WithContext(ctx).Model(&model.Order{}).
		Where("paid_at >= CURRENT_DATE AND status IN ?", paidStatuses), merchantID)
	return n, db.Select("COALESCE(SUM(amount), 0)").Scan(&n).Error
}
func (r *dashboardRepo) TodayRefundAmount(ctx context.Context, merchantID int64) (int64, error) {
	var n int64
	db := r.scopeMerchant(r.db.WithContext(ctx).Model(&model.RefundOrder{}).
		Where("created_at >= CURRENT_DATE AND status = ?", model.RefundSuccess), merchantID)
	return n, db.Select("COALESCE(SUM(amount), 0)").Scan(&n).Error
}
func (r *dashboardRepo) TotalMerchantCount(ctx context.Context) (int64, error) {
	var n int64
	return n, r.db.WithContext(ctx).Model(&model.Merchant{}).Where("status = 1 AND role != 'admin'").Count(&n).Error
}
func (r *dashboardRepo) TotalOrderCount(ctx context.Context, merchantID int64) (int64, error) {
	var n int64
	db := r.scopeMerchant(r.db.WithContext(ctx).Model(&model.Order{}).
		Where("status IN ?", paidStatuses), merchantID)
	return n, db.Count(&n).Error
}
func (r *dashboardRepo) TotalRevenue(ctx context.Context, merchantID int64) (int64, error) {
	var n int64
	db := r.scopeMerchant(r.db.WithContext(ctx).Model(&model.Order{}).
		Where("status IN ?", paidStatuses), merchantID)
	return n, db.Select("COALESCE(SUM(amount), 0)").Scan(&n).Error
}
func (r *dashboardRepo) TotalRefund(ctx context.Context, merchantID int64) (int64, error) {
	var n int64
	db := r.scopeMerchant(r.db.WithContext(ctx).Model(&model.RefundOrder{}).
		Where("status = ?", model.RefundSuccess), merchantID)
	return n, db.Select("COALESCE(SUM(amount), 0)").Scan(&n).Error
}
func (r *dashboardRepo) PendingSettlementAmount(ctx context.Context) (int64, error) {
	var n int64
	return n, r.db.WithContext(ctx).Model(&model.Settlement{}).
		Where("status = ?", model.SettlementPending).
		Select("COALESCE(SUM(net_amount), 0)").Scan(&n).Error
}
func (r *dashboardRepo) Last7DaysPayments(ctx context.Context, merchantID int64) ([]DailyPayment, error) {
	var list []DailyPayment
	q := r.db.WithContext(ctx).Model(&model.Order{}).
		Select("DATE(paid_at) as date, COALESCE(SUM(amount), 0) as amount, COUNT(*) as count").
		Where("paid_at >= CURRENT_DATE - INTERVAL '6 days' AND status IN ?", paidStatuses)
	if merchantID > 0 {
		q = q.Where("merchant_id = ?", merchantID)
	}
	err := q.Group("DATE(paid_at)").Order("date").Find(&list).Error
	return list, err
}
func (r *dashboardRepo) RecentOrders(ctx context.Context, merchantID int64, limit int) ([]*model.Order, error) {
	var list []*model.Order
	db := r.db.WithContext(ctx).Model(&model.Order{})
	if merchantID > 0 {
		db = db.Where("merchant_id = ?", merchantID)
	}
	err := db.Order("id DESC").Limit(limit).Find(&list).Error
	return list, err
}
