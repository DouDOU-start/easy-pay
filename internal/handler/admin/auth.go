package admin

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/easypay/easy-pay/internal/model"
)

const (
	adminCtxKey       = "ctx_admin_user"
	sessionTTL        = 24 * time.Hour
	sessionKeyPrefix  = "admin:session:"
	bearerPrefix      = "Bearer "
)

type AuthHandler struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewAuthHandler(db *gorm.DB, rdb *redis.Client) *AuthHandler {
	return &AuthHandler{db: db, rdb: rdb}
}

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	var u model.AdminUser
	if err := h.db.WithContext(c.Request.Context()).
		Where("username = ? AND status = 1", req.Username).
		First(&u).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED", "msg": "invalid credentials"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED", "msg": "invalid credentials"})
		return
	}
	token, err := h.newSession(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SESSION_FAILED", "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"token":      token,
		"expires_in": int(sessionTTL.Seconds()),
		"user":       gin.H{"id": u.ID, "username": u.Username, "role": u.Role},
	}})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if token := extractToken(c); token != "" {
		_ = h.rdb.Del(c.Request.Context(), sessionKeyPrefix+token).Err()
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK"})
}

func (h *AuthHandler) Me(c *gin.Context) {
	uid, _ := c.Get(adminCtxKey)
	var u model.AdminUser
	if err := h.db.WithContext(c.Request.Context()).First(&u, uid).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"id": u.ID, "username": u.Username, "role": u.Role,
	}})
}

// Middleware validates "Authorization: Bearer <token>" against Redis.
func (h *AuthHandler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "AUTH_MISSING"})
			return
		}
		uidStr, err := h.rdb.Get(c.Request.Context(), sessionKeyPrefix+token).Result()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "AUTH_INVALID"})
			return
		}
		c.Set(adminCtxKey, uidStr)
		// slide the session so active admins stay logged in
		_ = h.rdb.Expire(c.Request.Context(), sessionKeyPrefix+token, sessionTTL).Err()
		c.Next()
	}
}

func (h *AuthHandler) newSession(ctx context.Context, userID int64) (string, error) {
	token := randomToken()
	key := sessionKeyPrefix + token
	if err := h.rdb.Set(ctx, key, userID, sessionTTL).Err(); err != nil {
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


