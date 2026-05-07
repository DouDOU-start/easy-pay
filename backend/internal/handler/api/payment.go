package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/easypay/easy-pay/backend/internal/handler/middleware"
	"github.com/easypay/easy-pay/backend/internal/model"
	"github.com/easypay/easy-pay/backend/internal/service/payment"
)

type PaymentHandler struct {
	svc *payment.Service
	log *zap.Logger
}

func NewPaymentHandler(svc *payment.Service, log *zap.Logger) *PaymentHandler {
	return &PaymentHandler{svc: svc, log: log}
}

type createOrderReq struct {
	MerchantOrderNo string         `json:"merchant_order_no" binding:"required,max=64"`
	Channel         model.Channel  `json:"channel" binding:"required,oneof=wechat alipay"`
	TradeType       model.TradeType `json:"trade_type" binding:"required,oneof=native h5"`
	Subject         string         `json:"subject" binding:"required,max=256"`
	Amount          int64          `json:"amount" binding:"required,min=1"`
	Currency        string         `json:"currency"`
	ExpireSeconds   int            `json:"expire_seconds"`
	Extra           map[string]any `json:"extra"`
}

func (h *PaymentHandler) Create(c *gin.Context) {
	var req createOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	m := middleware.GetMerchant(c)
	h.log.Info("create order",
		zap.Int64("merchant_id", m.ID),
		zap.String("merchant_order_no", req.MerchantOrderNo),
		zap.String("channel", string(req.Channel)),
		zap.String("trade_type", string(req.TradeType)),
		zap.Int64("amount", req.Amount))
	res, err := h.svc.CreateOrder(c.Request.Context(), payment.CreateOrderInput{
		MerchantID:      m.ID,
		MerchantOrderNo: req.MerchantOrderNo,
		Channel:         req.Channel,
		TradeType:       req.TradeType,
		Subject:         req.Subject,
		Amount:          req.Amount,
		Currency:        req.Currency,
		ClientIP:        c.ClientIP(),
		Extra:           req.Extra,
		ExpireSeconds:   req.ExpireSeconds,
	})
	if err != nil {
		h.log.Error("create order failed",
			zap.Int64("merchant_id", m.ID),
			zap.String("merchant_order_no", req.MerchantOrderNo),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "CREATE_FAILED", "msg": err.Error()})
		return
	}
	h.log.Info("order created",
		zap.Int64("merchant_id", m.ID),
		zap.String("order_no", res.OrderNo))
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": res})
}

func (h *PaymentHandler) Query(c *gin.Context) {
	merchantOrderNo := c.Query("merchant_order_no")
	if merchantOrderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": "merchant_order_no required"})
		return
	}
	m := middleware.GetMerchant(c)
	o, err := h.svc.QueryOrder(c.Request.Context(), m.ID, merchantOrderNo)
	if err != nil {
		if errors.Is(err, payment.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "msg": err.Error()})
			return
		}
		h.log.Error("query order failed",
			zap.Int64("merchant_id", m.ID),
			zap.String("merchant_order_no", merchantOrderNo),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "QUERY_FAILED", "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": o})
}

type closeOrderReq struct {
	MerchantOrderNo string `json:"merchant_order_no" binding:"required"`
}

func (h *PaymentHandler) Close(c *gin.Context) {
	var req closeOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	m := middleware.GetMerchant(c)
	h.log.Info("close order",
		zap.Int64("merchant_id", m.ID),
		zap.String("merchant_order_no", req.MerchantOrderNo))
	if err := h.svc.CloseOrder(c.Request.Context(), m.ID, req.MerchantOrderNo); err != nil {
		h.log.Error("close order failed",
			zap.Int64("merchant_id", m.ID),
			zap.String("merchant_order_no", req.MerchantOrderNo),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "CLOSE_FAILED", "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK"})
}

type refundReq struct {
	MerchantOrderNo  string `json:"merchant_order_no" binding:"required"`
	MerchantRefundNo string `json:"merchant_refund_no" binding:"required"`
	Amount           int64  `json:"amount" binding:"required,min=1"`
	Reason           string `json:"reason"`
}

func defaultReason(r string) string {
	if r == "" {
		return "商家退款"
	}
	return r
}

func (h *PaymentHandler) Refund(c *gin.Context) {
	var req refundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	m := middleware.GetMerchant(c)
	h.log.Info("refund request",
		zap.Int64("merchant_id", m.ID),
		zap.String("merchant_order_no", req.MerchantOrderNo),
		zap.String("merchant_refund_no", req.MerchantRefundNo),
		zap.Int64("amount", req.Amount))
	ro, err := h.svc.Refund(c.Request.Context(), payment.RefundInput{
		MerchantID:       m.ID,
		MerchantOrderNo:  req.MerchantOrderNo,
		MerchantRefundNo: req.MerchantRefundNo,
		Amount:           req.Amount,
		Reason:           defaultReason(req.Reason),
	})
	if err != nil {
		h.log.Error("refund failed",
			zap.Int64("merchant_id", m.ID),
			zap.String("merchant_order_no", req.MerchantOrderNo),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "REFUND_FAILED", "msg": "refund failed"})
		return
	}
	h.log.Info("refund submitted",
		zap.Int64("merchant_id", m.ID),
		zap.String("refund_no", ro.RefundNo),
		zap.String("status", string(ro.Status)))
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": ro})
}
