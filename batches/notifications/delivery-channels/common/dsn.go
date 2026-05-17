package common

import (
	"fmt"
	"strings"
)

type MySQLConfig struct {
	User     string
	Password string
	Host     string
	Port     string
	Database string
	Params   string
}

func BuildDSN(cfg MySQLConfig) string {
	params := cfg.Params
	if strings.TrimSpace(params) == "" {
		params = "parseTime=true&charset=utf8mb4&loc=Local"
	}
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?%s",
		ValueOr(cfg.User, ""), ValueOr(cfg.Password, ""), HostPort(cfg.Host, cfg.Port), ValueOr(cfg.Database, ""), params)
}

func HostPort(h, p string) string {
	if strings.TrimSpace(p) == "" {
		p = "3306"
	}
	return fmt.Sprintf("%s:%s", ValueOr(h, "localhost"), p)
}

func ValueOr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
