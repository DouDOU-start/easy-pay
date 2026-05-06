package defaults

import "fmt"

type Values struct {
	DBHost     string `json:"db_host"`
	DBPort     int    `json:"db_port"`
	DBUser     string `json:"db_user"`
	DBPassword string `json:"-"`
	DBName     string `json:"db_name"`
	RedisAddr  string `json:"redis_addr"`
}

func Get() Values {
	return Values{
		DBHost:     "localhost",
		DBPort:     5432,
		DBUser:     "easypay",
		DBPassword: "easypay",
		DBName:     "easypay",
		RedisAddr:  "localhost:6379",
	}
}

func (v Values) DSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai",
		v.DBHost, v.DBUser, v.DBPassword, v.DBName, v.DBPort,
	)
}
