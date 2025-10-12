package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	DB  *gorm.DB
	RDB *redis.Client
)

type User struct {
	ID               uint    `gorm:"primaryKey"`
	Email            string  `gorm:"uniqueIndex;size:255;not null"`
	DisplayName      string  `gorm:"size:255"`
	HashedPassword   *string `gorm:"size:255"`
	IsDeleted        bool    `gorm:"default:false"`
	IsDayShiftActive bool    `gorm:"default:false"`
	CreatedAt        time.Time
}

type Session struct {
	ID             uint   `gorm:"primaryKey"`
	ClientPlatform string `gorm:"size:64;not null"`
	HashedToken    string `gorm:"size:255;not null"`
	ExpiresAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// RBAC and permissions
type RoleUser struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"index;not null"`
	Role      string `gorm:"size:16;not null"` // admin | manager
	CreatedAt time.Time
}

type Permission struct {
	ID        uint    `gorm:"primaryKey"`
	UserID    uint    `gorm:"index;not null"`
	Target    string  `gorm:"size:64;not null"`
	Method    string  `gorm:"size:16;not null"`
	Value     *string `gorm:"size:255"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Block struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint `gorm:"index;not null"`
	Reason    *string
	ExpiresAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type APIKey struct {
	ID          uint    `gorm:"primaryKey"`
	UserID      uint    `gorm:"index;not null"`
	HashedToken string  `gorm:"size:255;not null"`
	Description *string `gorm:"size:255"`
	IsValid     bool    `gorm:"default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustInitDB() *gorm.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4,utf8",
		env("DB_USER", "taskly"),
		env("DB_PASSWORD", "taskly_password"),
		env("DB_HOST", "127.0.0.1"),
		env("DB_PORT", "3306"),
		env("DB_NAME", "taskly"))
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	return db
}

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

func metricsServer(bind string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	ln, err := net.Listen("tcp", bind)
	if err != nil {
		log.Printf("metrics listen err: %v", err)
		return
	}
	go func() {
		_ = http.Serve(ln, mux)
	}()
}

func ptrTime(t time.Time) *time.Time { return &t }

func main() {
	DB = mustInitDB()
	RDB = mustInitRedis()
	metricsServer(env("METRICS_BIND_ADDR", "127.0.0.1:9464"))

	app := fiber.New()
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     env("API_CORS_ORIGINS", "http://localhost:4321"),
		AllowCredentials: strings.ToLower(env("CORS_ALLOW_CREDENTIALS", "true")) == "true",
		AllowHeaders:     "Authorization, Content-Type",
	}))

	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendString("ok") })

	// Auth endpoints (minimal)
	app.Post("/auth/request-code", func(c *fiber.Ctx) error {
		var body struct {
			Email string `json:"email"`
		}
		if err := c.BodyParser(&body); err != nil || body.Email == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		// TODO: send code by email via SMTP (Postfix)
		return c.SendStatus(fiber.StatusAccepted)
	})

	app.Post("/auth/login", func(c *fiber.Ctx) error {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Code     string `json:"code"`
		}
		if err := c.BodyParser(&body); err != nil || body.Email == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		// Lookup user or create placeholder
		var u User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				u = User{Email: body.Email, DisplayName: body.Email}
				if err := DB.Create(&u).Error; err != nil {
					return c.Status(500).JSON(fiber.Map{"error": "db"})
				}
			} else {
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
		}
		// Blocklist enforcement
		var cnt int64
		if err := DB.Model(&Block{}).
			Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", u.ID, time.Now()).
			Count(&cnt).Error; err == nil && cnt > 0 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "blocked"})
		}
		// TODO: verify password/code/2FA; check blocklist

		// Create session token (opaque) and store in Redis
		token := fmt.Sprintf("sess_%d_%d", u.ID, time.Now().UnixNano())
		ttl := time.Hour * 24 * 30
		if err := RDB.Set(c.Context(), "session:"+token, fmt.Sprintf("%d", u.ID), ttl).Err(); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "session"})
		}
		// Optionally record DB session (hashed)
		DB.AutoMigrate(&Session{})
		// For brevity, store plain (not recommended) — replace with hash in production
		DB.Create(&Session{ClientPlatform: "web", HashedToken: token, ExpiresAt: ptrTime(time.Now().Add(ttl))})

		// Also provide a JWT if needed (optional)
		claims := jwt.MapClaims{"sub": u.ID, "exp": time.Now().Add(ttl).Unix()}
		jwtt := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		jwtStr, _ := jwtt.SignedString([]byte(env("JWT_SECRET", "dev")))

		return c.JSON(fiber.Map{"token": token, "jwt": jwtStr})
	})

	app.Post("/auth/logout", func(c *fiber.Ctx) error {
		token := c.Get("Authorization")
		if token != "" {
			_ = RDB.Del(c.Context(), "session:"+token).Err()
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	// Auth-required group
	authRequired := func(c *fiber.Ctx) error {
		token := c.Get("Authorization")
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing token"})
		}
		uidStr, err := RDB.Get(c.Context(), "session:"+token).Result()
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}
		var u User
		if err := DB.First(&u, uidStr).Error; err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found"})
		}
		// enforce block on all authed calls
		var cnt int64
		if err := DB.Model(&Block{}).
			Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", u.ID, time.Now()).
			Count(&cnt).Error; err == nil && cnt > 0 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "blocked"})
		}
		c.Locals("user", &u)
		c.Locals("token", token)
		return c.Next()
	}

	// API keys endpoints
	app.Get("/apikeys", authRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var keys []APIKey
		if err := DB.Where("user_id = ?", u.ID).Order("id DESC").Find(&keys).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		// Do not expose hashed tokens
		type resp struct {
			ID          uint      `json:"id"`
			Description *string   `json:"description"`
			IsValid     bool      `json:"is_valid"`
			CreatedAt   time.Time `json:"created_at"`
		}
		out := make([]resp, 0, len(keys))
		for _, k := range keys {
			out = append(out, resp{ID: k.ID, Description: k.Description, IsValid: k.IsValid, CreatedAt: k.CreatedAt})
		}
		return c.JSON(out)
	})

	app.Post("/apikeys", authRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		limit, err := apiKeyLimitForUser(u.ID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		if limit >= 0 { // -1 means unlimited
			var cur int64
			if err := DB.Model(&APIKey{}).Where("user_id = ? AND is_valid = 1", u.ID).Count(&cur).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
			if int(cur) >= limit {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "api key limit reached"})
			}
		}
		var body struct {
			Description *string `json:"description"`
		}
		_ = c.BodyParser(&body)
		plain, hashed, genErr := generateAPIKey()
		if genErr != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gen"})
		}
		rec := APIKey{UserID: u.ID, HashedToken: hashed, Description: body.Description}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": rec.ID, "token": plain})
	})

	app.Patch("/apikeys/:id", authRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var rec APIKey
		if err := DB.Where("id = ? AND user_id = ?", c.Params("id"), u.ID).First(&rec).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		var body struct {
			IsValid     *bool   `json:"is_valid"`
			Description *string `json:"description"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		if body.IsValid != nil {
			rec.IsValid = *body.IsValid
		}
		if body.Description != nil {
			rec.Description = body.Description
		}
		if err := DB.Save(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(fiber.Map{"id": rec.ID, "is_valid": rec.IsValid})
	})

	// minimal routes to unblock scaffolding
	app.Get("/users/me", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"id": 1, "email": "dev@example.com", "display_name": "Dev"})
	})

	app.Get("/tasks", func(c *fiber.Ctx) error {
		return c.JSON([]fiber.Map{})
	})

	app.Post("/integrations/teams/status", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	app.Get("/ical/:token.ics", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/calendar")
		return c.SendString("BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Taskly//EN\nEND:VCALENDAR\n")
	})

	port := env("API_PORT", "8080")
	log.Printf("API listening on :%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}

// helpers
func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])
}

func generateAPIKey() (plain string, hashed string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	plain = fmt.Sprintf("tsk_%x", buf)
	hashed = sha256Hex([]byte(plain))
	return
}

func userHasRole(userID uint, role string) (bool, error) {
	var cnt int64
	if err := DB.Model(&RoleUser{}).Where("user_id = ? AND role = ?", userID, role).Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func apiKeyLimitForUser(userID uint) (int, error) {
	// admin unlimited
	if ok, err := userHasRole(userID, "admin"); err != nil {
		return 0, err
	} else if ok {
		return -1, nil
	}
	// manager 100
	if ok, err := userHasRole(userID, "manager"); err != nil {
		return 0, err
	} else if ok {
		return 100, nil
	}
	// override via permissions target=api_key_limit
	var p Permission
	if err := DB.Where("user_id = ? AND target = ?", userID, "api_key_limit").Order("id DESC").First(&p).Error; err == nil && p.Value != nil {
		var v int
		fmt.Sscanf(*p.Value, "%d", &v)
		if v >= 0 {
			return v, nil
		}
	}
	// default user 10
	return 10, nil
}
