// Package merchant exposes the self-service endpoints that a merchant (not the
// platform admin) uses to log in, read its own bills, and update its own
// profile / password. Authentication is a Redis session token, mirroring the
// admin auth pattern; the session key space is deliberately disjoint so a
// merchant token can never reach an admin route and vice versa.
package merchant

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/easypay/easy-pay/internal/repository"
)

const (
	merchantCtxKey   = "ctx_merchant_id"
	sessionTTL       = 24 * time.Hour
	sessionKeyPrefix = "merchant:session:"
	bearerPrefix     = "Bearer "
)

type AuthHandler struct {
	merchants repository.MerchantRepo
	rdb       *redis.Client
}

func NewAuthHandler(m repository.MerchantRepo, rdb *redis.Client) *AuthHandler {
	return &AuthHandler{merchants: m, rdb: rdb}
}

type loginReq struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	m, err := h.merchants.GetByEmail(c.Request.Context(), email)
	if err != nil || m.Status != 1 || m.PasswordHash == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED", "msg": "邮箱或密码不正确"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(m.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED", "msg": "邮箱或密码不正确"})
		return
	}
	token, err := h.newSession(c.Request.Context(), m.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SESSION_FAILED", "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"token":      token,
		"expires_in": int(sessionTTL.Seconds()),
		"merchant": gin.H{
			"id": m.ID, "mch_no": m.MchNo, "name": m.Name, "email": m.Email,
		},
	}})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if token := extractToken(c); token != "" {
		_ = h.rdb.Del(c.Request.Context(), sessionKeyPrefix+token).Err()
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK"})
}

// Middleware validates "Authorization: Bearer <token>" against Redis and
// stashes the merchant id (string form) in the context. Handlers should read
// it back via CurrentMerchantID.
func (h *AuthHandler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "AUTH_MISSING"})
			return
		}
		idStr, err := h.rdb.Get(c.Request.Context(), sessionKeyPrefix+token).Result()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "AUTH_INVALID"})
			return
		}
		c.Set(merchantCtxKey, idStr)
		_ = h.rdb.Expire(c.Request.Context(), sessionKeyPrefix+token, sessionTTL).Err()
		c.Next()
	}
}

func (h *AuthHandler) newSession(ctx context.Context, merchantID int64) (string, error) {
	token := randomToken()
	if err := h.rdb.Set(ctx, sessionKeyPrefix+token, merchantID, sessionTTL).Err(); err != nil {
		return "", err
	}
	return token, nil
}

func extractToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if len(h) < len(bearerPrefix) || h[:len(bearerPrefix)] != bearerPrefix {
		return ""
	}
	return h[len(bearerPrefix):]
}

func randomToken() string {
	b := make([]byte, 24)
	_, _ = cryptorand.Read(b)
	return hex.EncodeToString(b)
}

// CurrentMerchantID reads the merchant id that the middleware stashed.
// Returns 0 + error if not present, which handlers should treat as 401.
func CurrentMerchantID(c *gin.Context) (int64, error) {
	v, ok := c.Get(merchantCtxKey)
	if !ok {
		return 0, errors.New("merchant: no session")
	}
	s, ok := v.(string)
	if !ok {
		return 0, errors.New("merchant: bad session value")
	}
	var id int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, errors.New("merchant: bad session value")
		}
		id = id*10 + int64(ch-'0')
	}
	return id, nil
}
