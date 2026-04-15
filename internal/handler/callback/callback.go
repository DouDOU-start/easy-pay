package callback

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/easypay/easy-pay/internal/channel"
	"github.com/easypay/easy-pay/internal/model"
	"github.com/easypay/easy-pay/internal/service/payment"
)

type Handler struct {
	svc      *payment.Service
	registry channel.Registry
	log      *zap.Logger
}

func New(svc *payment.Service, reg channel.Registry, log *zap.Logger) *Handler {
	return &Handler{svc: svc, registry: reg, log: log}
}

// Receive is mounted at /callback/:channel/:merchant_id and dispatches
// incoming provider callbacks through the resolved channel implementation.
func (h *Handler) Receive(c *gin.Context) {
	chName := model.Channel(c.Param("channel"))
	merchantID, err := strconv.ParseInt(c.Param("merchant_id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "bad merchant id")
		return
	}

	impl, err := h.registry.Resolve(c.Request.Context(), merchantID, chName)
	if err != nil {
		h.log.Warn("callback resolve failed", zap.Error(err))
		c.String(http.StatusNotFound, "unknown channel")
		return
	}
	ev, err := impl.ParseNotify(c.Request.Context(), c.Request)
	if err != nil {
		h.log.Warn("parse notify failed", zap.Error(err))
		c.String(http.StatusBadRequest, "bad notify")
		return
	}
	if err := h.svc.HandlePaymentNotify(c.Request.Context(), ev); err != nil {
		h.log.Error("handle notify failed", zap.Error(err))
		c.String(http.StatusInternalServerError, "internal error")
		return
	}
	ct, body := impl.NotifyAck()
	c.Data(http.StatusOK, ct, body)
}
