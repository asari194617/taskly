package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	adminH "github.com/asari194617/taskly/apps/server/internal/handler/admin"
	mw "github.com/asari194617/taskly/apps/server/internal/middleware"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/getsentry/sentry-go"
	sentryfiber "github.com/getsentry/sentry-go/fiber"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	fibertrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gofiber/fiber.v2"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	ddprofiler "gopkg.in/DataDog/dd-trace-go.v1/profiler"
)

func main() {
	// Start Datadog tracer (reads most settings from env; override agent addr explicitly if provided)
	serviceName := utils.Env("DD_SERVICE", "taskly-server")
	envName := utils.Env("DD_ENV", utils.Env("SENTRY_ENVIRONMENT", utils.Env("NODE_ENV", "development")))
	version := utils.Env("DD_VERSION", utils.Env("SENTRY_RELEASE", ""))
	agentHost := utils.Env("DD_AGENT_HOST", "127.0.0.1")
	agentPort := utils.Env("DD_TRACE_AGENT_PORT", "8126")
	// Configure Datadog tracer options (including optional sampling rate)
	tOpts := []tracer.StartOption{
		tracer.WithService(serviceName),
		tracer.WithEnv(envName),
		tracer.WithServiceVersion(version),
		tracer.WithAgentAddr(agentHost + ":" + agentPort),
		tracer.WithRuntimeMetrics(),
	}
	if sr := strings.TrimSpace(utils.Env("DD_TRACE_SAMPLE_RATE", "")); sr != "" {
		if v, err := strconv.ParseFloat(sr, 64); err == nil && v >= 0 && v <= 1 {
			tOpts = append(tOpts, tracer.WithSampler(tracer.NewRateSampler(v)))
		}
	}
	tracer.Start(tOpts...)
	defer tracer.Stop()

	// Enforce JWT_SECRET in non-development environments
	nodeEnv := strings.ToLower(utils.Env("NODE_ENV", "development"))
	jwtSecret := os.Getenv("JWT_SECRET")
	if nodeEnv != "development" && jwtSecret == "" {
		log.Fatal("FATAL: JWT_SECRET must be set in non-development environments.")
	}

	// Optional: Start Datadog profiler when enabled
	if strings.ToLower(utils.Env("DD_PROFILING_ENABLED", "false")) == "true" {
		_ = ddprofiler.Start(
			ddprofiler.WithService(serviceName),
			ddprofiler.WithEnv(envName),
			ddprofiler.WithVersion(version),
		)
		defer ddprofiler.Stop()
	}

	db := mustInitDB()       // ローカル変数にする
  rdb := mustInitRedis()
	wAuth := initWebAuthn()
	metricsServer(utils.Env("METRICS_BIND_ADDR", "127.0.0.1:9464"))

	// Run pending migrations first (includes timestamp sanitize/defaults)
	utils.RunMigrations(db)

	mw := mw.NewMiddleware(db,rdb,wAuth);
	adminH := adminH.NewAdminHandler(db, rdb, wAuth)

	// AutoMigrate is disabled per policy; rely on SQL migrations only.
	// Ensure only read-only helpers after runMigrations.

	// Sentry initialization (optional)
	if dsn := utils.Env("SENTRY_DSN", ""); dsn != "" {
		_ = sentry.Init(sentry.ClientOptions{
			Dsn:              dsn,
			TracesSampleRate: 0.1,
			Environment:      utils.Env("SENTRY_ENVIRONMENT", utils.Env("NODE_ENV", "development")),
			Release:          utils.Env("SENTRY_RELEASE", ""),
			BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
				if event == nil {
					return event
				}
				// Scrub request details
				if event.Request != nil {
					// Drop cookies and body; keep only minimal query/ip if present elsewhere
					event.Request.Cookies = ""
					// Data is a string in some sentry-go versions; use empty string
					event.Request.Data = ""
					// Mask sensitive headers
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
					// Sanitize URL path (mask tokens like /ical/{token}.ics)
					if event.Request.URL != "" {
						event.Request.URL = utils.SanitizeURLPath(event.Request.URL)
					}
					// Scrub token-like query params
					if event.Request.QueryString != "" {
						event.Request.QueryString = utils.SanitizeQueryString(event.Request.QueryString)
					}
				}
				// Scrub extra and tags
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
				// For HTTP breadcrumbs, scrub URL and headers in data
				if strings.EqualFold(crumb.Category, "http") && crumb.Data != nil {
					if urlv, ok := crumb.Data["url"].(string); ok && urlv != "" {
						crumb.Data["url"] = utils.SanitizeURLPath(urlv)
					}
				}
				if crumb.Data != nil {
					// Remove or mask sensitive fields
					for k := range crumb.Data {
						if utils.IsSensitiveKey(fmt.Sprintf("%v", k)) {
							crumb.Data[k] = "[REDACTED]"
						}
					}
				}
				return crumb
			},
		})
		// Add helpful tags and emit a startup log
		sentry.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetTag("service", "taskly-server")
			scope.SetTag("port", utils.Env("API_PORT", "8080"))
		})
		sentry.CaptureMessage("Taskly server starting")
		defer sentry.Flush(2 * time.Second)
	}

	// Centralized error handler to send all errors to Sentry with request/user context
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			// Derive status code (default 500)
			status := fiber.StatusInternalServerError
			var fe *fiber.Error
			if errors.As(err, &fe) {
				status = fe.Code
			}

			// Get Sentry hub from Fiber context (set by Sentry middleware)
			hub := sentryfiber.GetHubFromContext(c)
			if hub == nil {
				hub = sentry.CurrentHub()
			}
			if hub != nil && status >= 500 { // only report server-side errors
				hub.WithScope(func(scope *sentry.Scope) {
					// Basic request metadata
					if r := c.Route(); r != nil {
						scope.SetTag("route", r.Path)
					}
					scope.SetTag("method", c.Method())
					scope.SetTag("url", utils.SanitizeURLPath(string(c.OriginalURL())))
					scope.SetContext("request", map[string]any{
						"query": c.Queries(),
						"ip":    c.IP(),
					})
					// Attach user when available
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

			// Send sanitized error response
			if status >= 500 {
				return c.Status(status).JSON(fiber.Map{"error": "internal error"})
			}
			return c.Status(status).JSON(fiber.Map{"error": err.Error()})
		},
	})

	// Trace inbound HTTP requests with Datadog (must be registered early)
	app.Use(fibertrace.Middleware(
		fibertrace.WithServiceName(serviceName),
		fibertrace.WithSpanOptions(),
		// Ensure span resource uses route pattern (avoids leaking tokens in paths)
		fibertrace.WithResourceNamer(func(c *fiber.Ctx) string {
			if r := c.Route(); r != nil {
				return r.Method + " " + r.Path
			}
			// Fallback to method + sanitized URL
			return c.Method() + " " + utils.SanitizeURLPath(string(c.OriginalURL()))
		}),
	))

	// Capture panics and attach request context to Sentry
	app.Use(sentryfiber.New(sentryfiber.Options{
		Repanic:         true,            // let Fiber recover middleware handle the response
		WaitForDelivery: false,           // don't block requests on delivery
		Timeout:         2 * time.Second, // used only when WaitForDelivery is true
	}))

	// Keep recover after Sentry so panics are reported then gracefully recovered
	app.Use(recover.New())
	// Safe logger: log route pattern instead of concrete path to avoid leaking tokens
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${ip} ${status} ${latency} ${method} ${route}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     utils.Env("API_CORS_ORIGINS", "http://localhost:4321,http://127.0.0.1:4321,http://localhost:3000,http://127.0.0.1:3000,http://localhost:5173,http://127.0.0.1:5173"),
		AllowCredentials: strings.ToLower(utils.Env("CORS_ALLOW_CREDENTIALS", "true")) == "true",
		AllowHeaders:     "Authorization,authorization, Content-Type,content-type, X-Client-Platform,x-client-platform, X-API-Key,x-api-key",
		// Be explicit about methods to satisfy strict proxies and preflight checks
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
	}))

	// Restrict API key accessible endpoints
	app.Use(mw.ApiKeyRouteScope)

	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendString("ok") })
	// GraphQL routes
	registerGraphQL(app)

	// No in-process batch schedulers in server.
	// - Day shift rotation/notification: handled by Airflow DAGs (infra/airflow/dags/day_shift_rotations.py)
	// - Task notifications (email/WebPush): handled by Airflow DAG (infra/airflow/dags/notifications.py)
	// If an emergency fallback is ever needed, re-enable via dedicated batch apps or DAGs, not here.

	// Register extracted route groups
	registerAuthRoutes(app)
	registerPushRoutes(app)
	registerPushVAPIDRoutes(app)
	registerNotificationSettingsRoutes(app)
	registerSettingsRoutes(app)
	registerWebAuthnRoutes(app)
	registerAPIKeyRoutes(app)
	register2FARoutes(app)
	registerTasksRoutes(app)
	registerReportRoutes(app)

	adminDayShiftGroup := app.Group("/admin/dayshift", mw.AuthRequired, mw.AdminOnly, mw.Audit)
	adminH.RegisterAdminDayShiftRoutes(adminDayShiftGroup)

	adminPermissionsGroup := app.Group("/admin/permissions", mw.AuthRequired, mw.AdminOnly, mw.Audit)
	adminH.RegisterAdminPermissionsRoutes(adminPermissionsGroup)

	adminModerationGroup := app.Group("/admin/moderation", mw.AuthRequired, mw.AdminOnly, mw.Audit)
	adminH.RegisterAdminModerationRoutes(adminModerationGroup)

	adminUserGroup := app.Group("/admin/users", mw.AuthRequired, mw.AdminOnly, mw.Audit)
	adminH.RegisterAdminUsersRoutes(adminUserGroup)

	registerIntegrationRoutes(app)

	// User profile: update display name
	app.Patch("/users/me", mw.AuthRequired, mw.Audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct {
			DisplayName *string `json:"display_name"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		updates := map[string]any{}
		if body.DisplayName != nil {
			dn := strings.TrimSpace(*body.DisplayName)
			if len(dn) > 255 {
				dn = dn[:255]
			}
			updates["display_name"] = dn
		}
		if len(updates) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "no changes"})
		}
		if err := DB.Model(&domain.User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// User account delete (soft delete)
	app.Delete("/users/me", mw.AuthRequired, mw.Audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		if err := DB.Model(&domain.User{}).Where("id = ?", u.ID).Update("is_deleted", true).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// Users/me (auth required)
	app.Get("/users/me", mw.AuthRequired, mw.Audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		// roles
		var rls []domain.RoleUser
		_ = DB.Where("user_id = ?", u.ID).Find(&rls).Error
		roles := make([]string, 0, len(rls))
		isAdmin := false
		isManager := false
		for _, r := range rls {
			roles = append(roles, r.Role)
			if r.Role == "admin" {
				isAdmin = true
			}
			if r.Role == "manager" {
				isManager = true
			}
		}
		return c.JSON(fiber.Map{"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "roles": roles, "is_admin": isAdmin, "is_manager": isManager})
	})

	// Public ICS feed
	app.Get("/ical/:token.ics", func(c *fiber.Ctx) error {
		token := c.Params("token")
		var key domain.IcalKey
		if err := DB.Where("token = ?", token).First(&key).Error; err != nil {
			return c.Status(404).SendString("not found")
		}
		// enforce expiry if set
		if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
			return c.Status(404).SendString("not found")
		}
		ics := utils.BuildICSForUser(key.UserID, DB)
		c.Set("Content-Type", "text/calendar; charset=utf-8")
		return c.SendString(ics)
	})

	port := utils.Env("API_PORT", "8080")
	log.Printf("Application Server listening on :%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}
