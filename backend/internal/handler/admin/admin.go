package admin

import (
	cryptorand "crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/easypay/easy-pay/backend/internal/channel/registry"
	"github.com/easypay/easy-pay/backend/internal/handler/httputil"
	"github.com/easypay/easy-pay/backend/internal/model"
	"github.com/easypay/easy-pay/backend/internal/pkg/crypto"
	"github.com/easypay/easy-pay/backend/internal/pkg/idgen"
	"github.com/easypay/easy-pay/backend/internal/repository"
	"github.com/easypay/easy-pay/backend/internal/service/payment"
)

type Handler struct {
	merchants   repository.MerchantRepo
	platformChs repository.PlatformChannelRepo
	orders      repository.OrderRepo
	refunds     repository.RefundRepo
	logs        repository.NotifyLogRepo
	settlements repository.SettlementRepo
	balances    repository.BalanceRepo
	dashboard   repository.DashboardRepo
	cipher      *crypto.AESGCM
	registry    *registry.Registry
	paymentSvc  *payment.Service
	settings    repository.SystemSettingRepo
	log         *zap.Logger
}

func New(
	merchants repository.MerchantRepo,
	platformChs repository.PlatformChannelRepo,
	orders repository.OrderRepo,
	refunds repository.RefundRepo,
	logs repository.NotifyLogRepo,
	settlements repository.SettlementRepo,
	balances repository.BalanceRepo,
	dashboard repository.DashboardRepo,
	cipher *crypto.AESGCM,
	reg *registry.Registry,
	paymentSvc *payment.Service,
	settings repository.SystemSettingRepo,
	log *zap.Logger,
) *Handler {
	return &Handler{
		merchants:   merchants,
		platformChs: platformChs,
		orders:      orders,
		refunds:     refunds,
		logs:        logs,
		settlements: settlements,
		balances:    balances,
		dashboard:   dashboard,
		cipher:      cipher,
		registry:    reg,
		paymentSvc:  paymentSvc,
		settings:    settings,
		log:         log,
	}
}

// ---------- Dashboard ----------

func (h *Handler) Dashboard(c *gin.Context) {
	ctx := c.Request.Context()
	var mid int64
	if v := c.Query("merchant_id"); v != "" {
		mid, _ = strconv.ParseInt(v, 10, 64)
	}
	todayCount, _ := h.dashboard.TodayOrderCount(ctx, mid)
	todayPaid, _ := h.dashboard.TodayPaidAmount(ctx, mid)
	todayRefund, _ := h.dashboard.TodayRefundAmount(ctx, mid)
	totalMerchants, _ := h.dashboard.TotalMerchantCount(ctx)
	totalOrders, _ := h.dashboard.TotalOrderCount(ctx, mid)
	totalRevenue, _ := h.dashboard.TotalRevenue(ctx, mid)
	pendingSettlement, _ := h.dashboard.PendingSettlementAmount(ctx)
	trend, _ := h.dashboard.Last7DaysPayments(ctx, mid)
	recentOrders, _ := h.dashboard.RecentOrders(ctx, mid, 10)
	if trend == nil {
		trend = []repository.DailyPayment{}
	}
	if recentOrders == nil {
		recentOrders = []*model.Order{}
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"today": gin.H{
			"order_count":   todayCount,
			"paid_amount":   todayPaid,
			"refund_amount": todayRefund,
		},
		"overall": gin.H{
			"total_merchants": totalMerchants,
			"total_orders":    totalOrders,
			"total_revenue":   totalRevenue,
		},
		"pending_settlement": pendingSettlement,
		"trend":              trend,
		"recent_orders":      recentOrders,
	}})
}

// ---------- Merchants ----------

type createMerchantReq struct {
	Name      string `json:"name" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	NotifyURL string `json:"notify_url"`
	Remark    string `json:"remark"`
}

func (h *Handler) CreateMerchant(c *gin.Context) {
	var req createMerchantReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if existing, err := h.merchants.GetByEmail(c.Request.Context(), email); err == nil && existing != nil {
		c.JSON(http.StatusConflict, gin.H{"code": "EMAIL_TAKEN", "msg": "该邮箱已被其它商户使用"})
		return
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		httputil.Fail500(c, "CREATE_FAILED", "操作失败，请稍后重试", err)
		return
	}
	password, err := randomPassword(12)
	if err != nil {
		httputil.Fail500(c, "CREATE_FAILED", "操作失败，请稍后重试", err)
		return
	}
	pwHash, err := HashPassword(password)
	if err != nil {
		httputil.Fail500(c, "CREATE_FAILED", "操作失败，请稍后重试", err)
		return
	}
	now := time.Now()
	m := &model.Merchant{
		MchNo:             generateMchNo(),
		Name:              req.Name,
		Email:             email,
		PasswordHash:      pwHash,
		PasswordChangedAt: &now,
		AppID:             "ap_" + uuid.NewString()[:12],
		AppSecret:         uuid.NewString() + uuid.NewString(),
		NotifyURL:         req.NotifyURL,
		Status:            1,
		Remark:            req.Remark,
	}
	if err := h.merchants.Create(c.Request.Context(), m); err != nil {
		httputil.Fail500(c, "CREATE_FAILED", "操作失败，请稍后重试", err)
		return
	}
	h.log.Info("merchant created",
		zap.Int64("id", m.ID),
		zap.String("mch_no", m.MchNo),
		zap.String("email", m.Email))
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"id":         m.ID,
		"mch_no":     m.MchNo,
		"name":       m.Name,
		"email":      m.Email,
		"app_id":     m.AppID,
		"app_secret": m.AppSecret,
		"password":   password,
	}})
}

// randomPassword returns an n-character printable token without ambiguous
// characters (0/O, 1/l/I). n must be >= 8.
func randomPassword(n int) (string, error) {
	const alphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, n)
	raw := make([]byte, n)
	if _, err := cryptorand.Read(raw); err != nil {
		return "", err
	}
	for i, b := range raw {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}

// generateMchNo produces a merchant number like "M" + 10-digit timestamp suffix
// + 4 random digits, e.g. "M17131472853891".
func generateMchNo() string {
	ts := time.Now().UnixNano()
	// Last 10 digits of nanosecond timestamp + 4 random digits.
	rnd := make([]byte, 2)
	_, _ = cryptorand.Read(rnd)
	r := (uint16(rnd[0])<<8 | uint16(rnd[1])) % 10000
	return fmt.Sprintf("M%010d%04d", ts%1e10, r)
}

func (h *Handler) ListMerchants(c *gin.Context) {
	page, size := httputil.ParsePage(c)
	filter := repository.MerchantFilter{
		Keyword: strings.TrimSpace(c.Query("keyword")),
		Offset:  (page - 1) * size,
		Limit:   size,
	}
	if s := c.Query("status"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			st := int16(v)
			filter.Status = &st
		}
	}
	list, total, err := h.merchants.List(c.Request.Context(), filter)
	if err != nil {
		httputil.Fail500(c, "LIST_FAILED", "查询失败，请稍后重试", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"list": list, "total": total, "page": page, "size": size,
	}})
}

type updateMerchantReq struct {
	Name      string `json:"name"`
	NotifyURL string `json:"notify_url"`
	Remark    string `json:"remark"`
	Status    *int16 `json:"status"`
}

func (h *Handler) UpdateMerchant(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	m, err := h.merchants.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	}
	var req updateMerchantReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	if req.Name != "" {
		m.Name = req.Name
	}
	if req.NotifyURL != "" {
		m.NotifyURL = req.NotifyURL
	}
	m.Remark = req.Remark
	if req.Status != nil {
		m.Status = *req.Status
	}
	if err := h.merchants.Update(c.Request.Context(), m); err != nil {
		httputil.Fail500(c, "UPDATE_FAILED", "更新失败，请稍后重试", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": m})
}

// DeleteMerchant physically removes a merchant and every record that
// references it (orders, refund_orders, notify_logs, merchant_channels). The
// random typed-back confirmation lives entirely in the admin UI; the API
// itself is gated by admin auth alone.
func (h *Handler) DeleteMerchant(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if _, err := h.merchants.GetByID(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	}
	if err := h.merchants.Delete(c.Request.Context(), id); err != nil {
		httputil.Fail500(c, "DELETE_FAILED", "删除失败，请稍后重试", err)
		return
	}
	h.log.Warn("merchant deleted", zap.Int64("id", id))
	c.JSON(http.StatusOK, gin.H{"code": "OK"})
}

// ResetMerchantPassword generates a new random password for a merchant and
// returns the plaintext once. The merchant must use this to log in and can
// change it afterwards.
func (h *Handler) ResetMerchantPassword(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	m, err := h.merchants.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	}
	password, err := randomPassword(12)
	if err != nil {
		httputil.Fail500(c, "RESET_FAILED", "操作失败，请稍后重试", err)
		return
	}
	pwHash, err := HashPassword(password)
	if err != nil {
		httputil.Fail500(c, "RESET_FAILED", "操作失败，请稍后重试", err)
		return
	}
	now := time.Now()
	m.PasswordHash = pwHash
	m.PasswordChangedAt = &now
	if err := h.merchants.Update(c.Request.Context(), m); err != nil {
		httputil.Fail500(c, "RESET_FAILED", "操作失败，请稍后重试", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"email":    m.Email,
		"password": password,
	}})
}

// ---------- Merchant channel authorisation ----------
// No credentials are managed here — only which channels the merchant may use.

type upsertMerchantChannelReq struct {
	Status int16 `json:"status"`
}

func (h *Handler) UpsertMerchantChannel(c *gin.Context) {
	merchantID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ch := model.Channel(c.Param("channel"))
	if ch != model.ChannelWechat && ch != model.ChannelAlipay {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_CHANNEL"})
		return
	}
	var req upsertMerchantChannelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	// Block authorising a channel the platform itself can't service yet —
	// otherwise the merchant looks "已启用" in the UI but every API call would
	// fail at runtime. Disabling (status=0) always goes through.
	if req.Status == 1 {
		pc, err := h.platformChs.Get(c.Request.Context(), ch)
		if err != nil || pc == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code": "PLATFORM_NOT_CONFIGURED",
				"msg":  "平台「" + channelDisplayName(ch) + "」凭证尚未配置或已停用，无法授权该渠道。请先在「渠道凭证」中完成配置。",
			})
			return
		}
		ok, err := h.platformChannelConfigured(pc)
		if err != nil {
			httputil.Fail500(c, "BAD_CONFIG", "配置格式错误，请稍后重试", err)
			return
		}
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"code": "PLATFORM_NOT_CONFIGURED",
				"msg":  "平台「" + channelDisplayName(ch) + "」凭证不完整或已停用，无法授权该渠道。请先在「渠道凭证」中完成配置。",
			})
			return
		}
	}
	mc := &model.MerchantChannel{
		MerchantID: merchantID,
		Channel:    ch,
		Status:     req.Status,
	}
	if err := h.merchants.UpsertMerchantChannel(c.Request.Context(), mc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SAVE_FAILED", "msg": "保存渠道授权失败，请稍后重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK"})
}

func (h *Handler) ListMerchantChannels(c *gin.Context) {
	merchantID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	list, err := h.merchants.ListChannels(c.Request.Context(), merchantID)
	if err != nil {
		httputil.Fail500(c, "LIST_FAILED", "查询失败，请稍后重试", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": list})
}

// ---------- Platform channel credentials ----------
// These are the system-level credentials shared by all merchants.

// channelKeepSentinel is what the admin UI sends back for sensitive fields
// that the user did not re-enter during an edit. The upsert handler swaps
// these for the currently-stored value before re-encrypting so secrets never
// have to leave the server.
const channelKeepSentinel = "__KEEP__"

// channelSecretFields lists the config keys to mask on read and merge on write.
var channelSecretFields = map[model.Channel][]string{
	model.ChannelWechat: {"api_v3_key", "private_key_pem", "public_key_pem"},
	model.ChannelAlipay: {"private_key", "alipay_public_key"},
}

var channelRequiredFields = map[model.Channel][]string{
	model.ChannelWechat: {"mch_id", "app_id", "api_v3_key", "serial_no", "private_key_pem", "public_key_id", "public_key_pem"},
	model.ChannelAlipay: {"app_id", "private_key", "alipay_public_key"},
}

type upsertPlatformChannelReq struct {
	Config json.RawMessage `json:"config" binding:"required"`
	// Pointer so an absent JSON field defaults to 1 while an explicit "status":0
	// actually disables the platform channel (which then cascades to disable
	// every merchant_channels row referencing it).
	Status *int16 `json:"status"`
}

func (h *Handler) UpsertPlatformChannel(c *gin.Context) {
	ch := model.Channel(c.Param("channel"))
	if _, ok := channelSecretFields[ch]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_CHANNEL"})
		return
	}
	var req upsertPlatformChannelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	merged, err := h.mergePlatformSecrets(c, ch, req.Config)
	if err != nil {
		httputil.Fail500(c, "MERGE_FAILED", "配置合并失败，请稍后重试", err)
		return
	}
	enc, err := h.cipher.Encrypt(merged)
	if err != nil {
		httputil.Fail500(c, "ENCRYPT_FAILED", "配置加密失败，请稍后重试", err)
		return
	}
	status := int16(1)
	if req.Status != nil {
		status = *req.Status
	}
	pc := &model.PlatformChannel{
		Channel: ch,
		Config:  enc,
		Status:  status,
	}
	if err := h.platformChs.Upsert(c.Request.Context(), pc); err != nil {
		httputil.Fail500(c, "SAVE_FAILED", "保存失败，请稍后重试", err)
		return
	}
	h.registry.Invalidate(ch)

	// If the resulting platform state can't actually serve traffic (disabled or
	// missing required fields), cascade-disable every merchant authorisation
	// for this channel. Otherwise the merchant UI keeps showing "已启用" for a
	// route that runtime-resolves to "platform not configured". Re-enabling
	// the platform later does NOT auto-restore — admins reopen per merchant.
	cascaded := int64(0)
	if usable, err := h.platformChannelConfigured(pc); err == nil && !usable {
		n, derr := h.merchants.DisableChannelForAll(c.Request.Context(), ch)
		if derr != nil {
			// Save already committed; surface the cascade failure but don't
			// roll back the platform change — admin can retry by re-saving.
			httputil.Fail500(c, "CASCADE_FAILED", "平台凭证已保存，但同步停用商户授权失败，请重试", derr)
			return
		}
		cascaded = n
	}
	h.log.Info("platform channel updated",
		zap.String("channel", string(ch)),
		zap.Int16("status", status),
		zap.Int64("merchant_channels_disabled", cascaded))
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"merchant_channels_disabled": cascaded,
	}})
}

func (h *Handler) GetPlatformChannel(c *gin.Context) {
	ch := model.Channel(c.Param("channel"))
	if _, ok := channelSecretFields[ch]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_CHANNEL"})
		return
	}
	pc, err := h.platformChs.Get(c.Request.Context(), ch)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": "OK", "data": nil})
		return
	}
	plain, err := h.cipher.Decrypt(pc.Config)
	if err != nil {
		httputil.Fail500(c, "DECRYPT_FAILED", "配置解密失败，请稍后重试", err)
		return
	}
	var cfg map[string]any
	if err := json.Unmarshal(plain, &cfg); err != nil {
		httputil.Fail500(c, "BAD_CONFIG", "配置格式错误，请稍后重试", err)
		return
	}
	for _, k := range channelSecretFields[ch] {
		if v, ok := cfg[k]; ok && v != nil && v != "" {
			cfg[k] = channelKeepSentinel
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"channel":    pc.Channel,
		"status":     pc.Status,
		"updated_at": pc.UpdatedAt,
		"config":     cfg,
	}})
}

func (h *Handler) ListPlatformChannels(c *gin.Context) {
	list, err := h.platformChs.List(c.Request.Context())
	if err != nil {
		httputil.Fail500(c, "LIST_FAILED", "查询失败，请稍后重试", err)
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, pc := range list {
		configured, err := h.platformChannelConfigured(pc)
		if err != nil {
			httputil.Fail500(c, "BAD_CONFIG", "配置格式错误，请稍后重试", err)
			return
		}
		out = append(out, gin.H{
			"id":         pc.ID,
			"channel":    pc.Channel,
			"status":     pc.Status,
			"configured": configured,
			"created_at": pc.CreatedAt,
			"updated_at": pc.UpdatedAt,
		})
	}
	// Config is intentionally omitted — use GetPlatformChannel for the edit view.
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": out})
}

func (h *Handler) platformChannelConfigured(pc *model.PlatformChannel) (bool, error) {
	if pc.Status != 1 {
		return false, nil
	}
	plain, err := h.cipher.Decrypt(pc.Config)
	if err != nil {
		return false, err
	}
	var cfg map[string]any
	if err := json.Unmarshal(plain, &cfg); err != nil {
		return false, err
	}
	for _, field := range channelRequiredFields[pc.Channel] {
		v, ok := cfg[field]
		if !ok || strings.TrimSpace(fmt.Sprint(v)) == "" {
			return false, nil
		}
	}
	return true, nil
}

// mergePlatformSecrets replaces __KEEP__ sentinels in incoming config with the
// values currently stored in platform_channels, so secrets don't need to be
// re-submitted on every edit.
func (h *Handler) mergePlatformSecrets(c *gin.Context, ch model.Channel, incoming json.RawMessage) (json.RawMessage, error) {
	var cfg map[string]any
	if err := json.Unmarshal(incoming, &cfg); err != nil {
		return nil, err
	}
	needsMerge := false
	for _, k := range channelSecretFields[ch] {
		if v, ok := cfg[k]; ok && v == channelKeepSentinel {
			needsMerge = true
			break
		}
	}
	if !needsMerge {
		return incoming, nil
	}
	existing, err := h.platformChs.Get(c.Request.Context(), ch)
	if err != nil {
		return nil, fmt.Errorf("cannot keep existing secrets: no prior config for channel %s", ch)
	}
	plain, err := h.cipher.Decrypt(existing.Config)
	if err != nil {
		return nil, err
	}
	var prev map[string]any
	if err := json.Unmarshal(plain, &prev); err != nil {
		return nil, err
	}
	for _, k := range channelSecretFields[ch] {
		if cfg[k] == channelKeepSentinel {
			cfg[k] = prev[k]
		}
	}
	return json.Marshal(cfg)
}

// ---------- Orders ----------

func (h *Handler) ListOrders(c *gin.Context) {
	page, size := httputil.ParsePage(c)
	filter := repository.OrderFilter{
		Status:  model.OrderStatus(c.Query("status")),
		Channel: model.Channel(c.Query("channel")),
		Offset:  (page - 1) * size,
		Limit:   size,
	}
	if v := c.Query("merchant_id"); v != "" {
		filter.MerchantID, _ = strconv.ParseInt(v, 10, 64)
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.To = &t
		}
	}
	list, total, err := h.orders.List(c.Request.Context(), filter)
	if err != nil {
		httputil.Fail500(c, "LIST_FAILED", "查询失败，请稍后重试", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"list": list, "total": total, "page": page, "size": size,
	}})
}

// ---------- WeChat cert parsing helper ----------

type parseCertReq struct {
	PEM string `json:"pem" binding:"required"`
}

func (h *Handler) ParseWechatCert(c *gin.Context) {
	var req parseCertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	block, _ := pem.Decode([]byte(req.PEM))
	if block == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_PEM", "msg": "not a valid PEM block"})
		return
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_CERT", "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"serial_no":  strings.ToUpper(hex.EncodeToString(cert.SerialNumber.Bytes())),
		"not_before": cert.NotBefore.Format(time.RFC3339),
		"not_after":  cert.NotAfter.Format(time.RFC3339),
		"subject":    cert.Subject.CommonName,
	}})
}

// ---------- Test order ----------

type testCreateOrderReq struct {
	MerchantID      int64           `json:"merchant_id" binding:"required"`
	Channel         model.Channel   `json:"channel" binding:"required,oneof=wechat alipay"`
	TradeType       model.TradeType `json:"trade_type" binding:"required,oneof=native h5"`
	Subject         string          `json:"subject" binding:"required"`
	Amount          int64           `json:"amount" binding:"required,min=1"`
	MerchantOrderNo string          `json:"merchant_order_no"`
	ExpireSeconds   int             `json:"expire_seconds"`
}

func (h *Handler) TestCreateOrder(c *gin.Context) {
	var req testCreateOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	if req.MerchantOrderNo == "" {
		req.MerchantOrderNo = idgen.OrderNo("ADMIN")
	}
	if req.ExpireSeconds == 0 {
		req.ExpireSeconds = 900
	}
	res, err := h.paymentSvc.CreateOrder(c.Request.Context(), payment.CreateOrderInput{
		MerchantID:      req.MerchantID,
		MerchantOrderNo: req.MerchantOrderNo,
		Channel:         req.Channel,
		TradeType:       req.TradeType,
		Subject:         req.Subject,
		Amount:          req.Amount,
		ClientIP:        c.ClientIP(),
		ExpireSeconds:   req.ExpireSeconds,
	})
	if err != nil {
		httputil.Fail500(c, "CREATE_FAILED", "操作失败，请稍后重试", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"order_no":          res.OrderNo,
		"merchant_order_no": req.MerchantOrderNo,
		"code_url":          res.CodeURL,
		"h5_url":            res.H5URL,
	}})
}

// ---------- Refund ----------

type adminRefundReq struct {
	Amount int64  `json:"amount" binding:"required,min=1"`
	Reason string `json:"reason"`
}

func defaultReason(r string) string {
	if r == "" {
		return "商家退款"
	}
	return r
}

func (h *Handler) RefundOrder(c *gin.Context) {
	orderNo := c.Param("order_no")
	o, err := h.orders.GetByOrderNo(c.Request.Context(), orderNo)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "msg": "订单不存在"})
		return
	}
	var req adminRefundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	h.log.Info("admin refund request",
		zap.String("order_no", orderNo),
		zap.Int64("merchant_id", o.MerchantID),
		zap.Int64("amount", req.Amount))
	ro, err := h.paymentSvc.Refund(c.Request.Context(), payment.RefundInput{
		MerchantID:       o.MerchantID,
		MerchantOrderNo:  o.MerchantOrderNo,
		MerchantRefundNo: idgen.OrderNo("ARF"),
		Amount:           req.Amount,
		Reason:           defaultReason(req.Reason),
	})
	if err != nil {
		h.log.Error("admin refund failed",
			zap.String("order_no", orderNo),
			zap.Error(err))
		code, status, msg := http.StatusInternalServerError, "REFUND_FAILED", "退款失败，请稍后重试"
		switch {
		case errors.Is(err, payment.ErrOrderNotFound):
			code, msg = http.StatusNotFound, "订单不存在"
		case errors.Is(err, payment.ErrInvalidStatus):
			code, status, msg = http.StatusBadRequest, "INVALID_STATUS", "当前订单状态不支持退款"
		case errors.Is(err, payment.ErrRefundExceedAmount):
			code, status, msg = http.StatusBadRequest, "EXCEED_AMOUNT", "退款金额超过可退金额"
		}
		c.JSON(code, gin.H{"code": status, "msg": msg})
		return
	}
	h.log.Info("admin refund submitted",
		zap.String("order_no", orderNo),
		zap.String("refund_no", ro.RefundNo),
		zap.String("status", string(ro.Status)))
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": ro})
}

// ---------- Notify logs ----------

func (h *Handler) ListNotifyLogs(c *gin.Context) {
	page, size := httputil.ParsePage(c)
	filter := repository.NotifyLogFilter{
		OrderNo: strings.TrimSpace(c.Query("order_no")),
		Status:  model.NotifyStatus(c.Query("status")),
		Offset:  (page - 1) * size,
		Limit:   size,
	}
	list, total, err := h.logs.List(c.Request.Context(), filter)
	if err != nil {
		httputil.Fail500(c, "LIST_FAILED", "查询失败，请稍后重试", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"list": list, "total": total, "page": page, "size": size,
	}})
}

func (h *Handler) RetryNotify(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	n, err := h.logs.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	}
	now := time.Now()
	n.Status = model.NotifyPending
	n.NextRetryAt = &now
	// Reset the attempt counter + error so the full backoff schedule runs
	// again. Otherwise a dropped log (retry_count already at max) would only
	// get one more shot before being re-dropped.
	n.RetryCount = 0
	n.LastError = ""
	n.HTTPStatus = 0
	n.ResponseBody = ""
	if err := h.logs.Update(c.Request.Context(), n); err != nil {
		httputil.Fail500(c, "UPDATE_FAILED", "更新失败，请稍后重试", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK"})
}

// ---------- Settlements ----------

func (h *Handler) MerchantBalance(c *gin.Context) {
	merchantID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	bal, err := h.balances.GetBalance(c.Request.Context(), merchantID)
	if err != nil {
		httputil.Fail500(c, "BALANCE_FAILED", "查询余额失败", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": bal})
}

func (h *Handler) MerchantPeriodBalance(c *gin.Context) {
	merchantID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	start, err1 := time.Parse("2006-01-02", c.Query("start"))
	end, err2 := time.Parse("2006-01-02", c.Query("end"))
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": "start/end required (YYYY-MM-DD)"})
		return
	}
	end = end.AddDate(0, 0, 1)
	pb, err := h.balances.GetPeriodBalance(c.Request.Context(), merchantID, start, end)
	if err != nil {
		httputil.Fail500(c, "BALANCE_FAILED", "查询失败", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": pb})
}

func (h *Handler) ListSettlements(c *gin.Context) {
	page, size := httputil.ParsePage(c)
	filter := repository.SettlementFilter{
		Offset: (page - 1) * size,
		Limit:  size,
	}
	if v := c.Query("merchant_id"); v != "" {
		filter.MerchantID, _ = strconv.ParseInt(v, 10, 64)
	}
	if v := c.Query("status"); v != "" {
		filter.Status = model.SettlementStatus(v)
	}
	list, total, err := h.settlements.List(c.Request.Context(), filter)
	if err != nil {
		httputil.Fail500(c, "LIST_FAILED", "查询失败", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"list": list, "total": total, "page": page, "size": size,
	}})
}

func (h *Handler) ListMerchantBalances(c *gin.Context) {
	list, _, err := h.merchants.List(c.Request.Context(), repository.MerchantFilter{Offset: 0, Limit: 500})
	if err != nil {
		httputil.Fail500(c, "LIST_FAILED", "查询失败", err)
		return
	}
	type row struct {
		MerchantID   int64  `json:"merchant_id"`
		Name         string `json:"name"`
		MchNo        string `json:"mch_no"`
		TotalIncome  int64  `json:"total_income"`
		TotalRefund  int64  `json:"total_refund"`
		TotalSettled int64  `json:"total_settled"`
		Available    int64  `json:"available"`
		PeriodStart  string `json:"period_start"`
	}
	var rows []row
	for _, m := range list {
		bal, err := h.balances.GetBalance(c.Request.Context(), m.ID)
		if err != nil {
			continue
		}
		ps := m.CreatedAt.Format("2006-01-02")
		if last, err := h.settlements.LastPaidEndTime(c.Request.Context(), m.ID); err == nil && last != nil {
			ps = last.Format("2006-01-02")
		}
		rows = append(rows, row{
			MerchantID:   m.ID,
			Name:         m.Name,
			MchNo:        m.MchNo,
			TotalIncome:  bal.TotalIncome,
			TotalRefund:  bal.TotalRefund,
			TotalSettled: bal.TotalSettled,
			Available:    bal.Available,
			PeriodStart:  ps,
		})
	}
	if rows == nil {
		rows = []row{}
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": rows})
}

type createSettlementReq struct {
	MerchantID  int64   `json:"merchant_id" binding:"required"`
	FeeRate     float64 `json:"fee_rate"`
	PeriodStart string  `json:"period_start"`
	PeriodEnd   string  `json:"period_end"`
	Remark      string  `json:"remark"`
}

func (h *Handler) CreateSettlement(c *gin.Context) {
	var req createSettlementReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	if _, err := h.merchants.GetByID(c.Request.Context(), req.MerchantID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "msg": "商户不存在"})
		return
	}
	now := time.Now()
	periodStart := now
	periodEnd := now
	if req.PeriodStart != "" {
		if t, err := time.Parse("2006-01-02", req.PeriodStart); err == nil {
			periodStart = t
		}
	}
	if req.PeriodEnd != "" {
		if t, err := time.Parse("2006-01-02", req.PeriodEnd); err == nil {
			periodEnd = t
		}
	}
	pb, err := h.balances.GetPeriodBalance(c.Request.Context(), req.MerchantID, periodStart, periodEnd.AddDate(0, 0, 1))
	if err != nil {
		httputil.Fail500(c, "BALANCE_FAILED", "查询余额失败", err)
		return
	}
	if pb.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INSUFFICIENT", "msg": "该周期内无可结算余额"})
		return
	}
	amount := pb.Amount
	fee := int64(float64(amount) * req.FeeRate)
	netAmount := amount - fee
	s := &model.Settlement{
		SettlementNo: idgen.OrderNo("ST"),
		MerchantID:   req.MerchantID,
		Amount:       amount,
		Fee:          fee,
		NetAmount:    netAmount,
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		Status:       model.SettlementPending,
		Remark:       req.Remark,
	}
	if err := h.settlements.Create(c.Request.Context(), s); err != nil {
		httputil.Fail500(c, "CREATE_FAILED", "创建结算单失败", err)
		return
	}
	h.log.Info("settlement created",
		zap.String("settlement_no", s.SettlementNo),
		zap.Int64("merchant_id", req.MerchantID),
		zap.Int64("amount", amount),
		zap.Int64("fee", fee),
		zap.Int64("net_amount", netAmount))
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": s})
}

func (h *Handler) MarkSettlementPaid(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	s, err := h.settlements.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	}
	if s.Status != model.SettlementPending {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_STATUS", "msg": "只有待结算状态可标记已打款"})
		return
	}
	now := time.Now()
	s.Status = model.SettlementPaid
	s.PaidAt = &now
	if err := h.settlements.Update(c.Request.Context(), s); err != nil {
		httputil.Fail500(c, "UPDATE_FAILED", "更新失败", err)
		return
	}
	h.log.Info("settlement marked paid",
		zap.String("settlement_no", s.SettlementNo),
		zap.Int64("merchant_id", s.MerchantID))
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": s})
}

func (h *Handler) CancelSettlement(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	s, err := h.settlements.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	}
	if s.Status != model.SettlementPending {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_STATUS", "msg": "只有待结算状态可取消"})
		return
	}
	s.Status = model.SettlementCancelled
	if err := h.settlements.Update(c.Request.Context(), s); err != nil {
		httputil.Fail500(c, "UPDATE_FAILED", "更新失败", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK"})
}

// ---------- helpers ----------

func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(h), err
}

// ---------- System settings ----------

func (h *Handler) GetSettings(c *gin.Context) {
	list, err := h.settings.GetAll(c.Request.Context())
	if err != nil {
		httputil.Fail500(c, "LIST_FAILED", "查询失败，请稍后重试", err)
		return
	}
	m := make(map[string]string, len(list))
	for _, s := range list {
		m[s.Key] = s.Value
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": m})
}

type updateSettingsReq struct {
	PlatformBase string `json:"platform_base"`
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	var req updateSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	val := strings.TrimRight(strings.TrimSpace(req.PlatformBase), "/")
	if err := h.settings.Set(c.Request.Context(), "platform_base", val); err != nil {
		httputil.Fail500(c, "SAVE_FAILED", "保存失败，请稍后重试", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK"})
}

// ---------- helpers ----------

func channelDisplayName(ch model.Channel) string {
	switch ch {
	case model.ChannelWechat:
		return "微信支付"
	case model.ChannelAlipay:
		return "支付宝"
	default:
		return string(ch)
	}
}
