// Package defaults centralizes the default values used by the config loader,
// the setup wizard backend, and the setup wizard frontend.
//
// Single source of truth: changing a port here propagates to viper defaults,
// the install wizard's request fallbacks, and the /setup/defaults API the SPA
// reads on first paint. Environment variables (loaded from .env at process
// start) override anything set here.
package defaults

import (
	"fmt"
	"os"
	"strconv"
)

type Values struct {
	DBHost     string `json:"db_host"`
	DBPort     int    `json:"db_port"`
	DBUser     string `json:"db_user"`
	DBPassword string `json:"-"`
	DBName     string `json:"db_name"`
	RedisAddr  string `json:"redis_addr"`
}

func FromEnv() Values {
	return Values{
		DBHost:     envOr("EASYPAY_DB_HOST", "localhost"),
		DBPort:     envOrInt("EASYPAY_DB_PORT", 15432),
		DBUser:     envOr("EASYPAY_DB_USER", "easypay"),
		DBPassword: envOr("EASYPAY_DB_PASSWORD", "easypay"),
		DBName:     envOr("EASYPAY_DB_NAME", "easypay"),
		RedisAddr: fmt.Sprintf("%s:%s",
			envOr("EASYPAY_REDIS_HOST", "localhost"),
			envOr("EASYPAY_REDIS_PORT", "6379"),
		),
	}
}

// DSN composes a Postgres DSN from these defaults — used as viper's fallback
// when configs/config.yaml omits database.dsn.
func (v Values) DSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai",
		v.DBHost, v.DBUser, v.DBPassword, v.DBName, v.DBPort,
	)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
