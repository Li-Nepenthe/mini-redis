package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr       string
	DSN            string
	JWTSecret      string
	JWTTTL         time.Duration
	DBMaxOpen      int
	DBMaxIdle      int
	DBConnLifetime time.Duration
	MonthlyLimit   int64
}

func Load() (Config, error) {
	c := Config{
		HTTPAddr: get("HTTP_ADDR", "127.0.0.1:8080"),
		DSN:      os.Getenv("DATABASE_DSN"), JWTSecret: os.Getenv("JWT_SECRET"),
		JWTTTL: time.Hour, DBMaxOpen: 20, DBMaxIdle: 10,
		DBConnLifetime: 5 * time.Minute, MonthlyLimit: 100000,
	}
	if c.DSN == "" || len(c.JWTSecret) < 32 {
		return c, errors.New("DATABASE_DSN and JWT_SECRET (at least 32 bytes) are required")
	}
	for name, target := range map[string]*int{"DB_MAX_OPEN": &c.DBMaxOpen, "DB_MAX_IDLE": &c.DBMaxIdle} {
		if value := os.Getenv(name); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 {
				return c, fmt.Errorf("%s must be a positive integer", name)
			}
			*target = n
		}
	}
	for name, target := range map[string]*time.Duration{"JWT_TTL": &c.JWTTTL, "DB_CONN_LIFETIME": &c.DBConnLifetime} {
		if value := os.Getenv(name); value != "" {
			d, err := time.ParseDuration(value)
			if err != nil || d <= 0 {
				return c, fmt.Errorf("%s must be a positive duration", name)
			}
			*target = d
		}
	}
	if value := os.Getenv("MONTHLY_TOKEN_LIMIT"); value != "" {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n < 1 {
			return c, errors.New("MONTHLY_TOKEN_LIMIT must be positive")
		}
		c.MonthlyLimit = n
	}
	if c.DBMaxIdle > c.DBMaxOpen {
		return c, errors.New("DB_MAX_IDLE must not exceed DB_MAX_OPEN")
	}
	if !strings.Contains(c.HTTPAddr, ":") {
		return c, errors.New("HTTP_ADDR must contain a port")
	}
	return c, nil
}

func get(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
