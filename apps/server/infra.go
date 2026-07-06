package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/getsentry/sentry-go"
	mysql_driver "github.com/go-sql-driver/mysql"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	promHttp "github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	sqlTrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
	gormTrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorm.io/gorm.v1"
	gorm_mysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DB接続を開く
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

// mustInitSentry initializes Sentry when configured. If SENTRY_DSN is missing,
// it reports the misconfiguration to Discord and skips Sentry setup.
func mustInitSentry() bool {
	dsn := utils.Env("SENTRY_DSN", "")
	if strings.TrimSpace(dsn) == "" {
		go utils.SendDiscordEmbeds([]map[string]any{{
			"title":       "Missing SENTRY_DSN",
			"description": "Sentry was not initialized because SENTRY_DSN is not set.",
			"color":       15158332,
			"timestamp":   time.Now().Format(time.RFC3339),
			"fields": []map[string]any{
				{"name": "variable", "value": "SENTRY_DSN", "inline": true},
				{"name": "service", "value": "taskly-server", "inline": true},
				{"name": "host", "value": utils.Env("HOSTNAME", utils.Env("API_HOST", "unknown")), "inline": true},
			},
		}})
		log.Printf("sentry: skipped (no SENTRY_DSN)")
		return false
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		TracesSampleRate: 0.1,
		Environment:      utils.Env("SENTRY_ENVIRONMENT", utils.Env("NODE_ENV", "development")),
		Release:          utils.Env("SENTRY_RELEASE", ""),
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if event == nil {
				return event
			}
			if event.Request != nil {
				event.Request.Cookies = ""
				event.Request.Data = ""
				if event.Request.Headers != nil {
					clean := map[string]string{}
					for k, v := range event.Request.Headers {
						switch strings.ToLower(k) {
						case "authorization", "x-api-key", "cookie", "set-cookie":
							clean[k] = "[REDACTED]"
						default:
							clean[k] = v
						}
					}
					event.Request.Headers = clean
				}
				if event.Request.URL != "" {
					event.Request.URL = utils.SanitizeURLPath(event.Request.URL)
				}
				if event.Request.QueryString != "" {
					event.Request.QueryString = utils.SanitizeQueryString(event.Request.QueryString)
				}
			}
			if event.Contexts != nil {
				for k, ctx := range event.Contexts {
					utils.ScrubPIIMap(ctx)
					event.Contexts[k] = ctx
				}
			}
			if event.Tags != nil {
				for k, v := range event.Tags {
					if utils.IsSensitiveKey(k) || utils.IsSensitiveValue(v) {
						event.Tags[k] = "[REDACTED]"
					}
				}
			}
			return event
		},
		BeforeBreadcrumb: func(crumb *sentry.Breadcrumb, hint *sentry.BreadcrumbHint) *sentry.Breadcrumb {
			if crumb == nil {
				return crumb
			}
			if strings.EqualFold(crumb.Category, "http") && crumb.Data != nil {
				if urlV, ok := crumb.Data["url"].(string); ok && urlV != "" {
					crumb.Data["url"] = utils.SanitizeURLPath(urlV)
				}
			}
			if crumb.Data != nil {
				for k := range crumb.Data {
					if utils.IsSensitiveKey(fmt.Sprintf("%v", k)) {
						crumb.Data[k] = "[REDACTED]"
					}
				}
			}
			return crumb
		},
	}); err != nil {
		go utils.SendDiscordEmbeds([]map[string]any{{
			"title":       "Sentry initialization failed",
			"description": fmt.Sprintf("sentry.Init returned an error: %v", err),
			"color":       15158332,
			"timestamp":   time.Now().Format(time.RFC3339),
			"fields": []map[string]any{
				{"name": "variable", "value": "SENTRY_DSN", "inline": true},
				{"name": "service", "value": "taskly-server", "inline": true},
			},
		}})
		log.Printf("sentry init error: %v", err)
		return false
	}

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("service", "taskly-server")
		scope.SetTag("port", utils.Env("API_PORT", "8080"))
	})
	sentry.CaptureMessage("Taskly server starting")
	return true
}

// metricsServer starts a Prometheus metrics HTTP server on the given bind address.
func metricsServer(bind string) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true, // 起動ログが邪魔な場合は無効に
	})

	// 2. prometheus の http.Handler を Fiber のハンドラーに変換して登録
	app.Get("/metrics", adaptor.HTTPHandler(promHttp.Handler()))

	// 3. リスナーを作成してバックグラウンドで起動
	ln, err := net.Listen("tcp", bind)
	if err != nil {
		log.Printf("metrics listen err: %v", err)
		return
	}

	go func() {
		if err := app.Listener(ln); err != nil {
			log.Printf("metrics server err: %v", err)
		}
	}()
}

// initWebAuthn configures the global WebAuthn relying party if possible.
func initWebAuthn() *webauthn.WebAuthn {
	var wAuth *webauthn.WebAuthn
	if wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: utils.Env("WEBAUTHN_RP_NAME", "Taskly"),
		RPID:          utils.Env("WEBAUTHN_RP_ID", "localhost"),
		RPOrigins:     []string{utils.Env("WEBAUTHN_ORIGIN", "http://localhost:4321")},
	}); err != nil {
		log.Printf("webauthn init error: %v", err)
	} else {
		wAuth = wa
	}
	return wAuth
}
