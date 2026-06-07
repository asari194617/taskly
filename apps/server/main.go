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
	"github.com/asari194617/taskly/apps/server/internal/handler"
	adminH "github.com/asari194617/taskly/apps/server/internal/handler/admin"
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql"
	mw "github.com/asari194617/taskly/apps/server/internal/middleware"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/getsentry/sentry-go"
	sentryFiber "github.com/getsentry/sentry-go/fiber"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	fiberTrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gofiber/fiber.v2"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	ddProfiler "gopkg.in/DataDog/dd-trace-go.v1/profiler"
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

	// Sentry initialization
	if dsn := os.Getenv("SENTRY_DSN"); dsn != "" {
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
					if urlV, ok := crumb.Data["url"].(string); ok && urlV != "" {
						crumb.Data["url"] = utils.SanitizeURLPath(urlV)
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

	// Enforce JWT_SECRET in non-development environments
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("FATAL: JWT_SECRET must be set in non-development environments.")
	}

	// Optional: Start Datadog profiler when enabled
	if strings.ToLower(utils.Env("DD_PROFILING_ENABLED", "false")) == "true" {
		_ = ddProfiler.Start(
			ddProfiler.WithService(serviceName),
			ddProfiler.WithEnv(envName),
			ddProfiler.WithVersion(version),
		)
		defer ddProfiler.Stop()
	}

	db := mustInitDB() // ローカル変数にする
	rdb := mustInitRedis()
	wAuth := initWebAuthn()
	metricsServer(utils.Env("METRICS_BIND_ADDR", "127.0.0.1:9464"))

	mw := mw.NewMiddleware(db, rdb, wAuth)
	adminH := adminH.NewAdminHandler(db, rdb, wAuth)
	handler := handler.NewHandler(db, rdb, wAuth)
	graphql := gqlH.NewHandler(db, rdb, wAuth)

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
			hub := sentryFiber.GetHubFromContext(c)
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
	app.Use(fiberTrace.Middleware(
		fiberTrace.WithServiceName(serviceName),
		fiberTrace.WithSpanOptions(),
		// Ensure span resource uses route pattern (avoids leaking tokens in paths)
		fiberTrace.WithResourceNamer(func(c *fiber.Ctx) string {
			if r := c.Route(); r != nil {
				return r.Method + " " + r.Path
			}
			// Fallback to method + sanitized URL
			return c.Method() + " " + utils.SanitizeURLPath(string(c.OriginalURL()))
		}),
	))

	// Capture panics and attach request context to Sentry
	app.Use(sentryFiber.New(sentryFiber.Options{
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
	graphql.RegisterGraphQL(
		app.Group("/graphql"),
	)

	// Register extracted route groups
	handler.Register2FARoutes(
		app.Group("/2fa"), mw,
	)
	handler.RegisterAuthRoutes(
		app.Group("/auth"),
	)

	handler.RegisterPushRoutes(
		app.Group("/push"), mw,
	)

	handler.RegisterIcalTokensRoutes(
		app.Group("/ical/tokens", mw.AuthRequired, mw.Audit),
	)

	handler.RegisterSettingsRoutes(
		app.Group("/settings", mw.AuthRequired, mw.Audit),
	)

	handler.RegisterWebAuthnRoutes(
		app.Group("/webauthn"), mw,
	)

	handler.RegisterAPIKeyRoutes(
		app.Group("/apikeys", mw.AuthRequired, mw.Audit),
	)

	handler.RegisterTasksRoutes(
		app.Group("/tasks", mw.AuthRequired, mw.Audit),
	)

	handler.RegisterReportRoutes(
		app.Group("reports", mw.AuthRequired, mw.Audit),
	)

	adminH.RegisterAdminDayShiftRoutes(
		app.Group("/admin/dayshift", mw.AuthRequired, mw.AdminOnly, mw.Audit),
	)

	adminH.RegisterAdminPermissionsRoutes(
		app.Group("/admin/permissions", mw.AuthRequired, mw.AdminOnly, mw.Audit),
	)

	adminH.RegisterAdminModerationRoutes(
		app.Group("/admin/moderation", mw.AuthRequired, mw.AdminOnly, mw.Audit),
	)

	adminH.RegisterAdminUsersRoutes(
		app.Group("/admin/users", mw.AuthRequired, mw.AdminOnly, mw.Audit),
	)

	handler.RegisterIntegrationRoutes(
		app.Group("/integrations"),
	)

	handler.RegisterUsersRoutes(
		app.Group("/users", mw.AuthRequired, mw.Audit),
	)

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
