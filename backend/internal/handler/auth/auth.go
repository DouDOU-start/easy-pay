package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/easypay/easy-pay/backend/internal/handler/httputil"
	"github.com/easypay/easy-pay/backend/internal/repository"
)

const (
	sessionTTL       = 24 * time.Hour
	sessionKeyPrefix = "session:"
	ctxUserID        = "ctx_user_id"
	ctxUserRole      = "ctx_user_role"
)

type sessionData struct {
	ID   int64  `json:"id"`
	Role string `json:"role"`
}

type Handler struct {
	merchants repository.MerchantRepo
	rdb       *redis.Client
}

func New(merchants repository.MerchantRepo, rdb *redis.Client) *Handler {
	return &Handler{merchants: merchants, rdb: rdb}
}

type loginReq struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
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
	token, err := h.newSession(c.Request.Context(), m.ID, m.Role)
	if err != nil {
		httputil.Fail500(c, "SESSION_FAILED", "创建会话失败", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"token":      token,
		"expires_in": int(sessionTTL.Seconds()),
		"user": gin.H{
			"id":    m.ID,
			"name":  m.Name,
			"email": m.Email,
			"role":  m.Role,
		},
	}})
}

func (h *Handler) Logout(c *gin.Context) {
	if token := httputil.ExtractBearerToken(c); token != "" {
		_ = h.rdb.Del(c.Request.Context(), sessionKeyPrefix+token).Err()
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK"})
}

func (h *Handler) Me(c *gin.Context) {
	id, _ := CurrentUserID(c)
	m, err := h.merchants.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTH_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "data": gin.H{
		"id":    m.ID,
		"name":  m.Name,
		"email": m.Email,
		"role":  m.Role,
	}})
}

// Middleware authenticates any logged-in user (admin or merchant).
func (h *Handler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := httputil.ExtractBearerToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "AUTH_MISSING"})
			return
		}
		raw, err := h.rdb.Get(c.Request.Context(), sessionKeyPrefix+token).Result()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "AUTH_INVALID"})
			return
		}
		var sess sessionData
		if err := json.Unmarshal([]byte(raw), &sess); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "AUTH_INVALID"})
			return
		}
		c.Set(ctxUserID, sess.ID)
		c.Set(ctxUserRole, sess.Role)
		_ = h.rdb.Expire(c.Request.Context(), sessionKeyPrefix+token, sessionTTL).Err()
		c.Next()
	}
}

// RequireAdmin is middleware that must follow Middleware(). It rejects non-admin users.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(ctxUserRole)
		if role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "msg": "需要管理员权限"})
			return
		}
		c.Next()
	}
}

func (h *Handler) newSession(ctx context.Context, userID int64, role string) (string, error) {
	token := httputil.RandomToken()
	data, _ := json.Marshal(sessionData{ID: userID, Role: role})
	if err := h.rdb.Set(ctx, sessionKeyPrefix+token, data, sessionTTL).Err(); err != nil {
		return "", err
	}
	return token, nil
}

func CurrentUserID(c *gin.Context) (int64, error) {
	v, ok := c.Get(ctxUserID)
	if !ok {
		return 0, errors.New("auth: no session")
	}
	id, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("auth: unexpected id type %T", v)
	}
	return id, nil
}

func CurrentUserRole(c *gin.Context) string {
	v, _ := c.Get(ctxUserRole)
	s, _ := v.(string)
	return s
}
