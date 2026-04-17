// Package testsink exposes the dev-only callback receiver. Requests posted
// to /test/notify/* are captured into an in-memory ring buffer and can be
// inspected from the admin console. The public endpoint always returns 200
// so upstream retries don't loop.
package testsink

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/easypay/easy-pay/internal/testsink"
)

const maxBodyBytes = 64 * 1024

type Handler struct {
	sink *testsink.Sink
}

func New(sink *testsink.Sink) *Handler {
	return &Handler{sink: sink}
}

func (h *Handler) Receive(c *gin.Context) {
	slot := strings.TrimPrefix(c.Param("slot"), "/")

	body, truncated := readLimited(c.Request.Body, maxBodyBytes)
	_ = c.Request.Body.Close()

	headers := make(map[string]string, len(c.Request.Header))
	for k, v := range c.Request.Header {
		headers[k] = strings.Join(v, ", ")
	}

	h.sink.Push(testsink.Record{
		Method:    c.Request.Method,
		Path:      c.Request.URL.Path,
		Slot:      slot,
		RemoteIP:  c.ClientIP(),
		Query:     c.Request.URL.RawQuery,
		Headers:   headers,
		Body:      string(body),
		BodySize:  len(body),
		Truncated: truncated,
	})

	c.JSON(http.StatusOK, gin.H{"code": "OK", "msg": "captured"})
}

func (h *Handler) List(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"list":     h.sink.List(),
		"capacity": h.sink.Capacity(),
	}})
}

func (h *Handler) Clear(c *gin.Context) {
	h.sink.Clear()
	c.JSON(http.StatusOK, gin.H{"code": "OK"})
}

func readLimited(r io.Reader, limit int) ([]byte, bool) {
	buf, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err != nil || len(buf) <= limit {
		return buf, false
	}
	return buf[:limit], true
}
