package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"

	mysql_driver "github.com/go-sql-driver/mysql"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	sqltrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
	gormtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorm.io/gorm.v1"
	gorm_mysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// mustInitDB opens a MySQL connection using env() variables and migrates the base model.
func mustInitDB() *gorm.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4,utf8",
		env("DB_USER", "taskly"),
		env("DB_PASSWORD", "taskly_password"),
		env("DB_HOST", "127.0.0.1"),
		env("DB_PORT", "3306"),
		env("DB_NAME", "taskly"))
	// Register MySQL driver for Datadog tracing
	dbService := env("DD_DB_SERVICE", env("DD_SERVICE", "taskly-server")+"-db")
	sqltrace.Register("mysql", &mysql_driver.MySQLDriver{}, sqltrace.WithServiceName(dbService))
	// Open traced GORM DB
	db, err := gormtrace.Open(gorm_mysql.Open(dsn), &gorm.Config{}, gormtrace.WithServiceName(dbService))
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	return db
}

// mustInitRedis connects to Redis and verifies availability.
func mustInitRedis() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     env("REDIS_ADDR", "127.0.0.1:6379"),
		Password: env("REDIS_PASSWORD", ""),
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
	mux.Handle("/metrics", promhttp.Handler())
	ln, err := net.Listen("tcp", bind)
	if err != nil {
		log.Printf("metrics listen err: %v", err)
		return
	}
	go func() { _ = http.Serve(ln, mux) }()
}

// initWebAuthn configures the global WebAuthn relying party if possible.
func initWebAuthn() {
	if wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: env("WEBAUTHN_RP_NAME", "Taskly"),
		RPID:          env("WEBAUTHN_RP_ID", "localhost"),
		RPOrigin:      env("WEBAUTHN_ORIGIN", "http://localhost:4321"),
	}); err != nil {
		log.Printf("webauthn init error: %v", err)
	} else {
		WAuth = wa
	}
}
