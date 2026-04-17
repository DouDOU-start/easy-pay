package setup

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	_ "gorm.io/driver/postgres"

	"github.com/easypay/easy-pay/migrations"
)

// Handler serves the first-run setup wizard API.
type Handler struct {
	configPath string
	once       sync.Once
	installed  bool
}

func New(configPath string) *Handler {
	return &Handler{configPath: configPath}
}

// --- request types ---

type TestDBReq struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
	SSLMode  string `json:"ssl_mode"`
}

func (r *TestDBReq) applyDefaults() {
	if r.Host == "" {
		r.Host = "localhost"
	}
	if r.Port == 0 {
		r.Port = 15432
	}
	if r.User == "" {
		r.User = "easypay"
	}
	if r.DBName == "" {
		r.DBName = "easypay"
	}
}

type TestRedisReq struct {
	Addr     string `json:"addr"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

func (r *TestRedisReq) applyDefaults() {
	if r.Addr == "" {
		r.Addr = "localhost:6379"
	}
}

type InstallReq struct {
	DB    TestDBReq    `json:"db" binding:"required"`
	Redis TestRedisReq `json:"redis" binding:"required"`
	Admin struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	} `json:"admin" binding:"required"`
}

// --- endpoints ---

// Status returns whether setup is required.
func (h *Handler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"setup_required": !h.installed})
}

// StatusCompleted is used in normal mode — setup is already done.
func StatusCompleted(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"setup_required": false})
}

// TestDB validates a PostgreSQL connection. It first connects to the system
// "postgres" database to verify credentials, then checks whether the target
// database exists (it will be auto-created during install if missing).
func (h *Handler) TestDB(c *gin.Context) {
	if h.installed {
		c.JSON(http.StatusForbidden, gin.H{"code": "ALREADY_INSTALLED"})
		return
	}
	var req TestDBReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	req.applyDefaults()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Connect via the system "postgres" database to verify host/user/password.
	sysReq := req
	sysReq.DBName = "postgres"
	sysDB, err := sql.Open("pgx", buildDSN(sysReq))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "DB_ERROR", "msg": friendlyDBError(err)})
		return
	}
	defer sysDB.Close()
	if err := sysDB.PingContext(ctx); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "DB_ERROR", "msg": friendlyDBError(err)})
		return
	}

	// Check if the target database already exists.
	var exists bool
	err = sysDB.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", req.DBName,
	).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "DB_ERROR", "msg": friendlyDBError(err)})
		return
	}
	if exists {
		c.JSON(http.StatusOK, gin.H{"code": "OK", "msg": "数据库连接成功"})
	} else {
		c.JSON(http.StatusOK, gin.H{"code": "OK", "msg": fmt.Sprintf("连接成功，数据库 \"%s\" 不存在，安装时将自动创建", req.DBName)})
	}
}

// TestRedis validates a Redis connection.
func (h *Handler) TestRedis(c *gin.Context) {
	if h.installed {
		c.JSON(http.StatusForbidden, gin.H{"code": "ALREADY_INSTALLED"})
		return
	}
	var req TestRedisReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": err.Error()})
		return
	}
	req.applyDefaults()
	rdb := redis.NewClient(&redis.Options{
		Addr:     req.Addr,
		Password: req.Password,
		DB:       req.DB,
	})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "REDIS_ERROR", "msg": friendlyRedisError(err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "msg": "Redis 连接成功"})
}

// Install performs the full installation: write config, run migrations, create admin.
func (h *Handler) Install(c *gin.Context) {
	if h.installed {
		c.JSON(http.StatusForbidden, gin.H{"code": "ALREADY_INSTALLED"})
		return
	}

	var req InstallReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "msg": friendlyValidationError(err)})
		return
	}

	var installErr error
	h.once.Do(func() {
		installErr = h.doInstall(c.Request.Context(), req)
	})
	if installErr != nil {
		// reset once so user can retry
		h.once = sync.Once{}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INSTALL_FAILED", "msg": installErr.Error()})
		return
	}

	h.installed = true
	c.JSON(http.StatusOK, gin.H{"code": "OK", "msg": "安装成功，服务即将重启"})

	// Give the HTTP response time to flush, then exit so the process manager restarts us.
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

func (h *Handler) doInstall(ctx context.Context, req InstallReq) error {
	req.DB.applyDefaults()
	req.Redis.applyDefaults()

	// 1. Ensure the target database exists (create it if missing).
	if err := ensureDatabase(ctx, req.DB); err != nil {
		return err
	}

	// 2. Connect to the target database.
	dsn := buildDSN(req.DB)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("数据库连接失败: %w", err)
	}

	// 2. Test Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     req.Redis.Addr,
		Password: req.Redis.Password,
		DB:       req.Redis.DB,
	})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis 连接失败: %w", err)
	}

	// 3. Run migrations
	if err := runMigrations(ctx, db); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	// 4. Create admin user
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Admin.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码哈希失败: %w", err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO admin_users (username, password_hash, role, status)
		VALUES ($1, $2, 'admin', 1)
		ON CONFLICT (username) DO UPDATE
		SET password_hash = EXCLUDED.password_hash,
		    role          = EXCLUDED.role,
		    status        = EXCLUDED.status,
		    updated_at    = NOW()
	`, req.Admin.Email, string(hash))
	if err != nil {
		return fmt.Errorf("创建管理员失败: %w", err)
	}

	// 5. Generate crypto key
	cryptoKey := make([]byte, 32)
	if _, err := rand.Read(cryptoKey); err != nil {
		return fmt.Errorf("生成加密密钥失败: %w", err)
	}

	// 6. Write config file
	if err := h.writeConfig(req, hex.EncodeToString(cryptoKey[:16])); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// --- migrations ---

func runMigrations(ctx context.Context, db *sql.DB) error {
	entries, err := fs.ReadDir(migrations.SQLFiles, ".")
	if err != nil {
		return err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		data, err := fs.ReadFile(migrations.SQLFiles, name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			return fmt.Errorf("exec %s: %w", name, err)
		}
	}
	return nil
}

// --- config file ---

var configTmpl = template.Must(template.New("config").Parse(`server:
  addr: ":8080"
  mode: release
  read_timeout: 15s
  write_timeout: 15s

database:
  dsn: "{{.DSN}}"
  max_idle_conns: 10
  max_open_conns: 100
  log_level: info

redis:
  addr: "{{.RedisAddr}}"
  password: "{{.RedisPassword}}"
  db: {{.RedisDB}}

crypto:
  key: "{{.CryptoKey}}"

notify:
  timeout: 10s
  max_retries: 8
  backoff_seconds: [15, 60, 300, 900, 1800, 3600, 7200, 14400]

log:
  level: info
  format: json
`))

func (h *Handler) writeConfig(req InstallReq, cryptoKey string) error {
	path := h.configPath
	if path == "" {
		path = "configs/config.yaml"
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return configTmpl.Execute(f, map[string]any{
		"DSN":           buildDSN(req.DB),
		"RedisAddr":     req.Redis.Addr,
		"RedisPassword": req.Redis.Password,
		"RedisDB":       req.Redis.DB,
		"CryptoKey":     cryptoKey,
	})
}

// ensureDatabase connects to the "postgres" system database and creates the
// target database if it does not already exist.
func ensureDatabase(ctx context.Context, req TestDBReq) error {
	sysReq := req
	sysReq.DBName = "postgres"
	sysDB, err := sql.Open("pgx", buildDSN(sysReq))
	if err != nil {
		return fmt.Errorf("连接系统数据库失败: %w", err)
	}
	defer sysDB.Close()

	var exists bool
	if err := sysDB.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", req.DBName,
	).Scan(&exists); err != nil {
		return fmt.Errorf("检查数据库失败: %w", err)
	}
	if exists {
		return nil
	}

	// CREATE DATABASE cannot run inside a transaction, so use Exec directly.
	// Identifier is safe here because we validated it via binding:"required".
	if _, err := sysDB.ExecContext(ctx,
		fmt.Sprintf(`CREATE DATABASE "%s"`, req.DBName),
	); err != nil {
		return fmt.Errorf("创建数据库 \"%s\" 失败: %w", req.DBName, err)
	}
	return nil
}

// --- user-friendly errors ---

func friendlyDBError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "password authentication failed"):
		return "用户名或密码错误"
	case strings.Contains(msg, "connection refused"):
		return "无法连接到数据库，请检查主机地址和端口"
	case strings.Contains(msg, "timeout"):
		return "连接超时，请检查主机地址和端口是否正确"
	case strings.Contains(msg, "no such host"):
		return "主机地址无法解析，请检查输入"
	case strings.Contains(msg, "does not exist"):
		return "数据库不存在"
	default:
		return "数据库连接失败，请检查配置"
	}
}

func friendlyValidationError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "Email"):
		return "请填写管理员邮箱"
	case strings.Contains(msg, "Password"):
		return "请填写管理员密码（至少 6 位）"
	default:
		return "请完善安装信息"
	}
}

func friendlyRedisError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "connection refused"):
		return "无法连接到 Redis，请检查地址和端口"
	case strings.Contains(msg, "NOAUTH") || strings.Contains(msg, "AUTH"):
		return "Redis 认证失败，请检查密码"
	case strings.Contains(msg, "timeout"):
		return "连接超时，请检查地址是否正确"
	case strings.Contains(msg, "no such host"):
		return "地址无法解析，请检查输入"
	default:
		return "Redis 连接失败，请检查配置"
	}
}

// --- helpers ---

func buildDSN(r TestDBReq) string {
	sslMode := r.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=Asia/Shanghai",
		r.Host, r.User, r.Password, r.DBName, r.Port, sslMode,
	)
}
