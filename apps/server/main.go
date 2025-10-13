package main

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// Globals moved to globals.go

// env moved to support.go

// ptrTime moved to support.go

func main() {
	DB = mustInitDB()
	RDB = mustInitRedis()
	metricsServer(env("METRICS_BIND_ADDR", "127.0.0.1:9464"))

	// Init WebAuthn (Passkey)
	initWebAuthn()

	// Auto-migrate core models
	_ = DB.AutoMigrate(&User{}, &Session{}, &RoleUser{}, &Permission{}, &Block{}, &APIKey{}, &Subject{}, &Task{}, &ApiActionLog{}, &TOTPSecret{}, &RecoveryCode{}, &Passkey{}, &LoginCode{}, &DayShiftCycle{}, &DayShiftAssignment{}, &DayShiftAssignmentUser{}, &BusinessCalendar{}, &IcalKey{}, &DayShiftCyclePair{}, &MCP{}, &Log{}, &WebPushSubscription{}, &WebPushVAPID{})

	app := fiber.New()
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     env("API_CORS_ORIGINS", "http://localhost:4321"),
		AllowCredentials: strings.ToLower(env("CORS_ALLOW_CREDENTIALS", "true")) == "true",
		AllowHeaders:     "Authorization, Content-Type",
	}))

	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendString("ok") })
	// GraphQL routes
	registerGraphQL(app)

	// Day shift scheduler (08:30 local) – minimal implementation
	if strings.ToLower(env("DAYSHIFT_ENABLED", "true")) == "true" {
		notifyTime := env("DAYSHIFT_NOTIFY_TIME", "08:30")
		go dayShiftScheduler(notifyTime)
	}

	// Daily tasks email digest (07:30 local by default)
	if strings.ToLower(env("TASK_DIGEST_ENABLED", "true")) == "true" {
		go tasksNotifyScheduler(env("TASK_DIGEST_TIME", "07:30"))
	}
	// Daily tasks push digest (07:45 default)
	if strings.ToLower(env("PUSH_DIGEST_ENABLED", "false")) == "true" {
		go pushNotifyScheduler(env("PUSH_DIGEST_TIME", "07:45"))
	}

	// Register extracted route groups
	registerAuthRoutes(app)
	registerPushRoutes(app)
	registerPushVAPIDRoutes(app)
	registerSettingsRoutes(app)
	registerWebAuthnRoutes(app)
	registerAPIKeyRoutes(app)
	register2FARoutes(app)
	registerTasksRoutes(app)
	registerAdminDayShiftRoutes(app)
	registerAdminPermissionsRoutes(app)
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
