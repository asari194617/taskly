package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

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

// Globals moved to globals.go

// env moved to support.go

// ptrTime moved to support.go

func main() {
	// Start Datadog tracer (reads most settings from env; override agent addr explicitly if provided)
	serviceName := env("DD_SERVICE", "taskly-server")
	envName := env("DD_ENV", env("SENTRY_ENVIRONMENT", env("NODE_ENV", "development")))
	version := env("DD_VERSION", env("SENTRY_RELEASE", ""))
	agentHost := env("DD_AGENT_HOST", "127.0.0.1")
	agentPort := env("DD_TRACE_AGENT_PORT", "8126")
	// Configure Datadog tracer options (including optional sampling rate)
	tOpts := []tracer.StartOption{
		tracer.WithService(serviceName),
		tracer.WithEnv(envName),
		tracer.WithServiceVersion(version),
		tracer.WithAgentAddr(agentHost + ":" + agentPort),
		tracer.WithRuntimeMetrics(),
	}
	if sr := strings.TrimSpace(env("DD_TRACE_SAMPLE_RATE", "")); sr != "" {
		if v, err := strconv.ParseFloat(sr, 64); err == nil && v >= 0 && v <= 1 {
			tOpts = append(tOpts, tracer.WithSampler(tracer.NewRateSampler(v)))
		}
	}
	tracer.Start(tOpts...)
	defer tracer.Stop()

	// Enforce JWT_SECRET in non-development environments
	nodeEnv := strings.ToLower(env("NODE_ENV", "development"))
	jwtSecret := os.Getenv("JWT_SECRET")
	if nodeEnv != "development" && jwtSecret == "" {
		log.Fatal("FATAL: JWT_SECRET must be set in non-development environments.")
	}

	// Optional: Start Datadog profiler when enabled
	if strings.ToLower(env("DD_PROFILING_ENABLED", "false")) == "true" {
		_ = ddprofiler.Start(
			ddprofiler.WithService(serviceName),
			ddprofiler.WithEnv(envName),
			ddprofiler.WithVersion(version),
		)
		defer ddprofiler.Stop()
	}

	DB = mustInitDB()
	RDB = mustInitRedis()
	metricsServer(env("METRICS_BIND_ADDR", "127.0.0.1:9464"))

	// Init WebAuthn (Passkey)
	initWebAuthn()

	// Run pending migrations first (includes timestamp sanitize/defaults)
	runMigrations()

	// Auto-migrate core models (create missing tables/columns after migrations)
	_ = DB.AutoMigrate(&Session{}, &RoleUser{}, &Permission{}, &Block{}, &APIKey{}, &Subject{}, &Task{}, &ApiActionLog{}, &TOTPSecret{}, &RecoveryCode{}, &Passkey{}, &LoginCode{}, &DayShiftCycle{}, &DayShiftAssignment{}, &DayShiftAssignmentUser{}, &BusinessCalendar{}, &IcalKey{}, &DayShiftCyclePair{}, &MCP{}, &Log{}, &WebPushSubscription{}, &WebPushVAPID{}, &Migration{}, &FlaggedContent{}, &UserReport{}, &SystemSetting{}, &NotificationTypePref{}, &NotificationDevice{})
	// Ensure new group-based day shift schema exists even if SQL migrations are not applied
	ensureDayShiftGroupSchemaAndBackfill()
	// Ensure legacy tasks.deadline column is DATE (not DATETIME)
	ensureDeadlineDateColumn()
	// If tasks is empty and schema mismatches new enums/date, drop and recreate safely
	ensureTasksSchema()
	// Ensure integrations logging table exists even if SQL migrations aren't run externally
	ensureTasksTeamsLogsTable()
	// Ensure per-user task completion table exists
	ensureTasksCompletionTable()

	// Sentry initialization (optional)
	if dsn := env("SENTRY_DSN", ""); dsn != "" {
		_ = sentry.Init(sentry.ClientOptions{
			Dsn:              dsn,
			TracesSampleRate: 0.1,
			Environment:      env("SENTRY_ENVIRONMENT", env("NODE_ENV", "development")),
			Release:          env("SENTRY_RELEASE", ""),
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
						event.Request.URL = sanitizeURLPath(event.Request.URL)
					}
					// Scrub token-like query params
					if event.Request.QueryString != "" {
						event.Request.QueryString = sanitizeQueryString(event.Request.QueryString)
					}
				}
				// Scrub extra and tags
				if event.Extra != nil {
					scrubPIIMap(event.Extra)
				}
				if event.Tags != nil {
					for k, v := range event.Tags {
						if isSensitiveKey(k) || isSensitiveValue(v) {
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
						crumb.Data["url"] = sanitizeURLPath(urlv)
					}
				}
				if crumb.Data != nil {
					// Remove or mask sensitive fields
					for k := range crumb.Data {
						if isSensitiveKey(fmt.Sprintf("%v", k)) {
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
			scope.SetTag("port", env("API_PORT", "8080"))
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
					scope.SetTag("url", sanitizeURLPath(string(c.OriginalURL())))
					scope.SetContext("request", map[string]any{
						"query": c.Queries(),
						"ip":    c.IP(),
					})
					// Attach user when available
					if u, ok := c.Locals("user").(*User); ok && u != nil {
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
			return c.Method() + " " + sanitizeURLPath(string(c.OriginalURL()))
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
		AllowOrigins:     env("API_CORS_ORIGINS", "http://localhost:4321,http://127.0.0.1:4321,http://localhost:3000,http://127.0.0.1:3000,http://localhost:5173,http://127.0.0.1:5173"),
		AllowCredentials: strings.ToLower(env("CORS_ALLOW_CREDENTIALS", "true")) == "true",
		AllowHeaders:     "Authorization,authorization, Content-Type,content-type, X-Client-Platform,x-client-platform, X-API-Key,x-api-key",
		// Be explicit about methods to satisfy strict proxies and preflight checks
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
	}))

	// Restrict API key accessible endpoints
	app.Use(apiKeyRouteScope)

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
	registerSettingsRoutes(app)
	registerWebAuthnRoutes(app)
	registerAPIKeyRoutes(app)
	register2FARoutes(app)
	registerTasksRoutes(app)
	registerReportRoutes(app)
	registerAdminDayShiftRoutes(app)
	registerAdminPermissionsRoutes(app)
	registerAdminModerationRoutes(app)
	registerAdminUsersRoutes(app)
	registerIntegrationRoutes(app)

	// User profile: update display name
	app.Patch("/users/me", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
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
		if err := DB.Model(&User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// User account delete (soft delete)
	app.Delete("/users/me", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		if err := DB.Model(&User{}).Where("id = ?", u.ID).Update("is_deleted", true).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// Users/me (auth required)
	app.Get("/users/me", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		// roles
		var rls []RoleUser
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
		var key IcalKey
		if err := DB.Where("token = ?", token).First(&key).Error; err != nil {
			return c.Status(404).SendString("not found")
		}
		// enforce expiry if set
		if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
			return c.Status(404).SendString("not found")
		}
		ics := buildICSForUser(key.UserID)
		c.Set("Content-Type", "text/calendar; charset=utf-8")
		return c.SendString(ics)
	})

	port := env("API_PORT", "8080")
	log.Printf("Application Server listening on :%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}
