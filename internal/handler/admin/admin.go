package admin

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
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
	merchants  repository.MerchantRepo
	orders     repository.OrderRepo
	refunds    repository.RefundRepo
	logs       repository.NotifyLogRepo
	cipher     *crypto.AESGCM
	registry   *registry.Registry
	paymentSvc *payment.Service
}

func New(
	merchants repository.MerchantRepo,
	orders repository.OrderRepo,
	refunds repository.RefundRepo,
	logs repository.NotifyLogRepo,
	cipher *crypto.AESGCM,
	reg *registry.Registry,
	paymentSvc *payment.Service,
) *Handler {
	return &Handler{
		merchants: merchants, orders: orders, refunds: refunds,
		logs: logs, cipher: cipher, registry: reg,
		paymentSvc: paymentSvc,
	}
}

// ---------- Merchants ----------

type createMerchantReq struct {
	MchNo     string `json:"mch_no" binding:"required"`
	Name      string `json:"name" binding:"required"`
	NotifyURL string `json:"notify_url"`
	Remark    string `json:"remark"`
}

func (h *Handler) CreateMerchant(c *gin.Context) {
	var req createMerchantReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	m := &model.Merchant{
		MchNo:     req.MchNo,
		Name:      req.Name,
		AppID:     "ap_" + uuid.NewString()[:12],
		AppSecret: uuid.NewString() + uuid.NewString(),
		NotifyURL: req.NotifyURL,
		Status:    1,
		Remark:    req.Remark,
	}
	if err := h.merchants.Create(c.Request.Context(), m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "CREATE_FAILED", "msg": err.Error()})
		return
	}
	// return app_secret only on creation; downstream must store it.
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"id":         m.ID,
		"mch_no":     m.MchNo,
		"app_id":     m.AppID,
		"app_secret": m.AppSecret,
		"name":       m.Name,
	}})
}

func (h *Handler) ListMerchants(c *gin.Context) {
	page, size := parsePage(c)
	list, total, err := h.merchants.List(c.Request.Context(), (page-1)*size, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "LIST_FAILED", "msg": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "UPDATE_FAILED", "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": m})
}

// ---------- Merchant channels ----------

type upsertChannelReq struct {
	Channel model.Channel   `json:"channel" binding:"required,oneof=wechat alipay"`
	Config  json.RawMessage `json:"config" binding:"required"`
	Status  int16           `json:"status"`
}

// channelKeepSentinel is what the admin UI sends back for sensitive fields
// (api_v3_key, private keys, ...) that the user did not re-enter during an
// edit. The upsert handler swaps these for the currently-stored value before
// re-encrypting, so secrets never have to leave the server.
const channelKeepSentinel = "__KEEP__"

// channelSecretFields lists, per channel, the config keys whose values are
// masked when read back by the admin UI and must be merged from the existing
// row when a sentinel comes in on save.
var channelSecretFields = map[model.Channel][]string{
	model.ChannelWechat: {"api_v3_key", "private_key_pem", "public_key_pem"},
	model.ChannelAlipay: {"private_key", "alipay_public_key"},
}

func (h *Handler) UpsertMerchantChannel(c *gin.Context) {
	merchantID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req upsertChannelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	merged, err := h.mergeKeptSecrets(c, merchantID, req.Channel, req.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "MERGE_FAILED", "msg": err.Error()})
		return
	}
	enc, err := h.cipher.Encrypt(merged)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ENCRYPT_FAILED", "msg": err.Error()})
		return
	}
	mc := &model.MerchantChannel{
		MerchantID: merchantID,
		Channel:    req.Channel,
		Config:     enc,
		Status:     1,
	}
	if req.Status != 0 {
		mc.Status = req.Status
	}
	if err := h.merchants.UpsertChannelConfig(c.Request.Context(), mc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SAVE_FAILED", "msg": err.Error()})
		return
	}
	h.registry.Invalidate(merchantID, req.Channel)
	c.JSON(http.StatusOK, gin.H{"code": "OK"})
}

func (h *Handler) ListMerchantChannels(c *gin.Context) {
	merchantID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	list, err := h.merchants.ListChannels(c.Request.Context(), merchantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "LIST_FAILED", "msg": err.Error()})
		return
	}
	// Config is intentionally omitted from the JSON — secrets stay out of
	// the admin list response. Fetch a single channel explicitly to decrypt.
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": list})
}

// GetMerchantChannel returns a single channel's config for the edit drawer.
// Non-sensitive fields (mch_id, app_id, serial_no, ...) are returned as-is;
// secret fields are replaced with the KEEP sentinel so the form can show
// "already configured" without shipping the plaintext over the wire.
func (h *Handler) GetMerchantChannel(c *gin.Context) {
	merchantID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	channel := model.Channel(c.Param("channel"))
	if _, ok := channelSecretFields[channel]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_CHANNEL"})
		return
	}
	list, err := h.merchants.ListChannels(c.Request.Context(), merchantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "LOAD_FAILED", "msg": err.Error()})
		return
	}
	var found *model.MerchantChannel
	for _, mc := range list {
		if mc.Channel == channel {
			found = mc
			break
		}
	}
	if found == nil {
		c.JSON(http.StatusOK, gin.H{"code": "OK", "data": nil})
		return
	}
	plain, err := h.cipher.Decrypt(found.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DECRYPT_FAILED", "msg": err.Error()})
		return
	}
	var cfg map[string]any
	if err := json.Unmarshal(plain, &cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "BAD_CONFIG", "msg": err.Error()})
		return
	}
	for _, k := range channelSecretFields[channel] {
		if v, ok := cfg[k]; ok && v != nil && v != "" {
			cfg[k] = channelKeepSentinel
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"channel":    found.Channel,
		"status":     found.Status,
		"updated_at": found.UpdatedAt,
		"config":     cfg,
	}})
}

// mergeKeptSecrets looks for KEEP-sentinel values in the incoming config and
// swaps them for the currently-stored plaintext. If there is no existing row,
// or none of the incoming values is a sentinel, the input is returned
// unchanged.
func (h *Handler) mergeKeptSecrets(c *gin.Context, merchantID int64, channel model.Channel, incoming json.RawMessage) (json.RawMessage, error) {
	var cfg map[string]any
	if err := json.Unmarshal(incoming, &cfg); err != nil {
		return nil, err
	}
	needsMerge := false
	for _, k := range channelSecretFields[channel] {
		if v, ok := cfg[k]; ok && v == channelKeepSentinel {
			needsMerge = true
			break
		}
	}
	if !needsMerge {
		return incoming, nil
	}
	list, err := h.merchants.ListChannels(c.Request.Context(), merchantID)
	if err != nil {
		return nil, err
	}
	var existing *model.MerchantChannel
	for _, mc := range list {
		if mc.Channel == channel {
			existing = mc
			break
		}
	}
	if existing == nil {
		return nil, fmt.Errorf("cannot keep existing secrets: no prior config for channel %s", channel)
	}
	plain, err := h.cipher.Decrypt(existing.Config)
	if err != nil {
		return nil, err
	}
	var prev map[string]any
	if err := json.Unmarshal(plain, &prev); err != nil {
		return nil, err
	}
	for _, k := range channelSecretFields[channel] {
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "LIST_FAILED", "msg": err.Error()})
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

// ParseWechatCert extracts the serial number out of an apiclient_cert.pem
// contents. The browser has no built-in X.509 parser so the batch-import UI
// round-trips the certificate through this endpoint to fill in serial_no
// automatically.
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

// ---------- Test order (admin-authored, bypasses merchant HMAC) ----------

type testCreateOrderReq struct {
	MerchantID      int64           `json:"merchant_id" binding:"required"`
	Channel         model.Channel   `json:"channel" binding:"required,oneof=wechat alipay"`
	TradeType       model.TradeType `json:"trade_type" binding:"required,oneof=native h5"`
	Subject         string          `json:"subject" binding:"required"`
	Amount          int64           `json:"amount" binding:"required,min=1"`
	MerchantOrderNo string          `json:"merchant_order_no"`
	ExpireSeconds   int             `json:"expire_seconds"`
}

// TestCreateOrder is an admin-only convenience that calls the payment service
// directly, skipping the downstream HMAC handshake. Only mounted under the
// authenticated /admin router, so it cannot be abused by anonymous callers.
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "CREATE_FAILED", "msg": err.Error()})
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
	orderNo := c.Query("order_no")
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": "order_no required"})
		return
	}
	list, err := h.logs.ListByOrder(c.Request.Context(), orderNo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "LIST_FAILED", "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": list})
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
	if err := h.logs.Update(c.Request.Context(), n); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "UPDATE_FAILED", "msg": err.Error()})
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
