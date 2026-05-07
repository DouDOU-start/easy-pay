package callback

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/easypay/easy-pay/backend/internal/channel"
	"github.com/easypay/easy-pay/backend/internal/model"
	"github.com/easypay/easy-pay/backend/internal/service/payment"
)

type Handler struct {
	svc      *payment.Service
	registry channel.Registry
	log      *zap.Logger
}

func New(svc *payment.Service, reg channel.Registry, log *zap.Logger) *Handler {
	return &Handler{svc: svc, registry: reg, log: log}
}

func (h *Handler) Receive(c *gin.Context) {
	chName := model.Channel(c.Param("channel"))
	merchantID, err := strconv.ParseInt(c.Param("merchant_id"), 10, 64)
	if err != nil {
		h.log.Warn("callback bad merchant_id",
			zap.String("raw", c.Param("merchant_id")),
			zap.String("client_ip", c.ClientIP()))
		c.String(http.StatusBadRequest, "bad merchant id")
		return
	}

	impl, err := h.registry.Resolve(c.Request.Context(), merchantID, chName)
	if err != nil {
		h.log.Warn("callback resolve failed",
			zap.String("channel", string(chName)),
			zap.Int64("merchant_id", merchantID),
			zap.Error(err))
		c.String(http.StatusNotFound, "unknown channel")
		return
	}
	ev, err := impl.ParseNotify(c.Request.Context(), c.Request)
	if err != nil {
		h.log.Warn("callback parse notify failed",
			zap.String("channel", string(chName)),
			zap.Int64("merchant_id", merchantID),
			zap.Error(err))
		c.String(http.StatusBadRequest, "bad notify")
		return
	}
	h.log.Info("callback received",
		zap.String("channel", string(chName)),
		zap.Int64("merchant_id", merchantID),
		zap.String("order_no", ev.OrderNo),
		zap.String("channel_order_no", ev.ChannelOrderNo),
		zap.Int64("amount", ev.Amount))
	if err := h.svc.HandlePaymentNotify(c.Request.Context(), ev); err != nil {
		h.log.Error("callback handle notify failed",
			zap.String("channel", string(chName)),
			zap.Int64("merchant_id", merchantID),
			zap.String("order_no", ev.OrderNo),
			zap.Error(err))
		c.String(http.StatusInternalServerError, "internal error")
		return
	}
	ct, body := impl.NotifyAck()
	c.Data(http.StatusOK, ct, body)
}
