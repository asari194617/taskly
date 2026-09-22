package server

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/handler"
	adminH "github.com/asari194617/taskly/apps/server/internal/handler/admin"
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql"
	mw "github.com/asari194617/taskly/apps/server/internal/middleware"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/getsentry/sentry-go"
	sentryFiber "github.com/getsentry/sentry-go/fiber"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/redis/go-redis/v9"
	fiberTrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gofiber/fiber.v2"
	"gorm.io/gorm"
)

func newServerApp(db *gorm.DB, rdb *redis.Client, wAuth *webauthn.WebAuthn, serviceName string) *fiber.App {
	middleware := mw.NewMiddleware(db, rdb, wAuth)
	adminHandler := adminH.NewAdminHandler(db, rdb, wAuth)
	appHandler := handler.NewHandler(db, rdb, wAuth)
	graphqlHandler := gqlH.NewHandler(db, rdb, wAuth)

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			status := fiber.StatusInternalServerError
			var fe *fiber.Error
			if errors.As(err, &fe) {
				status = fe.Code
			}

			hub := sentryFiber.GetHubFromContext(c)
			if hub == nil {
				hub = sentry.CurrentHub()
			}
			if hub != nil && status >= 500 {
				hub.WithScope(func(scope *sentry.Scope) {
					if r := c.Route(); r != nil {
						scope.SetTag("route", r.Path)
					}
					scope.SetTag("method", c.Method())
					scope.SetTag("url", utils.SanitizeURLPath(string(c.OriginalURL())))
					scope.SetContext("request", map[string]any{
						"query": c.Queries(),
						"ip":    c.IP(),
					})
					if u, ok := c.Locals("user").(*domain.User); ok && u != nil {
						scope.SetUser(sentry.User{
							ID:       fmt.Sprintf("%v", u.ID),
							Email:    u.Email,
							Username: u.DisplayName,
						})
					}
					scope.SetLevel(sentry.LevelError)
				})
				hub.CaptureException(err)
			}

			if status >= 500 {
				return c.Status(status).JSON(fiber.Map{"error": "internal error"})
			}
			return c.Status(status).JSON(fiber.Map{"error": err.Error()})
		},
	})
	appHandler.Start()
	app.Hooks().OnShutdown(func() error {
		appHandler.Close()
		return nil
	})

	app.Use(fiberTrace.Middleware(
		fiberTrace.WithServiceName(serviceName),
		fiberTrace.WithSpanOptions(),
		fiberTrace.WithResourceNamer(func(c *fiber.Ctx) string {
			if r := c.Route(); r != nil {
				return r.Method + " " + r.Path
			}
			return c.Method() + " " + utils.SanitizeURLPath(string(c.OriginalURL()))
		}),
	))
	app.Use(telemetryMiddleware())
	app.Use(metricsMiddleware())

	app.Use(cors.New(cors.Config{
		AllowOrigins:     utils.Env("API_CORS_ORIGINS", "http://localhost:4321,http://127.0.0.1:4321,http://localhost:3000,http://127.0.0.1:3000,http://localhost:5173,http://127.0.0.1:5173"),
		AllowCredentials: strings.ToLower(utils.Env("CORS_ALLOW_CREDENTIALS", "true")) == "true",
		AllowHeaders:     "Authorization,authorization, Content-Type,content-type, X-Client-Platform,x-client-platform, X-API-Key,x-api-key",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		MaxAge:           86400,
	}))

	app.Use(utils.RequireCookieCSRF)

	app.Use(sentryFiber.New(sentryFiber.Options{
		Repanic:         true,
		WaitForDelivery: false,
		Timeout:         2 * time.Second,
	}))

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${ip} ${status} ${latency} ${method} ${route}\n",
	}))

	app.Use(middleware.ApiKeyRouteScope)

	/* ヘルスチェック */
	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendString("ok") })

	/* GraphQL */
	graphqlHandler.RegisterGraphQL(app.Group("/graphql"))

	/* 2段階認証 */
	appHandler.Register2FARoutes(app.Group("/2fa"), middleware)

	/* 認証 */
	appHandler.RegisterAuthRoutes(app.Group("/auth"))

	/* 認証 */
	appHandler.RegisterPushRoutes(app.Group("/push"), middleware)

	/* 設定画面のiCal */
	appHandler.RegisterIcalTokensRoutes(app.Group("/ical/tokens", middleware.AuthRequired, middleware.Audit))

	/* ユーザー設定 */
	appHandler.RegisterSettingsRoutes(app.Group("/settings", middleware.AuthRequired, middleware.Audit))

	/* パスキー */
	appHandler.RegisterWebAuthnRoutes(app.Group("/webauthn"), middleware)

	/* APIキー */
	appHandler.RegisterAPIKeyRoutes(app.Group("/apikeys", middleware.AuthRequired, middleware.Audit))

	/* 課題 */
	appHandler.RegisterTasksRoutes(app.Group("/tasks", middleware.AuthRequired, middleware.Audit))

	/* 通報 */
	appHandler.RegisterReportRoutes(app.Group("reports", middleware.AuthRequired, middleware.Audit))

	/* 管理者画面: 日直 */
	adminHandler.RegisterAdminDayShiftRoutes(app.Group("/admin/dayshift", middleware.AuthRequired, middleware.AdminOnly, middleware.Audit))

	/* 管理者画面: 権限 (ロールなど) */
	adminHandler.RegisterAdminPermissionsRoutes(app.Group("/admin/permissions", middleware.AuthRequired, middleware.AdminOnly, middleware.Audit))

	/* 管理者画面: モデレーション */
	adminHandler.RegisterAdminModerationRoutes(app.Group("/admin/moderation", middleware.AuthRequired, middleware.AdminOnly, middleware.Audit))

	/* 管理者画面: ユーザー */
	adminHandler.RegisterAdminUsersRoutes(app.Group("/admin/users", middleware.AuthRequired, middleware.AdminOnly, middleware.Audit))

	/* 外部サービスとの連携 */
	appHandler.RegisterIntegrationRoutes(app.Group("/integrations"))
	appHandler.RegisterIntegrationJobRoutes(app.Group("/admin/integration-jobs", middleware.AuthRequired, middleware.AdminOnly, middleware.Audit))

	/* ユーザー情報など */
	appHandler.RegisterUsersRoutes(app.Group("/users", middleware.AuthRequired, middleware.Audit))

	/* iCal */
	registerIcalFeedRoute(app, db)

	return app
}

func registerIcalFeedRoute(app fiber.Router, db *gorm.DB) {
	app.Get("/ical/:token.ics", func(c *fiber.Ctx) error {
		token := c.Params("token")
		var key domain.IcalKey
		if err := db.Joins("JOIN users ON users.id = ical_keys.user_id").Where("ical_keys.token = ? AND users.is_deleted = 0", token).First(&key).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(404).SendString("not found")
			}
			return fmt.Errorf("load iCal token: %w", err)
		}
		// enforce expiry if set
		if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
			return c.Status(404).SendString("not found")
		}
		ics, err := utils.BuildICSForUser(key.UserID, db)
		if err != nil {
			return fmt.Errorf("build iCal feed: %w", err)
		}
		c.Set("Content-Type", "text/calendar; charset=utf-8")
		return c.SendString(ics)
	})
}
