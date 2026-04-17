package admin

import (
	cryptorand "crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/easypay/easy-pay/internal/channel/registry"
	"github.com/easypay/easy-pay/internal/model"
	"github.com/easypay/easy-pay/internal/pkg/crypto"
	"github.com/easypay/easy-pay/internal/pkg/idgen"
	"github.com/easypay/easy-pay/internal/repository"
	"github.com/easypay/easy-pay/internal/service/payment"
)

type Handler struct {
	merchants   repository.MerchantRepo
	platformChs repository.PlatformChannelRepo
	orders      repository.OrderRepo
	refunds     repository.RefundRepo
	logs        repository.NotifyLogRepo
	cipher      *crypto.AESGCM
	registry    *registry.Registry
	paymentSvc  *payment.Service
}

func New(
	merchants repository.MerchantRepo,
	platformChs repository.PlatformChannelRepo,
	orders repository.OrderRepo,
	refunds repository.RefundRepo,
	logs repository.NotifyLogRepo,
	cipher *crypto.AESGCM,
	reg *registry.Registry,
	paymentSvc *payment.Service,
) *Handler {
	return &Handler{
		merchants:   merchants,
		platformChs: platformChs,
		orders:      orders,
		refunds:     refunds,
		logs:        logs,
		cipher:      cipher,
		registry:    reg,
		paymentSvc:  paymentSvc,
	}
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
		fail500(c, "CREATE_FAILED", "操作失败，请稍后重试", err)
		return
	}
	password, err := randomPassword(12)
	if err != nil {
		fail500(c, "CREATE_FAILED", "操作失败，请稍后重试", err)
		return
	}
	pwHash, err := HashPassword(password)
	if err != nil {
		fail500(c, "CREATE_FAILED", "操作失败，请稍后重试", err)
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
		fail500(c, "CREATE_FAILED", "操作失败，请稍后重试", err)
		return
	}
	// Return app_secret and the generated plaintext password only on creation.
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
	page, size := parsePage(c)
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
		fail500(c, "LIST_FAILED", "查询失败，请稍后重试", err)
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
		fail500(c, "UPDATE_FAILED", "更新失败，请稍后重试", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": m})
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
		fail500(c, "RESET_FAILED", "操作失败，请稍后重试", err)
		return
	}
	pwHash, err := HashPassword(password)
	if err != nil {
		fail500(c, "RESET_FAILED", "操作失败，请稍后重试", err)
		return
	}
	now := time.Now()
	m.PasswordHash = pwHash
	m.PasswordChangedAt = &now
	if err := h.merchants.Update(c.Request.Context(), m); err != nil {
		fail500(c, "RESET_FAILED", "操作失败，请稍后重试", err)
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
		fail500(c, "LIST_FAILED", "查询失败，请稍后重试", err)
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

type upsertPlatformChannelReq struct {
	Config json.RawMessage `json:"config" binding:"required"`
	Status int16           `json:"status"`
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
		fail500(c, "MERGE_FAILED", "配置合并失败，请稍后重试", err)
		return
	}
	enc, err := h.cipher.Encrypt(merged)
	if err != nil {
		fail500(c, "ENCRYPT_FAILED", "配置加密失败，请稍后重试", err)
		return
	}
	status := int16(1)
	if req.Status != 0 {
		status = req.Status
	}
	pc := &model.PlatformChannel{
		Channel: ch,
		Config:  enc,
		Status:  status,
	}
	if err := h.platformChs.Upsert(c.Request.Context(), pc); err != nil {
		fail500(c, "SAVE_FAILED", "保存失败，请稍后重试", err)
		return
	}
	h.registry.Invalidate(ch)
	c.JSON(http.StatusOK, gin.H{"code": "OK"})
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
		fail500(c, "DECRYPT_FAILED", "配置解密失败，请稍后重试", err)
		return
	}
	var cfg map[string]any
	if err := json.Unmarshal(plain, &cfg); err != nil {
		fail500(c, "BAD_CONFIG", "配置格式错误，请稍后重试", err)
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
		fail500(c, "LIST_FAILED", "查询失败，请稍后重试", err)
		return
	}
	// Config is intentionally omitted — use GetPlatformChannel for the edit view.
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": list})
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
	page, size := parsePage(c)
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
		fail500(c, "LIST_FAILED", "查询失败，请稍后重试", err)
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
		fail500(c, "CREATE_FAILED", "操作失败，请稍后重试", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"order_no":          res.OrderNo,
		"merchant_order_no": req.MerchantOrderNo,
		"code_url":          res.CodeURL,
		"h5_url":            res.H5URL,
	}})
}

// ---------- Notify logs ----------

func (h *Handler) ListNotifyLogs(c *gin.Context) {
	page, size := parsePage(c)
	filter := repository.NotifyLogFilter{
		OrderNo: strings.TrimSpace(c.Query("order_no")),
		Status:  model.NotifyStatus(c.Query("status")),
		Offset:  (page - 1) * size,
		Limit:   size,
	}
	list, total, err := h.logs.List(c.Request.Context(), filter)
	if err != nil {
		fail500(c, "LIST_FAILED", "查询失败，请稍后重试", err)
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
		fail500(c, "UPDATE_FAILED", "更新失败，请稍后重试", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK"})
}

// ---------- helpers ----------

func parsePage(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 20
	}
	return page, size
}

// HashPassword is exported so the bootstrap command can seed an admin user.
func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(h), err
}

// fail500 logs the real error server-side and returns a generic message to the
// client so internal details (SQL errors, stack traces, etc.) never leak.
func fail500(c *gin.Context, code string, msg string, err error) {
	log.Printf("[admin] %s: %v", code, err)
	c.JSON(http.StatusInternalServerError, gin.H{"code": code, "msg": msg})
}
