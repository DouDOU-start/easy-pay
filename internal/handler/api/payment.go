package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/easypay/easy-pay/internal/handler/middleware"
	"github.com/easypay/easy-pay/internal/model"
	"github.com/easypay/easy-pay/internal/service/payment"
)

type PaymentHandler struct {
	svc *payment.Service
}

func NewPaymentHandler(svc *payment.Service) *PaymentHandler {
	return &PaymentHandler{svc: svc}
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "CREATE_FAILED", "msg": err.Error()})
		return
	}
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
	if err := h.svc.CloseOrder(c.Request.Context(), m.ID, req.MerchantOrderNo); err != nil {
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

func (h *PaymentHandler) Refund(c *gin.Context) {
	var req refundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	m := middleware.GetMerchant(c)
	ro, err := h.svc.Refund(c.Request.Context(), payment.RefundInput{
		MerchantID:       m.ID,
		MerchantOrderNo:  req.MerchantOrderNo,
		MerchantRefundNo: req.MerchantRefundNo,
		Amount:           req.Amount,
		Reason:           req.Reason,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "REFUND_FAILED", "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": ro})
}
