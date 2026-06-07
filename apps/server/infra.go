package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"

	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	mysql_driver "github.com/go-sql-driver/mysql"
	"github.com/go-webauthn/webauthn/webauthn"
	promHttp "github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	sqlTrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
	gormTrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorm.io/gorm.v1"
	gorm_mysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// mustInitDB opens a MySQL connection using env() variables and migrates the base model.
func mustInitDB() *gorm.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4,utf8&loc=Asia%%2FTokyo",
		utils.MustEnv("MYSQL_USER"),
		utils.MustEnv("MYSQL_PASSWORD"),
		utils.MustEnv("DB_HOST"),
		utils.MustEnv("DB_PORT"),
		utils.MustEnv("MYSQL_DATABASE"))
	// Register MySQL driver for Datadog tracing
	dbService := utils.Env("DD_DB_SERVICE", utils.Env("DD_SERVICE", "taskly-server")+"-db")
	sqlTrace.Register("mysql", &mysql_driver.MySQLDriver{}, sqlTrace.WithServiceName(dbService))
	// Open traced GORM DB
	db, err := gormTrace.Open(gorm_mysql.Open(dsn), &gorm.Config{}, gormTrace.WithServiceName(dbService))
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	return db
}

// mustInitRedis connects to Redis and verifies availability.
func mustInitRedis() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     utils.Env("REDIS_ADDR", "127.0.0.1:6379"),
		Password: utils.Env("REDIS_PASSWORD", ""),
		DB:       0,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis: %v", err)
	}
	return rdb
}

// metricsServer starts a Prometheus metrics HTTP server on the given bind address.
func metricsServer(bind string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promHttp.Handler())
	ln, err := net.Listen("tcp", bind)
	if err != nil {
		log.Printf("metrics listen err: %v", err)
		return
	}
	go func() { _ = http.Serve(ln, mux) }()
}

// initWebAuthn configures the global WebAuthn relying party if possible.
func initWebAuthn() *webauthn.WebAuthn {
	if wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: utils.Env("WEBAUTHN_RP_NAME", "Taskly"),
		RPID:          utils.Env("WEBAUTHN_RP_ID", "localhost"),
		RPOrigins:     []string{utils.Env("WEBAUTHN_ORIGIN", "http://localhost:4321")},
	}); err != nil {
		log.Printf("webauthn init error: %v", err)
	} else {
		WAuth = wa
	}
	return WAuth
}
