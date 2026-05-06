package merchant

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/easypay/easy-pay/backend/internal/handler/auth"
	"github.com/easypay/easy-pay/backend/internal/handler/httputil"
	"github.com/easypay/easy-pay/backend/internal/model"
	"github.com/easypay/easy-pay/backend/internal/repository"
)

type Handler struct {
	merchants repository.MerchantRepo
	orders    repository.OrderRepo
	logs      repository.NotifyLogRepo
}

func New(
	merchants repository.MerchantRepo,
	orders repository.OrderRepo,
	logs repository.NotifyLogRepo,
) *Handler {
	return &Handler{merchants: merchants, orders: orders, logs: logs}
}

// ---------- profile ----------

func (h *Handler) Me(c *gin.Context) {
	id, err := auth.CurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED"})
		return
	}
	m, err := h.merchants.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"id":         m.ID,
		"mch_no":     m.MchNo,
		"name":       m.Name,
		"email":      m.Email,
		"role":       m.Role,
		"notify_url": m.NotifyURL,
		"app_id":     m.AppID,
		"status":     m.Status,
		"created_at": m.CreatedAt,
	}})
}

type updateProfileReq struct {
	Name      *string `json:"name"`
	NotifyURL *string `json:"notify_url"`
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	id, err := auth.CurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED"})
		return
	}
	var req updateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	m, err := h.merchants.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED"})
		return
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": "名称不能为空"})
			return
		}
		m.Name = name
	}
	if req.NotifyURL != nil {
		m.NotifyURL = strings.TrimSpace(*req.NotifyURL)
	}
	if err := h.merchants.Update(c.Request.Context(), m); err != nil {
		httputil.Fail500(c, "UPDATE_FAILED", "保存失败，请稍后重试", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"id":         m.ID,
		"name":       m.Name,
		"notify_url": m.NotifyURL,
	}})
}

type changePasswordReq struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

func (h *Handler) ChangePassword(c *gin.Context) {
	id, err := auth.CurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED"})
		return
	}
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	if req.OldPassword == req.NewPassword {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": "新密码不能与旧密码相同"})
		return
	}
	m, err := h.merchants.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(m.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "WRONG_PASSWORD", "msg": "旧密码不正确"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		httputil.Fail500(c, "HASH_FAILED", "操作失败，请稍后重试", err)
		return
	}
	m.PasswordHash = string(hash)
	now := time.Now()
	m.PasswordChangedAt = &now
	if err := h.merchants.Update(c.Request.Context(), m); err != nil {
		httputil.Fail500(c, "UPDATE_FAILED", "操作失败，请稍后重试", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK"})
}

// ---------- bills ----------

func (h *Handler) Orders(c *gin.Context) {
	id, err := auth.CurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED"})
		return
	}
	page, size := httputil.ParsePage(c)
	filter := repository.OrderFilter{
		MerchantID: id,
		Status:     model.OrderStatus(c.Query("status")),
		Channel:    model.Channel(c.Query("channel")),
		Offset:     (page - 1) * size,
		Limit:      size,
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

func (h *Handler) NotifyLogs(c *gin.Context) {
	id, err := auth.CurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED"})
		return
	}
	page, size := httputil.ParsePage(c)
	filter := repository.NotifyLogFilter{
		MerchantID: id,
		OrderNo:    strings.TrimSpace(c.Query("order_no")),
		Status:     model.NotifyStatus(c.Query("status")),
		Offset:     (page - 1) * size,
		Limit:      size,
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

func (h *Handler) OrderDetail(c *gin.Context) {
	id, err := auth.CurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED"})
		return
	}
	o, err := h.orders.GetByOrderNo(c.Request.Context(), c.Param("order_no"))
	if err != nil || o.MerchantID != id {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": o})
}
