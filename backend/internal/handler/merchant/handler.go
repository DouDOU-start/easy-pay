package merchant

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/easypay/easy-pay/backend/internal/handler/auth"
	"github.com/easypay/easy-pay/backend/internal/handler/httputil"
	"github.com/easypay/easy-pay/backend/internal/model"
	"github.com/easypay/easy-pay/backend/internal/repository"
)

type Handler struct {
	merchants   repository.MerchantRepo
	orders      repository.OrderRepo
	logs        repository.NotifyLogRepo
	settlements repository.SettlementRepo
	balances    repository.BalanceRepo
	dashboard   repository.DashboardRepo
}

func New(
	merchants repository.MerchantRepo,
	orders repository.OrderRepo,
	logs repository.NotifyLogRepo,
	settlements repository.SettlementRepo,
	balances repository.BalanceRepo,
	dashboard repository.DashboardRepo,
) *Handler {
	return &Handler{merchants: merchants, orders: orders, logs: logs, settlements: settlements, balances: balances, dashboard: dashboard}
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

func (h *Handler) ResetSecret(c *gin.Context) {
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
	m.AppSecret = uuid.NewString() + uuid.NewString()
	if err := h.merchants.Update(c.Request.Context(), m); err != nil {
		httputil.Fail500(c, "RESET_FAILED", "重置失败", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"app_id":     m.AppID,
		"app_secret": m.AppSecret,
	}})
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

// ---------- dashboard ----------

func (h *Handler) Dashboard(c *gin.Context) {
	id, err := auth.CurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED"})
		return
	}
	ctx := c.Request.Context()
	todayCount, _ := h.dashboard.TodayOrderCount(ctx, id)
	todayPaid, _ := h.dashboard.TodayPaidAmount(ctx, id)
	todayRefund, _ := h.dashboard.TodayRefundAmount(ctx, id)
	totalRevenue, _ := h.dashboard.TotalRevenue(ctx, id)
	totalRefund, _ := h.dashboard.TotalRefund(ctx, id)
	bal, _ := h.balances.GetBalance(ctx, id)
	trend, _ := h.dashboard.Last7DaysPayments(ctx, id)
	recentOrders, _ := h.dashboard.RecentOrders(ctx, id, 10)
	if trend == nil {
		trend = []repository.DailyPayment{}
	}
	if recentOrders == nil {
		recentOrders = []*model.Order{}
	}
	available := int64(0)
	if bal != nil {
		available = bal.Available
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"today": gin.H{
			"order_count":   todayCount,
			"paid_amount":   todayPaid,
			"refund_amount": todayRefund,
		},
		"overall": gin.H{
			"total_revenue": totalRevenue,
			"total_refund":  totalRefund,
			"available":     available,
		},
		"trend":         trend,
		"recent_orders": recentOrders,
	}})
}

// ---------- settlements ----------

func (h *Handler) Balance(c *gin.Context) {
	id, err := auth.CurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED"})
		return
	}
	bal, err := h.balances.GetBalance(c.Request.Context(), id)
	if err != nil {
		httputil.Fail500(c, "BALANCE_FAILED", "查询余额失败", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": bal})
}

func (h *Handler) Settlements(c *gin.Context) {
	id, err := auth.CurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED"})
		return
	}
	page, size := httputil.ParsePage(c)
	filter := repository.SettlementFilter{
		MerchantID: id,
		Offset:     (page - 1) * size,
		Limit:      size,
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
