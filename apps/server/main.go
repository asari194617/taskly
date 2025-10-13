package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/golang-jwt/jwt/v5"
	graphql "github.com/graphql-go/graphql"
	"github.com/pquerna/otp/totp"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	DB    *gorm.DB
	RDB   *redis.Client
	WAuth *webauthn.WebAuthn
)

type User struct {
	ID               uint    `gorm:"primaryKey"`
	Email            string  `gorm:"uniqueIndex;size:255;not null"`
	DisplayName      string  `gorm:"size:255"`
	HashedPassword   *string `gorm:"size:255"`
	EmailCodeAllowed bool    `gorm:"default:true"`
	IsDeleted        bool    `gorm:"default:false"`
	IsDayShiftActive bool    `gorm:"default:false"`
	CreatedAt        time.Time
}

// WebAuthn user interface implementation
func (u *User) WebAuthnID() []byte {
	return []byte(fmt.Sprintf("%d", u.ID))
}
func (u *User) WebAuthnName() string {
	if u.Email != "" {
		return u.Email
	}
	return fmt.Sprintf("user-%d", u.ID)
}
func (u *User) WebAuthnDisplayName() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.Email != "" {
		return u.Email
	}
	return fmt.Sprintf("user-%d", u.ID)
}
func (u *User) WebAuthnIcon() string { return "" }
func (u *User) WebAuthnCredentials() []webauthn.Credential {
	var pks []Passkey
	_ = DB.Where("user_id = ?", u.ID).Find(&pks).Error
	res := make([]webauthn.Credential, 0, len(pks))
	for _, p := range pks {
		cid, _ := base64.RawStdEncoding.DecodeString(p.CredentialID)
		pub, _ := base64.RawStdEncoding.DecodeString(p.PublicKey)
		res = append(res, webauthn.Credential{ID: cid, PublicKey: pub, Authenticator: webauthn.Authenticator{SignCount: uint32(p.SignCount)}})
	}
	return res
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

type Subject struct {
	ID        uint   `gorm:"primaryKey"`
	Title     string `gorm:"size:255;not null"`
	ExpiresAt *time.Time
	CreatedAt time.Time
}

type Task struct {
	ID              uint    `gorm:"primaryKey"`
	AddedUserID     uint    `gorm:"index;not null"`
	Title           string  `gorm:"size:255;not null"`
	SubjectID       uint    `gorm:"index;not null"`
	Description     *string `gorm:"type:text"`
	IsRequireHandIn bool    `gorm:"default:false"`
	Deadline        *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ApiActionLog struct {
	ID          uint   `gorm:"primaryKey"`
	UserID      *uint  `gorm:"index"`
	HashedToken string `gorm:"size:255;not null"`
	Action      string `gorm:"size:64;not null"`
	IP          string `gorm:"size:64;not null"`
	CreatedAt   time.Time
}

// Generic system/user logs per specification
type Log struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    *uint     `gorm:"index"`
	Action    string    `gorm:"size:64;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

type TOTPSecret struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"uniqueIndex;not null"`
	Secret    string `gorm:"size:255;not null"` // encrypted base32
	IsActive  bool   `gorm:"default:false"`
	CreatedAt time.Time
}

type RecoveryCode struct {
	ID         uint   `gorm:"primaryKey"`
	UserID     uint   `gorm:"index;not null"`
	HashedCode string `gorm:"size:255;not null"`
	IsUsed     bool   `gorm:"default:false"`
	CreatedAt  time.Time
}

// Login codes (email magic code)
type LoginCode struct {
	ID         uint      `gorm:"primaryKey"`
	Email      string    `gorm:"index;size:255;not null"`
	HashedCode string    `gorm:"size:255;not null"`
	ExpiresAt  time.Time `gorm:"index;not null"`
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

// Passkeys (WebAuthn) per specification
type Passkey struct {
	ID           uint      `gorm:"primaryKey"`
	UserID       uint      `gorm:"index;not null"`
	CredentialID string    `gorm:"size:255;uniqueIndex;not null"`
	PublicKey    string    `gorm:"type:text;not null"`
	SignCount    int64     `gorm:"not null"`
	DeviceName   *string   `gorm:"size:255"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

// Day shift models
type DayShiftCycle struct {
	ID          uint `gorm:"primaryKey"`
	BeginDate   time.Time
	FinishDate  time.Time
	Description string `gorm:"size:255"`
}
type DayShiftAssignment struct {
	ID        uint `gorm:"primaryKey"`
	Date      time.Time
	CycleID   uint
	Status    string `gorm:"size:16"`
	Notes     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}
type DayShiftAssignmentUser struct {
	ID                    uint `gorm:"primaryKey"`
	DayShiftAssignmentsID uint
	UserID                uint
}
type BusinessCalendar struct {
	ID            uint `gorm:"primaryKey"`
	Date          time.Time
	IsBusinessDay bool
	Notes         *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type DayShiftCyclePair struct {
	ID        uint `gorm:"primaryKey"`
	CycleID   uint `gorm:"index;not null"`
	User1ID   uint `gorm:"not null"`
	User2ID   uint `gorm:"not null"`
	Position  int  `gorm:"default:0"`
	CreatedAt time.Time
}

type IcalKey struct {
	ID          uint    `gorm:"primaryKey"`
	UserID      uint    `gorm:"index;not null"`
	Token       string  `gorm:"uniqueIndex;size:255;not null"`
	Description *string `gorm:"size:255"`
	CreatedAt   time.Time
}

// MCP integration tokens per user (for MCP server access via user session)
type MCP struct {
	ID          uint   `gorm:"primaryKey"`
	UserID      uint   `gorm:"index"`
	HashedToken string `gorm:"size:255"`
	Description *string
	CreatedAt   time.Time
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

	// Init WebAuthn (Passkey)
	if wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: env("WEBAUTHN_RP_NAME", "Taskly"),
		RPID:          env("WEBAUTHN_RP_ID", "localhost"),
		RPOrigin:      env("WEBAUTHN_ORIGIN", "http://localhost:4321"),
	}); err != nil {
		log.Printf("webauthn init error: %v", err)
	} else {
		WAuth = wa
	}

	// Auto-migrate core models
	_ = DB.AutoMigrate(&User{}, &Session{}, &RoleUser{}, &Permission{}, &Block{}, &APIKey{}, &Subject{}, &Task{}, &ApiActionLog{}, &TOTPSecret{}, &RecoveryCode{}, &Passkey{}, &LoginCode{}, &DayShiftCycle{}, &DayShiftAssignment{}, &DayShiftAssignmentUser{}, &BusinessCalendar{}, &IcalKey{}, &DayShiftCyclePair{}, &MCP{}, &Log{})

	app := fiber.New()
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     env("API_CORS_ORIGINS", "http://localhost:4321"),
		AllowCredentials: strings.ToLower(env("CORS_ALLOW_CREDENTIALS", "true")) == "true",
		AllowHeaders:     "Authorization, Content-Type",
	}))

	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendString("ok") })

	// GraphQL endpoint for application <-> application server traffic
	// Keep REST for API key based integrations. Minimal initial schema provided.
	// Reusable Task GraphQL type
	subjectType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Subject",
		Fields: graphql.Fields{
			"id":    &graphql.Field{Type: graphql.Int},
			"title": &graphql.Field{Type: graphql.String},
		},
	})
	taskType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Task",
		Fields: graphql.Fields{
			"id":                 &graphql.Field{Type: graphql.Int},
			"title":              &graphql.Field{Type: graphql.String},
			"subject_id":         &graphql.Field{Type: graphql.Int},
			"description":        &graphql.Field{Type: graphql.String},
			"is_require_hand_in": &graphql.Field{Type: graphql.Boolean},
			"deadline":           &graphql.Field{Type: graphql.String},
			"created_at":         &graphql.Field{Type: graphql.String},
			"updated_at":         &graphql.Field{Type: graphql.String},
		},
	})
	// Me and AuthSettings types
	meType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Me",
		Fields: graphql.Fields{
			"id":           &graphql.Field{Type: graphql.Int},
			"email":        &graphql.Field{Type: graphql.String},
			"display_name": &graphql.Field{Type: graphql.String},
			"is_admin":     &graphql.Field{Type: graphql.Boolean},
			"is_manager":   &graphql.Field{Type: graphql.Boolean},
		},
	})
	authSettingsType := graphql.NewObject(graphql.ObjectConfig{
		Name: "AuthSettings",
		Fields: graphql.Fields{
			"email_login_enabled": &graphql.Field{Type: graphql.Boolean},
			"has_password":        &graphql.Field{Type: graphql.Boolean},
			"totp_enabled":        &graphql.Field{Type: graphql.Boolean},
		},
	})
	rootQuery := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"myLogs": &graphql.Field{
				Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
					Name: "Log",
					Fields: graphql.Fields{
						"action":     &graphql.Field{Type: graphql.String},
						"created_at": &graphql.Field{Type: graphql.String},
					},
				})),
				Args: graphql.FieldConfigArgument{
					"prefix": &graphql.ArgumentConfig{Type: graphql.String},
					"limit":  &graphql.ArgumentConfig{Type: graphql.Int},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						limit := 100
						if v, ok := p.Args["limit"].(int); ok && v > 0 && v <= 500 {
							limit = v
						}
						q := DB.Model(&Log{}).Where("user_id = ?", u.ID).Order("id DESC").Limit(limit)
						if pf, ok := p.Args["prefix"].(string); ok && strings.TrimSpace(pf) != "" {
							q = q.Where("action LIKE ?", strings.TrimSpace(pf)+"%")
						}
						var items []Log
						if err := q.Find(&items).Error; err != nil {
							return nil, err
						}
						out := make([]map[string]any, 0, len(items))
						for _, it := range items {
							out = append(out, map[string]any{"action": it.Action, "created_at": it.CreatedAt.Format(time.RFC3339)})
						}
						return out, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"mcpInfo": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "McpInfo",
					Fields: graphql.Fields{
						"enabled":   &graphql.Field{Type: graphql.Boolean},
						"has_token": &graphql.Field{Type: graphql.Boolean},
					},
				}),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						var cnt int64
						_ = DB.Model(&MCP{}).Where("user_id = ?", u.ID).Count(&cnt).Error
						enabled := strings.ToLower(env("MCP_ENABLED", "true")) == "true"
						return map[string]any{"enabled": enabled, "has_token": cnt > 0}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"apiKeys": &graphql.Field{
				Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
					Name: "ApiKey",
					Fields: graphql.Fields{
						"id":          &graphql.Field{Type: graphql.Int},
						"description": &graphql.Field{Type: graphql.String},
						"is_valid":    &graphql.Field{Type: graphql.Boolean},
						"created_at":  &graphql.Field{Type: graphql.String},
					},
				})),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						var keys []APIKey
						if err := DB.Where("user_id = ?", u.ID).Order("id DESC").Find(&keys).Error; err != nil {
							return nil, err
						}
						out := make([]map[string]any, 0, len(keys))
						for _, k := range keys {
							out = append(out, map[string]any{"id": k.ID, "description": k.Description, "is_valid": k.IsValid, "created_at": k.CreatedAt.Format(time.RFC3339)})
						}
						return out, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"icalTokens": &graphql.Field{
				Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
					Name: "IcalToken",
					Fields: graphql.Fields{
						"id":          &graphql.Field{Type: graphql.Int},
						"description": &graphql.Field{Type: graphql.String},
						"url":         &graphql.Field{Type: graphql.String},
						"created_at":  &graphql.Field{Type: graphql.String},
					},
				})),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						var items []IcalKey
						if err := DB.Where("user_id = ?", u.ID).Order("id DESC").Find(&items).Error; err != nil {
							return nil, err
						}
						base := strings.TrimRight(env("ICAL_BASE_URL", env("API_PUBLIC_URL", "http://localhost:8080")+"/ical"), "/")
						out := make([]map[string]any, 0, len(items))
						for _, it := range items {
							out = append(out, map[string]any{"id": it.ID, "description": it.Description, "url": fmt.Sprintf("%s/%s.ics", base, it.Token), "created_at": it.CreatedAt.Format(time.RFC3339)})
						}
						return out, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"subjects": &graphql.Field{
				Type: graphql.NewList(subjectType),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) { // GraphQLはセッション専用
							return nil, fmt.Errorf("unauthorized")
						}
						var subs []Subject
						now := time.Now()
						if err := DB.Where("expires_at IS NULL OR expires_at > ?", now).Order("id ASC").Find(&subs).Error; err != nil {
							return nil, err
						}
						out := make([]map[string]any, 0, len(subs))
						for _, s := range subs {
							out = append(out, map[string]any{"id": s.ID, "title": s.Title})
						}
						return out, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"me": &graphql.Field{
				Type: meType,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					ctx := p.Context
					// Fiber context passed via Context
					if c, ok := ctx.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) { // GraphQLはセッション専用
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						var roles []RoleUser
						_ = DB.Where("user_id = ?", u.ID).Find(&roles).Error
						isAdmin := false
						isManager := false
						for _, r := range roles {
							if r.Role == "admin" {
								isAdmin = true
							}
							if r.Role == "manager" {
								isManager = true
							}
						}
						return map[string]any{
							"id":           u.ID,
							"email":        u.Email,
							"display_name": u.DisplayName,
							"is_admin":     isAdmin,
							"is_manager":   isManager,
						}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"authSettings": &graphql.Field{
				Type: authSettingsType,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) { // GraphQLはセッション専用
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						var fresh User
						if err := DB.First(&fresh, u.ID).Error; err != nil {
							return nil, err
						}
						hasPassword := fresh.HashedPassword != nil && *fresh.HashedPassword != ""
						totpEnabled, _, _ := getActiveTOTP(fresh.ID)
						return map[string]any{
							"email_login_enabled": fresh.EmailCodeAllowed,
							"has_password":        hasPassword,
							"totp_enabled":        totpEnabled,
						}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"tasks": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "TasksResult",
					Fields: graphql.Fields{
						"items": &graphql.Field{Type: graphql.NewList(taskType)},
						"next_cursor": &graphql.Field{Type: graphql.NewObject(graphql.ObjectConfig{
							Name: "TasksCursor",
							Fields: graphql.Fields{
								"cursor_deadline": &graphql.Field{Type: graphql.String},
								"cursor_id":       &graphql.Field{Type: graphql.String},
							},
						})},
					},
				}),
				Args: graphql.FieldConfigArgument{
					"limit":           &graphql.ArgumentConfig{Type: graphql.Int},
					"overdue":         &graphql.ArgumentConfig{Type: graphql.Boolean},
					"q":               &graphql.ArgumentConfig{Type: graphql.String},
					"cursor_deadline": &graphql.ArgumentConfig{Type: graphql.String},
					"cursor_id":       &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) && !userFromAPIKey(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						// permission read tasks (default allow)
						if okp, err := checkPermission(u.ID, "tasks", "read", nil); err != nil {
							return nil, err
						} else if !okp {
							return nil, fmt.Errorf("forbidden")
						}
						overdue, _ := p.Args["overdue"].(bool)
						qstr, _ := p.Args["q"].(string)
						limit := 20
						if v, ok := p.Args["limit"].(int); ok && v > 0 && v <= 100 {
							limit = v
						}
						var cursorDeadline *time.Time
						if s, _ := p.Args["cursor_deadline"].(string); s != "" {
							if t, err := time.Parse(time.RFC3339, s); err == nil {
								cursorDeadline = &t
							}
						}
						var cursorID uint
						if s, _ := p.Args["cursor_id"].(string); s != "" {
							if v, err := strconv.Atoi(s); err == nil && v > 0 {
								cursorID = uint(v)
							}
						}
						dbq := DB.Model(&Task{})
						now := time.Now()
						if overdue {
							dbq = dbq.Where("deadline IS NOT NULL AND deadline < ?", now)
						} else {
							dbq = dbq.Where("deadline IS NULL OR deadline >= ?", now)
						}
						if strings.TrimSpace(qstr) != "" {
							like := "%" + strings.TrimSpace(qstr) + "%"
							dbq = dbq.Where("title LIKE ? OR description LIKE ?", like, like)
						}
						if cursorDeadline != nil {
							if overdue {
								dbq = dbq.Where("(deadline < ?) OR (deadline = ? AND id < ?)", cursorDeadline, cursorDeadline, cursorID)
							} else {
								dbq = dbq.Where("(deadline > ?) OR (deadline = ? AND id > ?)", cursorDeadline, cursorDeadline, cursorID)
							}
						} else if cursorID != 0 {
							if overdue {
								dbq = dbq.Where("id < ?", cursorID)
							} else {
								dbq = dbq.Where("id > ?", cursorID)
							}
						}
						if overdue {
							dbq = dbq.Order("deadline DESC").Order("id DESC")
						} else {
							dbq = dbq.Order("CASE WHEN deadline IS NULL THEN 1 ELSE 0 END ASC").Order("deadline ASC").Order("id ASC")
						}
						var items []Task
						if err := dbq.Limit(limit + 1).Find(&items).Error; err != nil {
							return nil, err
						}
						var nextCursor map[string]string
						if len(items) > limit {
							last := items[limit-1]
							items = items[:limit]
							if last.Deadline != nil {
								nextCursor = map[string]string{
									"cursor_deadline": last.Deadline.Format(time.RFC3339),
									"cursor_id":       fmt.Sprintf("%d", last.ID),
								}
							} else {
								nextCursor = map[string]string{"cursor_id": fmt.Sprintf("%d", last.ID)}
							}
						}
						// serialize items with string dates
						outItems := make([]map[string]any, 0, len(items))
						for _, it := range items {
							out := map[string]any{
								"id":                 it.ID,
								"title":              it.Title,
								"subject_id":         it.SubjectID,
								"description":        it.Description,
								"is_require_hand_in": it.IsRequireHandIn,
							}
							if it.Deadline != nil {
								out["deadline"] = it.Deadline.Format(time.RFC3339)
							} else {
								out["deadline"] = nil
							}
							outItems = append(outItems, out)
						}
						return map[string]any{"items": outItems, "next_cursor": nextCursor}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
		},
	})
	rootMutation := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"mcpCreate": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "McpCreateResult",
					Fields: graphql.Fields{
						"ok":    &graphql.Field{Type: graphql.Boolean},
						"token": &graphql.Field{Type: graphql.String},
					},
				}),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						if strings.ToLower(env("MCP_ENABLED", "true")) != "true" {
							return nil, fmt.Errorf("disabled")
						}
						// generate token and store hashed (re-using API key generator for randomness)
						plain, hashed, err := generateAPIKey()
						if err != nil {
							return nil, fmt.Errorf("gen")
						}
						// store only hash; user fetches token via logs/UI if needed (omitted here for brevity)
						rec := MCP{UserID: u.ID, HashedToken: hashed}
						_ = DB.Where("user_id = ?", u.ID).Delete(&MCP{}).Error
						if err := DB.Create(&rec).Error; err != nil {
							return nil, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: "mcp.create"}).Error
						return map[string]any{"ok": true, "token": plain}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"mcpRevoke": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						if err := DB.Where("user_id = ?", u.ID).Delete(&MCP{}).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: "mcp.revoke"}).Error
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			// Admin: Users list/search
			"adminUsers": &graphql.Field{
				Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
					Name: "AdminUser",
					Fields: graphql.Fields{
						"id":           &graphql.Field{Type: graphql.Int},
						"email":        &graphql.Field{Type: graphql.String},
						"display_name": &graphql.Field{Type: graphql.String},
						"roles":        &graphql.Field{Type: graphql.NewList(graphql.String)},
						"is_blocked":   &graphql.Field{Type: graphql.Boolean},
					},
				})),
				Args: graphql.FieldConfigArgument{"q": &graphql.ArgumentConfig{Type: graphql.String}, "limit": &graphql.ArgumentConfig{Type: graphql.Int}},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						if !(isAdmin || isManager) {
							return nil, fmt.Errorf("forbidden")
						}
						qstr, _ := p.Args["q"].(string)
						limit := 100
						if v, ok := p.Args["limit"].(int); ok && v > 0 && v <= 500 {
							limit = v
						}
						dbq := DB.Model(&User{})
						if strings.TrimSpace(qstr) != "" {
							like := "%" + strings.TrimSpace(qstr) + "%"
							dbq = dbq.Where("email LIKE ? OR display_name LIKE ?", like, like)
						}
						var users []User
						if err := dbq.Order("id DESC").Limit(limit).Find(&users).Error; err != nil {
							return nil, err
						}
						ids := make([]uint, 0, len(users))
						for _, uu := range users {
							ids = append(ids, uu.ID)
						}
						var roles []RoleUser
						if len(ids) > 0 {
							_ = DB.Where("user_id IN ?", ids).Find(&roles).Error
						}
						roleMap := map[uint][]string{}
						for _, r := range roles {
							roleMap[r.UserID] = append(roleMap[r.UserID], r.Role)
						}
						blocked := map[uint]bool{}
						if len(ids) > 0 {
							var blks []Block
							_ = DB.Where("user_id IN ? AND (expires_at IS NULL OR expires_at > ?)", ids, time.Now()).Find(&blks).Error
							for _, b := range blks {
								blocked[b.UserID] = true
							}
						}
						out := make([]map[string]any, 0, len(users))
						for _, uu := range users {
							out = append(out, map[string]any{"id": uu.ID, "email": uu.Email, "display_name": uu.DisplayName, "roles": roleMap[uu.ID], "is_blocked": blocked[uu.ID]})
						}
						return out, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			// Admin: role add/remove
			`adminUserAddRole`: &graphql.Field{ // backticks used to avoid accidental formatting issues
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)}, "role": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						if !(isAdmin || isManager) {
							return false, fmt.Errorf("forbidden")
						}
						uid := uint(p.Args["user_id"].(int))
						role := strings.ToLower(strings.TrimSpace(p.Args["role"].(string)))
						if role != "admin" && role != "manager" {
							return false, fmt.Errorf("invalid role")
						}
						var cnt int64
						_ = DB.Model(&RoleUser{}).Where("user_id = ? AND role = ?", uid, role).Count(&cnt).Error
						if cnt == 0 {
							_ = DB.Create(&RoleUser{UserID: uid, Role: role}).Error
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.role.add:%d:%s", uid, role)}).Error
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			`adminUserRemoveRole`: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)}, "role": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						if !(isAdmin || isManager) {
							return false, fmt.Errorf("forbidden")
						}
						uid := uint(p.Args["user_id"].(int))
						role := strings.ToLower(strings.TrimSpace(p.Args["role"].(string)))
						if err := DB.Where("user_id = ? AND role = ?", uid, role).Delete(&RoleUser{}).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.role.remove:%d:%s", uid, role)}).Error
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			// Admin: block/unblock
			`adminUserBlock`: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)}, "reason": &graphql.ArgumentConfig{Type: graphql.String}, "expires_at": &graphql.ArgumentConfig{Type: graphql.String}},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						if !(isAdmin || isManager) {
							return false, fmt.Errorf("forbidden")
						}
						uid := uint(p.Args["user_id"].(int))
						var expPtr *time.Time
						if s, ok := p.Args["expires_at"].(string); ok && strings.TrimSpace(s) != "" {
							if t, err := time.Parse(time.RFC3339, s); err == nil {
								expPtr = &t
							}
						}
						var reasonPtr *string
						if s, ok := p.Args["reason"].(string); ok && strings.TrimSpace(s) != "" {
							v := s
							reasonPtr = &v
						}
						rec := Block{UserID: uid, Reason: reasonPtr, ExpiresAt: expPtr}
						if err := DB.Create(&rec).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.block:%d", uid)}).Error
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			`adminUserUnblock`: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)}},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						if !(isAdmin || isManager) {
							return false, fmt.Errorf("forbidden")
						}
						uid := uint(p.Args["user_id"].(int))
						now := time.Now()
						if err := DB.Model(&Block{}).Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", uid, now).Updates(map[string]any{"expires_at": now}).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.unblock:%d", uid)}).Error
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			// Admin: calendar
			`adminCalendarUpsertRange`: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{"start": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, "end": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, "is_business_day": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)}, "notes": &graphql.ArgumentConfig{Type: graphql.String}},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						if !(isAdmin || isManager) {
							return false, fmt.Errorf("forbidden")
						}
						s := p.Args["start"].(string)
						e := p.Args["end"].(string)
						isBiz := p.Args["is_business_day"].(bool)
						sd, err1 := time.Parse("2006-01-02", s)
						ed, err2 := time.Parse("2006-01-02", e)
						if err1 != nil || err2 != nil || sd.After(ed) {
							return false, fmt.Errorf("invalid dates")
						}
						var notes *string
						if v, ok := p.Args["notes"].(string); ok && strings.TrimSpace(v) != "" {
							nv := v
							notes = &nv
						}
						for d := sd; !d.After(ed); d = d.Add(24 * time.Hour) {
							var bc BusinessCalendar
							if err := DB.Where("date = ?", d.Format("2006-01-02")).First(&bc).Error; err != nil {
								bc = BusinessCalendar{Date: dateOnly(d), IsBusinessDay: isBiz, Notes: notes}
								_ = DB.Create(&bc).Error
							} else {
								bc.IsBusinessDay = isBiz
								bc.Notes = notes
								_ = DB.Save(&bc).Error
							}
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.calendar.upsert"}).Error
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			// Admin: day shift cycles and pairs
			`adminDayshiftCycles`: &graphql.Field{
				Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{Name: "DayShiftCycle", Fields: graphql.Fields{"id": &graphql.Field{Type: graphql.Int}, "begin_date": &graphql.Field{Type: graphql.String}, "finish_date": &graphql.Field{Type: graphql.String}, "description": &graphql.Field{Type: graphql.String}}})),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						if !(isAdmin || isManager) {
							return nil, fmt.Errorf("forbidden")
						}
						var items []DayShiftCycle
						if err := DB.Order("begin_date DESC").Find(&items).Error; err != nil {
							return nil, err
						}
						out := make([]map[string]any, 0, len(items))
						for _, it := range items {
							out = append(out, map[string]any{"id": it.ID, "begin_date": it.BeginDate.Format("2006-01-02"), "finish_date": it.FinishDate.Format("2006-01-02"), "description": it.Description})
						}
						return out, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			`adminDayshiftCreateCycle`: &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{Name: "DayShiftCycleCreateResult", Fields: graphql.Fields{"id": &graphql.Field{Type: graphql.Int}}}),
				Args: graphql.FieldConfigArgument{"begin_date": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, "finish_date": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, "description": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						if !(isAdmin || isManager) {
							return nil, fmt.Errorf("forbidden")
						}
						bd, err1 := time.Parse("2006-01-02", p.Args["begin_date"].(string))
						fd, err2 := time.Parse("2006-01-02", p.Args["finish_date"].(string))
						if err1 != nil || err2 != nil || !bd.Before(fd) {
							return nil, fmt.Errorf("invalid dates")
						}
						desc := p.Args["description"].(string)
						rec := DayShiftCycle{BeginDate: bd, FinishDate: fd, Description: desc}
						if err := DB.Create(&rec).Error; err != nil {
							return nil, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.cycle.create"}).Error
						return map[string]any{"id": rec.ID}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			`adminDayshiftDeleteCycle`: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						if !(isAdmin || isManager) {
							return false, fmt.Errorf("forbidden")
						}
						if err := DB.Delete(&DayShiftCycle{}, p.Args["id"].(int)).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.cycle.delete"}).Error
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			`adminDayshiftPairs`: &graphql.Field{
				Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
					Name: "DayShiftPair",
					Fields: graphql.Fields{
						"id":       &graphql.Field{Type: graphql.Int},
						"cycle_id": &graphql.Field{Type: graphql.Int},
						"user1_id": &graphql.Field{Type: graphql.Int},
						"user2_id": &graphql.Field{Type: graphql.Int},
						"position": &graphql.Field{Type: graphql.Int},
					},
				})),
				Args: graphql.FieldConfigArgument{
					"cycle_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						if !(isAdmin || isManager) {
							return nil, fmt.Errorf("forbidden")
						}
						cid := p.Args["cycle_id"].(int)
						var pairs []DayShiftCyclePair
						if err := DB.Where("cycle_id = ?", cid).Order("position ASC, id ASC").Find(&pairs).Error; err != nil {
							return nil, fmt.Errorf("db")
						}
						out := make([]map[string]any, 0, len(pairs))
						for _, it := range pairs {
							out = append(out, map[string]any{
								"id":       it.ID,
								"cycle_id": it.CycleID,
								"user1_id": it.User1ID,
								"user2_id": it.User2ID,
								"position": it.Position,
							})
						}
						return out, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			`adminDayshiftCreatePair`: &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "DayShiftPairCreateResult",
					Fields: graphql.Fields{
						"id": &graphql.Field{Type: graphql.Int},
					},
				}),
				Args: graphql.FieldConfigArgument{
					"cycle_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
					"user1_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
					"user2_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						if !(isAdmin || isManager) {
							return nil, fmt.Errorf("forbidden")
						}
						cid := p.Args["cycle_id"].(int)
						u1 := p.Args["user1_id"].(int)
						u2 := p.Args["user2_id"].(int)
						var max int64
						_ = DB.Model(&DayShiftCyclePair{}).Where("cycle_id = ?", cid).Select("COALESCE(MAX(position),0)").Scan(&max).Error
						pos := int(max) + 1
						rec := DayShiftCyclePair{CycleID: uint(cid), User1ID: uint(u1), User2ID: uint(u2), Position: pos}
						if err := DB.Create(&rec).Error; err != nil {
							return nil, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.pair.create"}).Error
						return map[string]any{"id": rec.ID}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			`adminDayshiftDeletePair`: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"cycle_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
					"pair_id":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						if !(isAdmin || isManager) {
							return false, fmt.Errorf("forbidden")
						}
						if err := DB.Delete(
							&DayShiftCyclePair{},
							"id = ? AND cycle_id = ?",
							p.Args["pair_id"].(int),
							p.Args["cycle_id"].(int),
						).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.pair.delete"}).Error
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"apiKeyCreate": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "ApiKeyCreateResult",
					Fields: graphql.Fields{
						"id":    &graphql.Field{Type: graphql.Int},
						"token": &graphql.Field{Type: graphql.String},
					},
				}),
				Args: graphql.FieldConfigArgument{
					"description": &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						limit, err := apiKeyLimitForUser(u.ID)
						if err != nil {
							return nil, err
						}
						if limit >= 0 {
							var cur int64
							if err := DB.Model(&APIKey{}).Where("user_id = ? AND is_valid = 1", u.ID).Count(&cur).Error; err != nil {
								return nil, err
							}
							if int(cur) >= limit {
								return nil, fmt.Errorf("api key limit reached")
							}
						}
						desc, _ := p.Args["description"].(string)
						var descPtr *string
						if strings.TrimSpace(desc) != "" {
							descPtr = &desc
						}
						plain, hashed, genErr := generateAPIKey()
						if genErr != nil {
							return nil, fmt.Errorf("gen")
						}
						rec := APIKey{UserID: u.ID, HashedToken: hashed, Description: descPtr}
						if err := DB.Create(&rec).Error; err != nil {
							return nil, fmt.Errorf("db")
						}
						return map[string]any{"id": rec.ID, "token": plain}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"apiKeyUpdate": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "ApiKeyUpdateResult",
					Fields: graphql.Fields{
						"id":       &graphql.Field{Type: graphql.Int},
						"is_valid": &graphql.Field{Type: graphql.Boolean},
					},
				}),
				Args: graphql.FieldConfigArgument{
					"id":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
					"is_valid":    &graphql.ArgumentConfig{Type: graphql.Boolean},
					"description": &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						id, _ := p.Args["id"].(int)
						var rec APIKey
						if err := DB.Where("id = ? AND user_id = ?", id, u.ID).First(&rec).Error; err != nil {
							return nil, fmt.Errorf("not found")
						}
						if v, ok := p.Args["is_valid"].(bool); ok {
							rec.IsValid = v
						}
						if v, ok := p.Args["description"].(string); ok {
							if strings.TrimSpace(v) == "" {
								rec.Description = nil
							} else {
								rec.Description = &v
							}
						}
						if err := DB.Save(&rec).Error; err != nil {
							return nil, fmt.Errorf("db")
						}
						return map[string]any{"id": rec.ID, "is_valid": rec.IsValid}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"icalCreate": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "IcalCreateResult",
					Fields: graphql.Fields{
						"id":  &graphql.Field{Type: graphql.Int},
						"url": &graphql.Field{Type: graphql.String},
					},
				}),
				Args: graphql.FieldConfigArgument{
					"description": &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						desc, _ := p.Args["description"].(string)
						var descPtr *string
						if strings.TrimSpace(desc) != "" {
							descPtr = &desc
						}
						tok := generateIcalToken()
						rec := IcalKey{UserID: u.ID, Token: tok, Description: descPtr}
						if err := DB.Create(&rec).Error; err != nil {
							return nil, fmt.Errorf("db")
						}
						base := strings.TrimRight(env("ICAL_BASE_URL", env("API_PUBLIC_URL", "http://localhost:8080")+"/ical"), "/")
						return map[string]any{"id": rec.ID, "url": fmt.Sprintf("%s/%s.ics", base, rec.Token)}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"icalDelete": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						id, _ := p.Args["id"].(int)
						if err := DB.Where("id = ? AND user_id = ?", id, u.ID).Delete(&IcalKey{}).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"createTask": &graphql.Field{
				Type: taskType,
				Args: graphql.FieldConfigArgument{
					"title":              &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"subject_id":         &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
					"description":        &graphql.ArgumentConfig{Type: graphql.String},
					"is_require_hand_in": &graphql.ArgumentConfig{Type: graphql.Boolean},
					"deadline":           &graphql.ArgumentConfig{Type: graphql.String}, // RFC3339 or empty
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						if okp, err := checkPermission(u.ID, "tasks", "create", nil); err != nil {
							return nil, err
						} else if !okp {
							return nil, fmt.Errorf("forbidden")
						}
						title, _ := p.Args["title"].(string)
						subjectID, _ := p.Args["subject_id"].(int)
						desc, _ := p.Args["description"].(string)
						var descPtr *string
						if desc != "" {
							descPtr = &desc
						}
						irh, _ := p.Args["is_require_hand_in"].(bool)
						deadlineStr, _ := p.Args["deadline"].(string)
						var deadlinePtr *time.Time
						if strings.TrimSpace(deadlineStr) != "" {
							if t, err := time.Parse(time.RFC3339, deadlineStr); err == nil {
								deadlinePtr = &t
							} else {
								return nil, fmt.Errorf("invalid deadline")
							}
						}
						rec := Task{AddedUserID: u.ID, Title: title, SubjectID: uint(subjectID), Description: descPtr, IsRequireHandIn: irh, Deadline: deadlinePtr}
						if err := DB.Create(&rec).Error; err != nil {
							return nil, fmt.Errorf("db")
						}
						out := map[string]any{"id": rec.ID, "title": rec.Title, "subject_id": rec.SubjectID, "description": rec.Description, "is_require_hand_in": rec.IsRequireHandIn}
						if rec.Deadline != nil {
							out["deadline"] = rec.Deadline.Format(time.RFC3339)
						} else {
							out["deadline"] = nil
						}
						return out, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"updateTask": &graphql.Field{
				Type: taskType,
				Args: graphql.FieldConfigArgument{
					"id":                 &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
					"title":              &graphql.ArgumentConfig{Type: graphql.String},
					"subject_id":         &graphql.ArgumentConfig{Type: graphql.Int},
					"description":        &graphql.ArgumentConfig{Type: graphql.String},
					"is_require_hand_in": &graphql.ArgumentConfig{Type: graphql.Boolean},
					"deadline":           &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						id, _ := p.Args["id"].(int)
						var rec Task
						if err := DB.First(&rec, id).Error; err != nil {
							return nil, fmt.Errorf("not found")
						}
						if okp, err := checkPermission(u.ID, "tasks", "update", &rec.AddedUserID); err != nil {
							return nil, err
						} else if !okp {
							return nil, fmt.Errorf("forbidden")
						}
						if v, ok := p.Args["title"].(string); ok {
							rec.Title = v
						}
						if v, ok := p.Args["subject_id"].(int); ok {
							rec.SubjectID = uint(v)
						}
						if v, ok := p.Args["description"].(string); ok {
							if strings.TrimSpace(v) == "" {
								rec.Description = nil
							} else {
								rec.Description = &v
							}
						}
						if v, ok := p.Args["is_require_hand_in"].(bool); ok {
							rec.IsRequireHandIn = v
						}
						if v, ok := p.Args["deadline"].(string); ok {
							if strings.TrimSpace(v) == "" {
								rec.Deadline = nil
							} else if t, err := time.Parse(time.RFC3339, v); err == nil {
								rec.Deadline = &t
							} else {
								return nil, fmt.Errorf("invalid deadline")
							}
						}
						if err := DB.Save(&rec).Error; err != nil {
							return nil, fmt.Errorf("db")
						}
						out := map[string]any{"id": rec.ID, "title": rec.Title, "subject_id": rec.SubjectID, "description": rec.Description, "is_require_hand_in": rec.IsRequireHandIn}
						if rec.Deadline != nil {
							out["deadline"] = rec.Deadline.Format(time.RFC3339)
						} else {
							out["deadline"] = nil
						}
						return out, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"deleteTask": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						id, _ := p.Args["id"].(int)
						var rec Task
						if err := DB.First(&rec, id).Error; err != nil {
							return false, fmt.Errorf("not found")
						}
						// Admin/manager allow, otherwise need delete permission
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						allowed := isAdmin || isManager
						if !allowed {
							if okp, err := checkPermission(u.ID, "tasks", "delete", &rec.AddedUserID); err != nil {
								return false, err
							} else {
								allowed = okp
							}
						}
						if !allowed {
							return false, fmt.Errorf("forbidden")
						}
						if err := DB.Delete(&Task{}, rec.ID).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"updateMe": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"display_name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					displayName, _ := p.Args["display_name"].(string)
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) { // GraphQLはセッション専用
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						dn := strings.TrimSpace(displayName)
						if len(dn) > 255 {
							dn = dn[:255]
						}
						if err := DB.Model(&User{}).Where("id = ?", u.ID).Update("display_name", dn).Error; err != nil {
							return false, err
						}
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"setAuthSettings": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"email_login_enabled": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) { // GraphQLはセッション専用
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						val, _ := p.Args["email_login_enabled"].(bool)
						var fresh User
						if err := DB.First(&fresh, u.ID).Error; err != nil {
							return false, err
						}
						hasPassword := fresh.HashedPassword != nil && *fresh.HashedPassword != ""
						totpEnabled, _, _ := getActiveTOTP(fresh.ID)
						var pkCount int64
						_ = DB.Model(&Passkey{}).Where("user_id = ?", fresh.ID).Count(&pkCount).Error
						if !val && !(hasPassword || totpEnabled || pkCount > 0) {
							return false, fmt.Errorf("cannot disable without another auth factor")
						}
						if err := DB.Model(&User{}).Where("id = ?", fresh.ID).Update("email_code_allowed", val).Error; err != nil {
							return false, err
						}
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"logout": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) { // セッション専用
							return false, fmt.Errorf("unauthorized")
						}
						if tok, ok := c.Locals("token").(string); ok && tok != "" {
							_ = RDB.Del(c.Context(), "session:"+tok).Err()
						}
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"deleteMe": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) { // セッション専用
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						if err := DB.Model(&User{}).Where("id = ?", u.ID).Update("is_deleted", true).Error; err != nil {
							return false, err
						}
						if tok, ok := c.Locals("token").(string); ok && tok != "" {
							_ = RDB.Del(c.Context(), "session:"+tok).Err()
						}
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"totpSetup": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "TOTPSetup",
					Fields: graphql.Fields{
						"secret":  &graphql.Field{Type: graphql.String},
						"otpauth": &graphql.Field{Type: graphql.String},
					},
				}),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						secret := generateBase32Secret()
						key := fmt.Sprintf("totp-setup:%d", u.ID)
						if err := RDB.Set(c.Context(), key, secret, 10*time.Minute).Err(); err != nil {
							return nil, fmt.Errorf("server")
						}
						issuer := "Taskly"
						label := fmt.Sprintf("%s:%s", issuer, u.Email)
						otpauth := fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s", urlEncode(label), secret, urlEncode(issuer))
						return map[string]any{"secret": secret, "otpauth": otpauth}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"totpActivate": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"code": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						code, _ := p.Args["code"].(string)
						key := fmt.Sprintf("totp-setup:%d", u.ID)
						secret, _ := RDB.Get(c.Context(), key).Result()
						if secret == "" {
							return false, fmt.Errorf("setup expired")
						}
						if !validateTOTP(secret, code) {
							return false, fmt.Errorf("invalid code")
						}
						enc, err := encryptString(secret)
						if err != nil {
							return false, fmt.Errorf("server")
						}
						var ts TOTPSecret
						if err := DB.Where("user_id = ?", u.ID).First(&ts).Error; err == nil {
							ts.Secret = enc
							ts.IsActive = true
							_ = DB.Save(&ts).Error
						} else {
							_ = DB.Create(&TOTPSecret{UserID: u.ID, Secret: enc, IsActive: true}).Error
						}
						_ = RDB.Del(c.Context(), key).Err()
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"totpDisable": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						var ts TOTPSecret
						if err := DB.Where("user_id = ?", u.ID).First(&ts).Error; err != nil {
							return false, fmt.Errorf("not enabled")
						}
						ts.IsActive = false
						if err := DB.Save(&ts).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"recoveryGenerate": &graphql.Field{
				Type: graphql.NewList(graphql.String),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						_ = DB.Where("user_id = ? AND is_used = 0", u.ID).Delete(&RecoveryCode{}).Error
						codes := make([]string, 0, 10)
						for i := 0; i < 10; i++ {
							plain := generateRecoveryCode()
							hashed := sha256Hex([]byte(plain))
							_ = DB.Create(&RecoveryCode{UserID: u.ID, HashedCode: hashed}).Error
							codes = append(codes, plain)
						}
						return codes, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"webauthnRegisterBegin": &graphql.Field{
				Type: graphql.String, // returns JSON string of creation options
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return "", fmt.Errorf("unauthorized")
						}
						if WAuth == nil {
							return "", fmt.Errorf("webauthn not configured")
						}
						u := c.Locals("user").(*User)
						creation, sd, err := WAuth.BeginRegistration(u)
						if err != nil {
							return "", fmt.Errorf("begin failed")
						}
						bsd, _ := json.Marshal(sd)
						_ = RDB.Set(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID), string(bsd), 10*time.Minute).Err()
						b, _ := json.Marshal(creation)
						return string(b), nil
					}
					return "", fmt.Errorf("context missing")
				},
			},
			"webauthnRegisterFinish": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, // JSON string of finish payload
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value("fiberCtx").(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						if WAuth == nil {
							return false, fmt.Errorf("webauthn not configured")
						}
						u := c.Locals("user").(*User)
						raw, _ := RDB.Get(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID)).Result()
						if raw == "" {
							return false, fmt.Errorf("session expired")
						}
						var sd webauthn.SessionData
						_ = json.Unmarshal([]byte(raw), &sd)
						// Build http.Request equivalent with JSON body
						payload, _ := p.Args["input"].(string)
						req, _ := http.NewRequestWithContext(c.Context(), "POST", "", strings.NewReader(payload))
						req.Header.Set("Content-Type", "application/json")
						cred, err := WAuth.FinishRegistration(u, sd, req)
						if err != nil {
							return false, fmt.Errorf("finish failed")
						}
						pub := base64.RawStdEncoding.EncodeToString(cred.PublicKey)
						cid := base64.RawStdEncoding.EncodeToString(cred.ID)
						devName := "Passkey"
						rec := Passkey{UserID: u.ID, CredentialID: cid, PublicKey: pub, SignCount: int64(cred.Authenticator.SignCount), DeviceName: &devName}
						if err := DB.Create(&rec).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						_ = RDB.Del(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID)).Err()
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
		},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: rootQuery, Mutation: rootMutation})
	if err == nil {
		app.Post("/graphql", func(c *fiber.Ctx) error {
			// Parse GraphQL request: { query, variables }
			var body struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			_ = c.BodyParser(&body)
			params := graphql.Params{
				Schema:         schema,
				RequestString:  body.Query,
				VariableValues: body.Variables,
				Context:        context.WithValue(context.Background(), "fiberCtx", c),
			}
			result := graphql.Do(params)
			if len(result.Errors) > 0 {
				return c.Status(400).JSON(result)
			}
			return c.JSON(result)
		})
	}

	// Day shift scheduler (08:30 local) – minimal implementation
	if strings.ToLower(env("DAYSHIFT_ENABLED", "true")) == "true" {
		notifyTime := env("DAYSHIFT_NOTIFY_TIME", "08:30")
		go dayShiftScheduler(notifyTime)
	}

	// Auth endpoints (with email code delivery)
	app.Post("/auth/request-code", func(c *fiber.Ctx) error {
		var body struct {
			Email string `json:"email"`
		}
		if err := c.BodyParser(&body); err != nil || body.Email == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		email := strings.ToLower(strings.TrimSpace(body.Email))
		// cooldown: deny if a code was created within last 60s and not expired
		var recent LoginCode
		if err := DB.Where("email = ?", email).Order("id DESC").First(&recent).Error; err == nil {
			if time.Since(recent.CreatedAt) < 60*time.Second && recent.ConsumedAt == nil {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many requests"})
			}
		}
		// generate code
		code := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
		// send email via SMTP synchronously; only store code if send succeeds
		smtpHost := env("SMTP_HOST", "localhost")
		smtpPort := env("SMTP_PORT", "25")
		from := env("SMTP_FROM", "noreply@taskly.local")
		smtpUser := env("SMTP_USER", from)
		smtpPass := env("SMTP_PASSWORD", "")
		// Build RFC5322 message with required headers (From/To/Subject/Date)
		nowRFC := time.Now().Format(time.RFC1123Z)
		msg := []byte(
			"From: " + from + "\r\n" +
				"To: " + body.Email + "\r\n" +
				"Subject: Taskly Login Code\r\n" +
				"Date: " + nowRFC + "\r\n" +
				"MIME-Version: 1.0\r\n" +
				"Content-Type: text/plain; charset=utf-8\r\n" +
				"\r\n" +
				fmt.Sprintf("Your login code is: %s\r\nIt expires in 10 minutes.\r\n", code),
		)
		var auth smtp.Auth
		if smtpPass != "" {
			auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
		}
		// Use manual client with timeouts to prevent hanging
		toSec, _ := strconv.Atoi(env("SMTP_TIMEOUT", "60"))
		if toSec <= 0 {
			toSec = 60
		}
		dialer := &net.Dialer{Timeout: time.Duration(toSec) * time.Second}
		mode := strings.ToLower(strings.TrimSpace(env("SMTP_TLS", ""))) // "starttls" | "smtps" | ""
		var conn net.Conn
		var err error
		if mode == "smtps" || smtpPort == "465" {
			// Implicit TLS
			conn, err = tls.DialWithDialer(dialer, "tcp", smtpHost+":"+smtpPort, &tls.Config{ServerName: smtpHost})
		} else {
			conn, err = dialer.DialContext(c.Context(), "tcp", smtpHost+":"+smtpPort)
		}
		if err != nil {
			log.Printf("smtp dial error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail connect failed"})
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
		cl, err := smtp.NewClient(conn, smtpHost)
		if err != nil {
			log.Printf("smtp client error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail client failed"})
		}
		defer func() { _ = cl.Quit() }()
		// STARTTLS if requested
		if mode == "starttls" {
			if ok, _ := cl.Extension("STARTTLS"); ok {
				if err := cl.StartTLS(&tls.Config{ServerName: smtpHost}); err != nil {
					log.Printf("smtp starttls error: %v", err)
					return c.Status(502).JSON(fiber.Map{"error": "mail tls failed"})
				}
			}
		}
		if auth != nil {
			if ok, _ := cl.Extension("AUTH"); ok {
				_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
				if err := cl.Auth(auth); err != nil {
					log.Printf("smtp auth error: %v", err)
					return c.Status(502).JSON(fiber.Map{"error": "mail auth failed"})
				}
			}
		}
		_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
		if err := cl.Mail(from); err != nil {
			log.Printf("smtp mail from error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail from failed"})
		}
		_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
		if err := cl.Rcpt(body.Email); err != nil {
			log.Printf("smtp rcpt to error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail rcpt failed"})
		}
		_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
		w, err := cl.Data()
		if err != nil {
			log.Printf("smtp data start error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail data failed"})
		}
		_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
		if _, err := w.Write(msg); err != nil {
			_ = w.Close()
			log.Printf("smtp data write error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail write failed"})
		}
		_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
		if err := w.Close(); err != nil {
			log.Printf("smtp data close error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail finalize failed"})
		}
		// store code after successful send
		exp := time.Now().Add(10 * time.Minute)
		lc := LoginCode{Email: email, HashedCode: sha256Hex([]byte(code)), ExpiresAt: exp}
		if err := DB.Create(&lc).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "server"})
		}
		return c.SendStatus(fiber.StatusOK)
	})

	// Discover auth options for an email (password/TOTP presence)
	app.Post("/auth/options", func(c *fiber.Ctx) error {
		var body struct {
			Email string `json:"email"`
		}
		if err := c.BodyParser(&body); err != nil || body.Email == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		var u User
		hasUser := DB.Where("email = ?", body.Email).First(&u).Error == nil
		hasPassword := false
		if hasUser && u.HashedPassword != nil && *u.HashedPassword != "" {
			hasPassword = true
		}
		totpEnabled := false
		if hasUser {
			if ok, _, _ := getActiveTOTP(u.ID); ok {
				totpEnabled = true
			}
		}
		// passkey presence
		passkeyEnabled := false
		if hasUser {
			var cnt int64
			_ = DB.Model(&Passkey{}).Where("user_id = ?", u.ID).Count(&cnt).Error
			passkeyEnabled = cnt > 0
		}
		methods := []string{}
		if hasPassword {
			methods = append(methods, "password")
		}
		if !hasUser || (hasUser && u.EmailCodeAllowed) {
			methods = append(methods, "email_code")
		}
		if totpEnabled {
			methods = append(methods, "totp", "recovery")
		}
		if passkeyEnabled {
			methods = append(methods, "passkey")
		}
		return c.JSON(fiber.Map{"has_password": hasPassword, "totp_enabled": totpEnabled, "passkey_enabled": passkeyEnabled, "email_login_enabled": func() bool {
			if hasUser {
				return u.EmailCodeAllowed
			}
			return true
		}(), "methods": methods})
	})

	app.Post("/auth/login", func(c *fiber.Ctx) error {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Code     string `json:"code"`
			Totp     string `json:"totp"`
			Recovery string `json:"recovery_code"`
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
		// Primary factor: if password is set, require password; otherwise allow email code
		reqPassword := (u.HashedPassword != nil && *u.HashedPassword != "")
		primaryOK := false
		if reqPassword {
			if body.Password == "" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "password required"})
			}
			// verify bcrypt password
			if err := bcrypt.CompareHashAndPassword([]byte(*u.HashedPassword), []byte(body.Password)); err == nil {
				primaryOK = true
			} else {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid password"})
			}
		} else {
			// Respect user setting for email code login
			if !u.EmailCodeAllowed {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "email code disabled"})
			}
			// verify login code (email magic code) using DB
			if body.Code == "" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "code required"})
			}
			email := strings.ToLower(strings.TrimSpace(body.Email))
			var lc LoginCode
			if err := DB.Where("email = ? AND consumed_at IS NULL AND expires_at > ?", email, time.Now()).Order("id DESC").First(&lc).Error; err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid code"})
			}
			if lc.HashedCode != sha256Hex([]byte(body.Code)) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid code"})
			}
			now := time.Now()
			_ = DB.Model(&LoginCode{}).Where("id = ?", lc.ID).Update("consumed_at", now).Error
			primaryOK = true
		}
		if !primaryOK {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		// 2FA enforcement (TOTP or recovery code)
		if active, secret, _ := getActiveTOTP(u.ID); active {
			ok := false
			if body.Totp != "" {
				if validateTOTP(secret, body.Totp) {
					ok = true
				}
			}
			if !ok && body.Recovery != "" {
				if consumeRecoveryCode(u.ID, body.Recovery) {
					ok = true
				}
			}
			if !ok {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "2fa required"})
			}
		}
		// TODO: verify password if set

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

	// Auth and audit middlewares
	authRequired := func(c *fiber.Ctx) error {
		if userFromSession(c) || userFromAPIKey(c) {
			return c.Next()
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	audit := func(c *fiber.Ctx) error {
		err := c.Next()
		// Per policy: record ONLY REST requests authenticated with API Key
		if apiKey, ok := c.Locals("apiKey").(string); ok && apiKey != "" {
			hashed := sha256Hex([]byte(apiKey))
			var uid *uint
			if u, ok := c.Locals("user").(*User); ok {
				uid = &u.ID
			}
			_ = DB.Create(&ApiActionLog{UserID: uid, HashedToken: hashed, Action: c.Method() + " " + c.Path(), IP: c.IP()}).Error
		}
		return err
	}

	// Auth settings endpoints
	app.Get("/settings/auth", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		// refresh user
		var fresh User
		_ = DB.First(&fresh, u.ID).Error
		hasPassword := fresh.HashedPassword != nil && *fresh.HashedPassword != ""
		totpEnabled, _, _ := getActiveTOTP(fresh.ID)
		return c.JSON(fiber.Map{
			"email_login_enabled": fresh.EmailCodeAllowed,
			"has_password":        hasPassword,
			"totp_enabled":        totpEnabled,
		})
	})
	app.Post("/settings/auth", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct {
			EmailLoginEnabled *bool `json:"email_login_enabled"`
		}
		if err := c.BodyParser(&body); err != nil || body.EmailLoginEnabled == nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var fresh User
		if err := DB.First(&fresh, u.ID).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		// Allow disabling email login if user has password or TOTP enabled
		hasPassword := fresh.HashedPassword != nil && *fresh.HashedPassword != ""
		totpEnabled, _, _ := getActiveTOTP(fresh.ID)
		var pkCount int64
		_ = DB.Model(&Passkey{}).Where("user_id = ?", fresh.ID).Count(&pkCount).Error
		if !*body.EmailLoginEnabled && !(hasPassword || totpEnabled || pkCount > 0) {
			return c.Status(400).JSON(fiber.Map{"error": "cannot disable without another auth factor"})
		}
		fresh.EmailCodeAllowed = *body.EmailLoginEnabled
		if err := DB.Model(&User{}).Where("id = ?", fresh.ID).Update("email_code_allowed", fresh.EmailCodeAllowed).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// WebAuthn: helpers
	fiberToHTTPRequest := func(c *fiber.Ctx) *http.Request {
		req, _ := http.NewRequestWithContext(c.Context(), "POST", "", io.NopCloser(strings.NewReader(string(c.Body()))))
		for k, v := range c.GetReqHeaders() {
			if len(v) > 0 {
				req.Header.Set(k, v[0])
			}
		}
		return req
	}

	// WebAuthn registration (requires auth)
	app.Post("/webauthn/register/begin", authRequired, audit, func(c *fiber.Ctx) error {
		if WAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		u := c.Locals("user").(*User)
		// Load credentials for user into interface methods
		creation, sd, err := WAuth.BeginRegistration(u)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "begin failed"})
		}
		b, _ := json.Marshal(sd)
		_ = RDB.Set(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID), string(b), 10*time.Minute).Err()
		return c.JSON(creation)
	})
	app.Post("/webauthn/register/finish", authRequired, audit, func(c *fiber.Ctx) error {
		if WAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		u := c.Locals("user").(*User)
		raw, _ := RDB.Get(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID)).Result()
		if raw == "" {
			return c.Status(400).JSON(fiber.Map{"error": "session expired"})
		}
		var sd webauthn.SessionData
		_ = json.Unmarshal([]byte(raw), &sd)
		req := fiberToHTTPRequest(c)
		cred, err := WAuth.FinishRegistration(u, sd, req)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "finish failed"})
		}
		// store credential
		pub := base64.RawStdEncoding.EncodeToString(cred.PublicKey)
		cid := base64.RawStdEncoding.EncodeToString(cred.ID)
		devName := "Passkey"
		rec := Passkey{
			UserID:       u.ID,
			CredentialID: cid,
			PublicKey:    pub,
			SignCount:    int64(cred.Authenticator.SignCount),
			DeviceName:   &devName,
		}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		_ = RDB.Del(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID)).Err()
		return c.SendStatus(204)
	})

	// WebAuthn login (no auth)
	app.Post("/webauthn/login/begin", func(c *fiber.Ctx) error {
		if WAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		var body struct {
			Email string `json:"email"`
		}
		if err := c.BodyParser(&body); err != nil || strings.TrimSpace(body.Email) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var u User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		assertion, sd, err := WAuth.BeginLogin(&u)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "begin failed"})
		}
		b, _ := json.Marshal(sd)
		_ = RDB.Set(c.Context(), fmt.Sprintf("webauthn:login:%d", u.ID), string(b), 10*time.Minute).Err()
		return c.JSON(assertion)
	})
	app.Post("/webauthn/login/finish", func(c *fiber.Ctx) error {
		if WAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		var body struct {
			Email string `json:"email"`
		}
		_ = c.BodyParser(&body)
		if strings.TrimSpace(body.Email) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var u User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		raw, _ := RDB.Get(c.Context(), fmt.Sprintf("webauthn:login:%d", u.ID)).Result()
		if raw == "" {
			return c.Status(400).JSON(fiber.Map{"error": "session expired"})
		}
		var sd webauthn.SessionData
		_ = json.Unmarshal([]byte(raw), &sd)
		req := fiberToHTTPRequest(c)
		cred, err := WAuth.FinishLogin(&u, sd, req)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "invalid passkey"})
		}
		// Blocklist enforcement before issuing session
		var blkCnt int64
		_ = DB.Model(&Block{}).
			Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", u.ID, time.Now()).
			Count(&blkCnt).Error
		if blkCnt > 0 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "blocked"})
		}
		// update sign count
		_ = DB.Model(&Passkey{}).Where("user_id = ? AND credential_id = ?", u.ID, base64.RawStdEncoding.EncodeToString(cred.ID)).Update("sign_count", int64(cred.Authenticator.SignCount)).Error
		// issue session token
		token := fmt.Sprintf("sess_%d_%d", u.ID, time.Now().UnixNano())
		ttl := time.Hour * 24 * 30
		if err := RDB.Set(c.Context(), "session:"+token, fmt.Sprintf("%d", u.ID), ttl).Err(); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "session"})
		}
		DB.Create(&Session{ClientPlatform: "web", HashedToken: token, ExpiresAt: ptrTime(time.Now().Add(ttl))})
		claims := jwt.MapClaims{"sub": u.ID, "exp": time.Now().Add(ttl).Unix()}
		jwtt := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		jwtStr, _ := jwtt.SignedString([]byte(env("JWT_SECRET", "dev")))
		_ = RDB.Del(c.Context(), fmt.Sprintf("webauthn:login:%d", u.ID)).Err()
		return c.JSON(fiber.Map{"token": token, "jwt": jwtStr})
	})

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

	// API keys endpoints
	app.Get("/apikeys", authRequired, audit, func(c *fiber.Ctx) error {
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

	app.Post("/apikeys", authRequired, audit, func(c *fiber.Ctx) error {
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

	app.Patch("/apikeys/:id", authRequired, audit, func(c *fiber.Ctx) error {
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

	// iCal token management
	app.Get("/ical/tokens", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var items []IcalKey
		if err := DB.Where("user_id = ?", u.ID).Order("id DESC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		base := strings.TrimRight(env("ICAL_BASE_URL", env("API_PUBLIC_URL", "http://localhost:8080")+"/ical"), "/")
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, map[string]any{
				"id":          it.ID,
				"description": it.Description,
				"url":         fmt.Sprintf("%s/%s.ics", base, it.Token),
				"created_at":  it.CreatedAt,
			})
		}
		return c.JSON(out)
	})
	app.Post("/ical/tokens", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct {
			Description *string `json:"description"`
		}
		_ = c.BodyParser(&body)
		tok := generateIcalToken()
		rec := IcalKey{UserID: u.ID, Token: tok, Description: body.Description}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		base := strings.TrimRight(env("ICAL_BASE_URL", env("API_PUBLIC_URL", "http://localhost:8080")+"/ical"), "/")
		return c.Status(201).JSON(fiber.Map{"id": rec.ID, "url": fmt.Sprintf("%s/%s.ics", base, rec.Token)})
	})
	app.Delete("/ical/tokens/:id", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		if err := DB.Where("id = ? AND user_id = ?", c.Params("id"), u.ID).Delete(&IcalKey{}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// Day shift admin endpoints (admin only)
	adminOnly := func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		isAdmin, err1 := userHasRole(u.ID, "admin")
		isManager, err2 := userHasRole(u.ID, "manager")
		if err1 != nil || err2 != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		}
		if !(isAdmin || isManager) {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		return c.Next()
	}
	app.Get("/admin/dayshift/assignments", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var items []DayShiftAssignment
		if err := DB.Order("date DESC").Limit(200).Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(items)
	})
	app.Post("/admin/dayshift/rotate", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		// rotate to next business day and assign next pair
		if err := performDayShiftRotation(time.Now()); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "rotate"})
		}
		return c.SendStatus(204)
	})
	// mark complete/unfulfilled
	app.Post("/admin/dayshift/:date/status", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		dateStr := c.Params("date") // YYYY-MM-DD
		var body struct {
			Status string  `json:"status"`
			Notes  *string `json:"notes"`
		}
		if err := c.BodyParser(&body); err != nil || (body.Status != "completed" && body.Status != "unfulfilled" && body.Status != "assigned") {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var asg DayShiftAssignment
		if err := DB.Where("date = ?", dateStr).First(&asg).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		asg.Status = body.Status
		asg.Notes = body.Notes
		if err := DB.Save(&asg).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
	// shift assignment to next business day (e.g., school closed)
	app.Post("/admin/dayshift/:date/shift", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		dateStr := c.Params("date")
		var asg DayShiftAssignment
		if err := DB.Where("date = ?", dateStr).First(&asg).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		next := nextBusinessDay(asg.Date)
		if next.IsZero() {
			return c.Status(400).JSON(fiber.Map{"error": "no next business day"})
		}
		var target DayShiftAssignment
		if err := DB.Where("date = ?", next.Format("2006-01-02")).First(&target).Error; err != nil {
			target = DayShiftAssignment{Date: next, CycleID: asg.CycleID, Status: "assigned"}
			if err := DB.Create(&target).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
		}
		var users []DayShiftAssignmentUser
		_ = DB.Where("day_shift_assignments_id = ?", asg.ID).Find(&users).Error
		_ = DB.Where("day_shift_assignments_id = ?", target.ID).Delete(&DayShiftAssignmentUser{}).Error
		for _, u := range users {
			_ = DB.Create(&DayShiftAssignmentUser{DayShiftAssignmentsID: target.ID, UserID: u.UserID}).Error
		}
		return c.SendStatus(204)
	})

	// cycle pair management
	app.Get("/admin/dayshift/cycles/:id/pairs", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var pairs []DayShiftCyclePair
		if err := DB.Where("cycle_id = ?", c.Params("id")).Order("position ASC, id ASC").Find(&pairs).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(pairs)
	})
	app.Post("/admin/dayshift/cycles/:id/pairs", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		cycleID, _ := strconv.Atoi(c.Params("id"))
		var body struct {
			User1ID  uint `json:"user1_id"`
			User2ID  uint `json:"user2_id"`
			Position *int `json:"position"`
		}
		if err := c.BodyParser(&body); err != nil || body.User1ID == 0 || body.User2ID == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		pos := 0
		if body.Position != nil {
			pos = *body.Position
		} else {
			var max int64
			_ = DB.Model(&DayShiftCyclePair{}).Where("cycle_id = ?", cycleID).Select("COALESCE(MAX(position),0)").Scan(&max).Error
			pos = int(max) + 1
		}
		rec := DayShiftCyclePair{CycleID: uint(cycleID), User1ID: body.User1ID, User2ID: body.User2ID, Position: pos}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(201).JSON(rec)
	})
	app.Delete("/admin/dayshift/cycles/:id/pairs/:pairId", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		if err := DB.Delete(&DayShiftCyclePair{}, "id = ? AND cycle_id = ?", c.Params("pairId"), c.Params("id")).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
	app.Post("/admin/dayshift/cycles/:id/pairs/:pairId/move", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var body struct {
			Position int `json:"position"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var rec DayShiftCyclePair
		if err := DB.Where("id = ? AND cycle_id = ?", c.Params("pairId"), c.Params("id")).First(&rec).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		rec.Position = body.Position
		if err := DB.Save(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(rec)
	})

	// cycles management
	app.Get("/admin/dayshift/cycles", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var items []DayShiftCycle
		if err := DB.Order("begin_date DESC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(items)
	})
	app.Post("/admin/dayshift/cycles", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var body struct {
			BeginDate   string `json:"begin_date"`
			FinishDate  string `json:"finish_date"`
			Description string `json:"description"`
		}
		if err := c.BodyParser(&body); err != nil || body.BeginDate == "" || body.FinishDate == "" || body.Description == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		bd, err1 := time.Parse("2006-01-02", body.BeginDate)
		fd, err2 := time.Parse("2006-01-02", body.FinishDate)
		if err1 != nil || err2 != nil || !bd.Before(fd) {
			return c.Status(400).JSON(fiber.Map{"error": "invalid dates"})
		}
		rec := DayShiftCycle{BeginDate: bd, FinishDate: fd, Description: body.Description}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(201).JSON(rec)
	})
	app.Delete("/admin/dayshift/cycles/:id", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		if err := DB.Delete(&DayShiftCycle{}, c.Params("id")).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// business calendar management (range updates)
	app.Post("/admin/calendar/upsert-range", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var body struct {
			Start      string  `json:"start"`
			End        string  `json:"end"`
			IsBusiness bool    `json:"is_business_day"`
			Notes      *string `json:"notes"`
		}
		if err := c.BodyParser(&body); err != nil || body.Start == "" || body.End == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		s, e := body.Start, body.End
		sd, err1 := time.Parse("2006-01-02", s)
		ed, err2 := time.Parse("2006-01-02", e)
		if err1 != nil || err2 != nil || sd.After(ed) {
			return c.Status(400).JSON(fiber.Map{"error": "invalid dates"})
		}
		for d := sd; !d.After(ed); d = d.Add(24 * time.Hour) {
			var bc BusinessCalendar
			if err := DB.Where("date = ?", d.Format("2006-01-02")).First(&bc).Error; err != nil {
				bc = BusinessCalendar{Date: dateOnly(d), IsBusinessDay: body.IsBusiness, Notes: body.Notes}
				_ = DB.Create(&bc).Error
			} else {
				bc.IsBusinessDay = body.IsBusiness
				bc.Notes = body.Notes
				_ = DB.Save(&bc).Error
			}
		}
		return c.SendStatus(204)
	})

	// Admin: manage permissions
	app.Get("/admin/permissions", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var items []Permission
		if err := DB.Order("id DESC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(items)
	})
	app.Post("/admin/permissions", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var body struct {
			UserID uint    `json:"user_id"`
			Target string  `json:"target"`
			Method string  `json:"method"`
			Value  *string `json:"value"`
		}
		if err := c.BodyParser(&body); err != nil || body.UserID == 0 || body.Target == "" || body.Method == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		rec := Permission{UserID: body.UserID, Target: body.Target, Method: strings.ToLower(body.Method), Value: body.Value}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(201).JSON(rec)
	})
	app.Delete("/admin/permissions/:id", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		if err := DB.Delete(&Permission{}, c.Params("id")).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// Admin: user management (list/search, roles, block/unblock)
	app.Get("/admin/users", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		q := strings.TrimSpace(c.Query("q"))
		limit, _ := strconv.Atoi(c.Query("limit", "100"))
		if limit <= 0 || limit > 500 {
			limit = 100
		}
		dbq := DB.Model(&User{})
		if q != "" {
			like := "%" + q + "%"
			dbq = dbq.Where("email LIKE ? OR display_name LIKE ?", like, like)
		}
		var users []User
		if err := dbq.Order("id DESC").Limit(limit).Find(&users).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		// fetch roles and block status
		ids := make([]uint, 0, len(users))
		for _, u := range users {
			ids = append(ids, u.ID)
		}
		var roles []RoleUser
		if len(ids) > 0 {
			_ = DB.Where("user_id IN ?", ids).Find(&roles).Error
		}
		roleMap := map[uint][]string{}
		for _, r := range roles {
			roleMap[r.UserID] = append(roleMap[r.UserID], r.Role)
		}
		// block status
		blocked := map[uint]bool{}
		if len(ids) > 0 {
			var blks []Block
			_ = DB.Where("user_id IN ? AND (expires_at IS NULL OR expires_at > ?)", ids, time.Now()).Find(&blks).Error
			for _, b := range blks {
				blocked[b.UserID] = true
			}
		}
		out := make([]map[string]any, 0, len(users))
		for _, u := range users {
			out = append(out, map[string]any{"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "is_deleted": u.IsDeleted, "roles": roleMap[u.ID], "is_blocked": blocked[u.ID]})
		}
		return c.JSON(out)
	})
	app.Post("/admin/users/:id/roles", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		uid := c.Params("id")
		var body struct {
			Role string `json:"role"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		role := strings.ToLower(strings.TrimSpace(body.Role))
		if role != "admin" && role != "manager" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid role"})
		}
		// check exists
		var cnt int64
		_ = DB.Model(&RoleUser{}).Where("user_id = ? AND role = ?", uid, role).Count(&cnt).Error
		if cnt == 0 {
			_ = DB.Create(&RoleUser{UserID: toUint(uid), Role: role}).Error
		}
		return c.SendStatus(201)
	})
	app.Delete("/admin/users/:id/roles/:role", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		if err := DB.Where("user_id = ? AND role = ?", c.Params("id"), c.Params("role")).Delete(&RoleUser{}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
	app.Post("/admin/users/:id/block", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var body struct {
			Reason    *string `json:"reason"`
			ExpiresAt *string `json:"expires_at"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var expPtr *time.Time
		if body.ExpiresAt != nil && *body.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, *body.ExpiresAt); err == nil {
				expPtr = &t
			} else {
				return c.Status(400).JSON(fiber.Map{"error": "invalid expires_at"})
			}
		}
		rec := Block{UserID: toUint(c.Params("id")), Reason: body.Reason, ExpiresAt: expPtr}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(201)
	})
	app.Post("/admin/users/:id/unblock", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		now := time.Now()
		if err := DB.Model(&Block{}).Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", c.Params("id"), now).Updates(map[string]any{"expires_at": now}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// 2FA: TOTP setup, activate, disable
	app.Post("/2fa/totp/setup", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		// generate base32 secret
		secret := generateBase32Secret()
		// store in Redis temporary
		key := fmt.Sprintf("totp-setup:%d", u.ID)
		if err := RDB.Set(c.Context(), key, secret, 10*time.Minute).Err(); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "server"})
		}
		issuer := "Taskly"
		label := fmt.Sprintf("%s:%s", issuer, u.Email)
		otpauth := fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s", urlEncode(label), secret, urlEncode(issuer))
		return c.JSON(fiber.Map{"secret": secret, "otpauth": otpauth})
	})
	app.Post("/2fa/totp/activate", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct {
			Code string `json:"code"`
		}
		if err := c.BodyParser(&body); err != nil || body.Code == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		key := fmt.Sprintf("totp-setup:%d", u.ID)
		secret, _ := RDB.Get(c.Context(), key).Result()
		if secret == "" {
			return c.Status(400).JSON(fiber.Map{"error": "setup expired"})
		}
		if !validateTOTP(secret, body.Code) {
			return c.Status(401).JSON(fiber.Map{"error": "invalid code"})
		}
		enc, err := encryptString(secret)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "server"})
		}
		// upsert
		var ts TOTPSecret
		if err := DB.Where("user_id = ?", u.ID).First(&ts).Error; err == nil {
			ts.Secret = enc
			ts.IsActive = true
			_ = DB.Save(&ts).Error
		} else {
			_ = DB.Create(&TOTPSecret{UserID: u.ID, Secret: enc, IsActive: true}).Error
		}
		_ = RDB.Del(c.Context(), key).Err()
		return c.SendStatus(204)
	})
	app.Post("/2fa/totp/disable", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var ts TOTPSecret
		if err := DB.Where("user_id = ?", u.ID).First(&ts).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not enabled"})
		}
		ts.IsActive = false
		if err := DB.Save(&ts).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// Recovery codes
	app.Post("/2fa/recovery/generate", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		// delete existing unused codes
		_ = DB.Where("user_id = ? AND is_used = 0", u.ID).Delete(&RecoveryCode{}).Error
		// generate 10 codes
		codes := make([]string, 0, 10)
		for i := 0; i < 10; i++ {
			plain := generateRecoveryCode()
			hashed := sha256Hex([]byte(plain))
			_ = DB.Create(&RecoveryCode{UserID: u.ID, HashedCode: hashed}).Error
			codes = append(codes, plain)
		}
		return c.JSON(fiber.Map{"codes": codes})
	})
	app.Post("/2fa/recovery/consume", func(c *fiber.Ctx) error {
		var body struct {
			Email string `json:"email"`
			Code  string `json:"code"`
		}
		if err := c.BodyParser(&body); err != nil || body.Email == "" || body.Code == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var u User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		if consumeRecoveryCode(u.ID, body.Code) {
			return c.SendStatus(204)
		}
		return c.Status(401).JSON(fiber.Map{"error": "invalid code"})
	})

	// Tasks CRUD with permissions
	app.Get("/tasks", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		// Permission: read tasks (default allow)
		if ok, err := checkPermission(u.ID, "tasks", "read", nil); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		} else if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}

		overdue := c.Query("overdue") == "true"
		qstr := strings.TrimSpace(c.Query("q"))
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		if limit <= 0 || limit > 100 {
			limit = 20
		}
		// Cursor based on deadline and id to ensure stable ordering
		cursorDeadlineStr := c.Query("cursor_deadline")
		cursorIDStr := c.Query("cursor_id")
		var cursorDeadline *time.Time
		var cursorID uint
		if cursorDeadlineStr != "" {
			if t, err := time.Parse(time.RFC3339, cursorDeadlineStr); err == nil {
				cursorDeadline = &t
			}
		}
		if cursorIDStr != "" {
			if v, err := strconv.Atoi(cursorIDStr); err == nil && v > 0 {
				cursorID = uint(v)
			}
		}

		// Build query
		dbq := DB.Model(&Task{})
		now := time.Now()
		if overdue {
			dbq = dbq.Where("deadline IS NOT NULL AND deadline < ?", now)
		} else {
			dbq = dbq.Where("deadline IS NULL OR deadline >= ?", now)
		}
		if qstr != "" {
			like := "%" + qstr + "%"
			dbq = dbq.Where("title LIKE ? OR description LIKE ?", like, like)
		}
		// Apply cursor for pagination
		// For non-overdue list (ascending by deadline NULLS LAST then id), we page after (deadline, id)
		// For overdue (descending), we page before (deadline, id)
		if cursorDeadline != nil {
			if overdue {
				dbq = dbq.Where("(deadline < ?) OR (deadline = ? AND id < ?)", cursorDeadline, cursorDeadline, cursorID)
			} else {
				dbq = dbq.Where("(deadline > ?) OR (deadline = ? AND id > ?)", cursorDeadline, cursorDeadline, cursorID)
			}
		} else if cursorID != 0 {
			// Fallback if only id is present
			if overdue {
				dbq = dbq.Where("id < ?", cursorID)
			} else {
				dbq = dbq.Where("id > ?", cursorID)
			}
		}
		// Ordering
		if overdue {
			dbq = dbq.Order("deadline DESC").Order("id DESC")
		} else {
			// deadline ASC with NULLS LAST: emulate by ordering by (deadline IS NULL) then deadline ASC
			dbq = dbq.Order("CASE WHEN deadline IS NULL THEN 1 ELSE 0 END ASC").Order("deadline ASC").Order("id ASC")
		}
		var items []Task
		if err := dbq.Limit(limit + 1).Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		var nextCursor map[string]string
		if len(items) > limit {
			last := items[limit-1]
			items = items[:limit]
			if last.Deadline != nil {
				nextCursor = map[string]string{
					"cursor_deadline": last.Deadline.Format(time.RFC3339),
					"cursor_id":       fmt.Sprintf("%d", last.ID),
				}
			} else {
				nextCursor = map[string]string{"cursor_id": fmt.Sprintf("%d", last.ID)}
			}
		}
		return c.JSON(fiber.Map{"items": items, "next_cursor": nextCursor})
	})
	app.Post("/tasks", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		if ok, err := checkPermission(u.ID, "tasks", "create", nil); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		} else if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		var body struct {
			Title           string  `json:"title"`
			SubjectID       uint    `json:"subject_id"`
			Description     *string `json:"description"`
			IsRequireHandIn *bool   `json:"is_require_hand_in"`
			Deadline        *string `json:"deadline"`
		}
		if err := c.BodyParser(&body); err != nil || body.Title == "" || body.SubjectID == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var deadlinePtr *time.Time
		if body.Deadline != nil && *body.Deadline != "" {
			if t, err := time.Parse(time.RFC3339, *body.Deadline); err == nil {
				deadlinePtr = &t
			} else {
				return c.Status(400).JSON(fiber.Map{"error": "invalid deadline"})
			}
		}
		rec := Task{AddedUserID: u.ID, Title: body.Title, SubjectID: body.SubjectID, Description: body.Description, Deadline: deadlinePtr}
		if body.IsRequireHandIn != nil {
			rec.IsRequireHandIn = *body.IsRequireHandIn
		}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(201).JSON(rec)
	})
	app.Patch("/tasks/:id", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var rec Task
		if err := DB.First(&rec, c.Params("id")).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		var body struct {
			Title           *string `json:"title"`
			SubjectID       *uint   `json:"subject_id"`
			Description     *string `json:"description"`
			IsRequireHandIn *bool   `json:"is_require_hand_in"`
			Deadline        *string `json:"deadline"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if body.Title != nil {
			rec.Title = *body.Title
		}
		if body.SubjectID != nil {
			rec.SubjectID = *body.SubjectID
		}
		if body.Description != nil {
			rec.Description = body.Description
		}
		if body.IsRequireHandIn != nil {
			rec.IsRequireHandIn = *body.IsRequireHandIn
		}
		if body.Deadline != nil {
			if *body.Deadline == "" {
				rec.Deadline = nil
			} else if t, err := time.Parse(time.RFC3339, *body.Deadline); err == nil {
				rec.Deadline = &t
			} else {
				return c.Status(400).JSON(fiber.Map{"error": "invalid deadline"})
			}
		}
		// enforce permission: update own or explicit
		if ok, err := checkPermission(u.ID, "tasks", "update", &rec.AddedUserID); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		} else if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		if err := DB.Save(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(rec)
	})
	app.Delete("/tasks/:id", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		if _, err := strconv.Atoi(c.Params("id")); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
		}
		var rec Task
		if err := DB.First(&rec, c.Params("id")).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		// Admin/manager fallback, otherwise require delete permission
		isAdmin, _ := userHasRole(u.ID, "admin")
		isManager, _ := userHasRole(u.ID, "manager")
		allowed := isAdmin || isManager
		if !allowed {
			if ok, err := checkPermission(u.ID, "tasks", "delete", &rec.AddedUserID); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "perm"})
			} else {
				allowed = ok
			}
		}
		if !allowed {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		if err := DB.Delete(&Task{}, rec.ID).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	app.Post("/integrations/teams/status", func(c *fiber.Ctx) error {
		// verify HMAC signature
		body := c.Body()
		sig := c.Get("X-Signature")
		secret := env("TEAMS_STATUS_SIGNING_SECRET", "")
		if secret != "" {
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(body)
			expected := fmt.Sprintf("sha256=%x", mac.Sum(nil))
			if sig == "" || !hmac.Equal([]byte(sig), []byte(expected)) {
				return c.Status(401).JSON(fiber.Map{"error": "invalid signature"})
			}
		}
		// store minimal log
		type payload struct {
			IsSuccess      bool   `json:"is_success"`
			TaskID         uint   `json:"task_id"`
			MessageID      string `json:"message_id"`
			ConversationID string `json:"conversation_id"`
		}
		var p payload
		_ = c.BodyParser(&p)
		if p.TaskID != 0 {
			_ = DB.Table("tasks_teams_logs").Create(map[string]any{
				"task_id":    p.TaskID,
				"is_success": p.IsSuccess,
				"teams_message_id": func() *string {
					if p.MessageID == "" {
						return nil
					}
					v := p.MessageID
					return &v
				}(),
				"teams_conversation_id": func() *string {
					if p.ConversationID == "" {
						return nil
					}
					v := p.ConversationID
					return &v
				}(),
				"last_status": "callback_received",
			}).Error
		}
		return c.SendStatus(fiber.StatusOK)
	})

	// Teams inbound + async processing
	// Simple buffered channel acts as a queue for demo purposes
	var teamsQueue = make(chan map[string]any, 100)
	// Worker goroutine
	go func() {
		for job := range teamsQueue {
			// Extract data
			title, _ := job["title"].(string)
			desc, _ := job["description"].(*string)
			addedUserID, _ := job["added_user_id"].(uint)
			subjectID, _ := job["subject_id"].(uint)
			callback, _ := job["status_callback"].(*string)
			// Create task
			rec := Task{AddedUserID: addedUserID, Title: title, SubjectID: subjectID, Description: desc}
			var isSuccess bool
			if err := DB.Create(&rec).Error; err != nil {
				log.Printf("teams worker: create task error: %v", err)
				isSuccess = false
			} else {
				isSuccess = true
			}
			// Log into tasks_teams_logs (initial)
			_ = DB.Table("tasks_teams_logs").Create(map[string]any{
				"task_id":    rec.ID,
				"is_success": isSuccess,
				"callback_url": func() *string {
					if callback == nil {
						return nil
					}
					return callback
				}(),
				"attempts": 0,
				"last_status": func() string {
					if isSuccess {
						return "created"
					} else {
						return "create_failed"
					}
				}(),
			}).Error
			// Callback with retries if provided
			if callback != nil && *callback != "" {
				cbURL := *callback
				payload := fmt.Sprintf(`{"is_success":%t,"task_id":%d}`, isSuccess, rec.ID)
				secret := env("TEAMS_STATUS_SIGNING_SECRET", "")
				// HMAC signature
				var sig string
				if secret != "" {
					mac := hmac.New(sha256.New, []byte(secret))
					mac.Write([]byte(payload))
					sig = fmt.Sprintf("sha256=%x", mac.Sum(nil))
				}
				client := &http.Client{Timeout: 10 * time.Second}
				attempts := 0
				for attempts < 3 {
					attempts++
					req, _ := http.NewRequest("POST", cbURL, strings.NewReader(payload))
					req.Header.Set("Content-Type", "application/json")
					if sig != "" {
						req.Header.Set("X-Signature", sig)
					}
					resp, err := client.Do(req)
					if err != nil {
						_ = DB.Table("tasks_teams_logs").Where("task_id = ?", rec.ID).Updates(map[string]any{
							"attempts":    attempts,
							"last_status": "callback_error",
							"last_error":  err.Error(),
						}).Error
						time.Sleep(time.Duration(attempts) * time.Second)
						continue
					}
					bodyBytes := make([]byte, 0)
					if resp.Body != nil {
						bodyBytes, _ = io.ReadAll(resp.Body)
						_ = resp.Body.Close()
					}
					_ = DB.Table("tasks_teams_logs").Where("task_id = ?", rec.ID).Updates(map[string]any{
						"attempts":             attempts,
						"last_status":          "callback_sent",
						"last_response_status": resp.StatusCode,
						"last_response_body":   string(bodyBytes),
					}).Error
					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						break
					}
					time.Sleep(time.Duration(attempts) * time.Second)
				}
			}
		}
	}()

	app.Post("/integrations/teams/inbound", func(c *fiber.Ctx) error {
		// Verify shared secret header
		secret := c.Get("X-Teams-Secret")
		if secret == "" || secret != env("TEAMS_CALLBACK_ENDPOINT_SECRET", "change_me") {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		var body struct {
			Email          string  `json:"email"`
			Title          string  `json:"title"`
			Description    *string `json:"description"`
			SubjectID      uint    `json:"subject_id"`
			StatusCallback *string `json:"status_callback"`
		}
		if err := c.BodyParser(&body); err != nil || body.Email == "" || body.Title == "" || body.SubjectID == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		// Resolve user by email
		var u User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "user not found"})
		}
		// Enqueue job
		job := map[string]any{
			"title":         body.Title,
			"description":   body.Description,
			"added_user_id": u.ID,
			"subject_id":    body.SubjectID,
		}
		if body.StatusCallback != nil {
			job["status_callback"] = body.StatusCallback
		}
		select {
		case teamsQueue <- job:
			return c.SendStatus(202)
		default:
			return c.Status(503).JSON(fiber.Map{"error": "queue full"})
		}
	})

	// Inbound endpoint to mark day shift unfulfilled when an email about 日直 is received via Power Automate
	app.Post("/integrations/dayshift/inbound", func(c *fiber.Ctx) error {
		secret := c.Get("X-Taskly-Secret")
		if secret == "" || secret != env("DAYSHIFT_INBOUND_SECRET", "change_me") {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		var body struct {
			Title   string  `json:"title"`
			Content string  `json:"content"`
			Date    *string `json:"date"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if !strings.Contains(body.Title+" "+body.Content, "日直") {
			return c.Status(400).JSON(fiber.Map{"error": "not a dayshift notice"})
		}
		var target time.Time
		if body.Date != nil && *body.Date != "" {
			if t, err := time.Parse("2006-01-02", *body.Date); err == nil {
				target = t
			} else {
				target = dateOnly(time.Now())
			}
		} else {
			target = dateOnly(time.Now())
		}
		var asg DayShiftAssignment
		if err := DB.Where("date = ?", target.Format("2006-01-02")).First(&asg).Error; err != nil {
			// create if missing within current cycle if any
			cyc, _ := getCurrentCycle(target)
			asg = DayShiftAssignment{Date: dateOnly(target)}
			if cyc != nil {
				asg.CycleID = cyc.ID
			}
			asg.Status = "unfulfilled"
			if err := DB.Create(&asg).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
		} else {
			asg.Status = "unfulfilled"
			_ = DB.Save(&asg).Error
		}
		return c.SendStatus(202)
	})

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

func generateIcalToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("ict_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("ict_%x", buf)
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

// checkPermission evaluates per-user permissions for a target/action.
// Returns true if allowed.
// Semantics:
// - Admin: allow all
// - Manager: allow all
// - If a Permission exists for (user, target, action):
//   - value == nil or "all": allow
//   - value == "own": allow only when ownerID != nil and *ownerID == userID
//
// - Defaults (no explicit permission):
//   - read: allow
//   - create: allow
//   - update: allow own only
//   - delete: deny
func checkPermission(userID uint, target, action string, ownerID *uint) (bool, error) {
	// Role shortcuts
	if ok, err := userHasRole(userID, "admin"); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}
	if ok, err := userHasRole(userID, "manager"); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}

	// Explicit permission override
	var p Permission
	if err := DB.Where("user_id = ? AND target = ? AND method = ?", userID, target, action).Order("id DESC").First(&p).Error; err == nil {
		if p.Value == nil || *p.Value == "" || strings.EqualFold(*p.Value, "all") {
			return true, nil
		}
		if strings.EqualFold(*p.Value, "own") {
			if ownerID != nil && *ownerID == userID {
				return true, nil
			}
			return false, nil
		}
		// Unknown value: deny by default
		return false, nil
	}

	// Defaults
	switch strings.ToLower(action) {
	case "read":
		return true, nil
	case "create":
		return true, nil
	case "update":
		if ownerID != nil && *ownerID == userID {
			return true, nil
		}
		return false, nil
	case "delete":
		return false, nil
	default:
		return false, nil
	}
}

// Extract user from session token
func userFromSession(c *fiber.Ctx) bool {
	token := c.Get("Authorization")
	if token == "" {
		return false
	}
	uidStr, err := RDB.Get(c.Context(), "session:"+token).Result()
	if err != nil {
		return false
	}
	var u User
	if err := DB.First(&u, uidStr).Error; err != nil {
		return false
	}
	var cnt int64
	if err := DB.Model(&Block{}).Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", u.ID, time.Now()).Count(&cnt).Error; err == nil && cnt > 0 {
		return false
	}
	c.Locals("user", &u)
	c.Locals("token", token)
	return true
}

// Extract user from API key
func userFromAPIKey(c *fiber.Ctx) bool {
	apiKey := c.Get("X-API-Key")
	if apiKey == "" {
		return false
	}
	hashed := sha256Hex([]byte(apiKey))
	var key APIKey
	if err := DB.Where("hashed_token = ? AND is_valid = 1", hashed).First(&key).Error; err != nil {
		return false
	}
	var u User
	if err := DB.First(&u, key.UserID).Error; err != nil {
		return false
	}
	var cnt int64
	if err := DB.Model(&Block{}).Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", u.ID, time.Now()).Count(&cnt).Error; err == nil && cnt > 0 {
		return false
	}
	c.Locals("user", &u)
	c.Locals("apiKey", apiKey)
	return true
}

// 2FA helpers
func urlEncode(s string) string { return url.QueryEscape(s) }

func generateBase32Secret() string {
	// TOTP recommends a 160-bit secret; generate 20 random bytes
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		// fallback to timestamp-based simple secret (should not happen)
		return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return strings.TrimRight(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), "=")
}

func validateTOTP(secret, code string) bool {
	// Allow default 30-second period and 1-step skew
	return totp.Validate(code, secret)
}

func generateRecoveryCode() string {
	// Human-friendly set excluding ambiguous chars
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const groups = 4
	const perGroup = 4
	out := make([]byte, 0, groups*perGroup+(groups-1))
	buf := make([]byte, groups*perGroup)
	if _, err := rand.Read(buf); err != nil {
		// fallback
		return fmt.Sprintf("%08x", time.Now().UnixNano())
	}
	for i := 0; i < groups*perGroup; i++ {
		out = append(out, alphabet[int(buf[i])%len(alphabet)])
		if (i+1)%perGroup == 0 && i+1 < groups*perGroup {
			out = append(out, '-')
		}
	}
	return string(out)
}

func consumeRecoveryCode(userID uint, code string) bool {
	hashed := sha256Hex([]byte(code))
	ok := false
	_ = DB.Transaction(func(tx *gorm.DB) error {
		var rc RecoveryCode
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ? AND hashed_code = ? AND is_used = 0", userID, hashed).First(&rc).Error; err != nil {
			return nil
		}
		rc.IsUsed = true
		if err := tx.Save(&rc).Error; err != nil {
			return err
		}
		ok = true
		return nil
	})
	return ok
}

func getActiveTOTP(userID uint) (bool, string, error) {
	var ts TOTPSecret
	if err := DB.Where("user_id = ? AND is_active = 1", userID).First(&ts).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, "", nil
		}
		return false, "", err
	}
	secret, err := decryptString(ts.Secret)
	if err != nil {
		return false, "", err
	}
	return true, secret, nil
}

func getEncKey() []byte {
	// Derive a 32-byte key from env
	keyStr := env("TOTP_ENC_KEY", "dev_totp_key")
	sum := sha256.Sum256([]byte(keyStr))
	return sum[:]
}

func encryptString(plain string) (string, error) {
	key := getEncKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, []byte(plain), nil)
	out := append(nonce, ct...)
	return base64.RawStdEncoding.EncodeToString(out), nil
}

func decryptString(enc string) (string, error) {
	key := getEncKey()
	raw, err := base64.RawStdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// iCal builder
func buildICSForUser(userID uint) string {
	// RFC5545 requires CRLF line endings
	b := &strings.Builder{}
	crlf := "\r\n"
	now := time.Now().UTC().Format("20060102T150405Z")
	b.WriteString("BEGIN:VCALENDAR" + crlf)
	b.WriteString("VERSION:2.0" + crlf)
	b.WriteString("PRODID:-//Taskly//EN" + crlf)
	b.WriteString("CALSCALE:GREGORIAN" + crlf)
	b.WriteString("METHOD:PUBLISH" + crlf)
	// Tasks with deadlines
	var tasks []Task
	_ = DB.Where("added_user_id = ? AND deadline IS NOT NULL", userID).Order("deadline ASC").Limit(500).Find(&tasks).Error
	for _, t := range tasks {
		if t.Deadline == nil {
			continue
		}
		// Use timed event with UTC timestamps at the deadline time
		b.WriteString("BEGIN:VEVENT" + crlf)
		b.WriteString(fmt.Sprintf("UID:tsk-%d@taskly\r\n", t.ID))
		b.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", now))
		b.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", icalEscape("[Task] "+t.Title)))
		if t.Description != nil {
			b.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", icalEscape(*t.Description)))
		}
		b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", t.Deadline.UTC().Format("20060102T150405Z")))
		b.WriteString(fmt.Sprintf("DTEND:%s\r\n", t.Deadline.UTC().Format("20060102T150405Z")))
		b.WriteString("END:VEVENT" + crlf)
	}
	// Day shift assignments for user
	var asgs []struct {
		ID   uint
		Date time.Time
	}
	_ = DB.Table("day_shift_assignments a").
		Select("a.id, a.date").
		Joins("JOIN day_shift_assignments_users u ON u.day_shift_assignments_id = a.id").
		Where("u.user_id = ?", userID).Order("a.date ASC").Limit(500).Scan(&asgs).Error
	for _, a := range asgs {
		start := time.Date(a.Date.Year(), a.Date.Month(), a.Date.Day(), 9, 0, 0, 0, a.Date.Location())
		end := start.Add(8 * time.Hour)
		b.WriteString("BEGIN:VEVENT" + crlf)
		b.WriteString(fmt.Sprintf("UID:ds-%d@taskly\r\n", a.ID))
		b.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", now))
		b.WriteString("SUMMARY:Day Shift\r\n")
		b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", start.UTC().Format("20060102T150405Z")))
		b.WriteString(fmt.Sprintf("DTEND:%s\r\n", end.UTC().Format("20060102T150405Z")))
		b.WriteString("END:VEVENT" + crlf)
	}
	b.WriteString("END:VCALENDAR" + crlf)
	return b.String()
}

func icalEscape(s string) string {
	// Escape commas, semicolons, and backslashes; replace newlines
	r := strings.ReplaceAll(s, "\\", "\\\\")
	r = strings.ReplaceAll(r, ",", "\\,")
	r = strings.ReplaceAll(r, ";", "\\;")
	r = strings.ReplaceAll(r, "\n", "\\n")
	r = strings.ReplaceAll(r, "\r", "")
	return r
}

// small helpers
func toUint(s string) uint {
	v, _ := strconv.ParseUint(s, 10, 64)
	return uint(v)
}

// Day shift helpers
func dayShiftScheduler(notifyHHmm string) {
	// parse HH:MM
	parts := strings.SplitN(notifyHHmm, ":", 2)
	hh, _ := strconv.Atoi(parts[0])
	mm, _ := strconv.Atoi(parts[1])
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), hh, mm, 0, 0, now.Location())
		if !now.Before(next) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(time.Until(next))
		// On trigger: notify assigned users for today if business day
		go func(ts time.Time) {
			_ = notifyDayShift(ts)
			// also check cycles and auto-extend if necessary per spec (minimal placeholder)
			_ = ensureNextCycleIfNearEnd(ts)
		}(time.Now())
	}
}

func isBusinessDay(d time.Time) bool {
	var bc BusinessCalendar
	if err := DB.Where("date = ?", d.Format("2006-01-02")).First(&bc).Error; err == nil {
		return bc.IsBusinessDay
	}
	// default Mon-Fri business day
	wd := d.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}

func notifyDayShift(ts time.Time) error {
	if !isBusinessDay(ts) {
		return nil
	}
	// fetch assignments for date
	var asg DayShiftAssignment
	if err := DB.Where("date = ?", ts.Format("2006-01-02")).First(&asg).Error; err != nil {
		return nil // no assignment
	}
	var users []DayShiftAssignmentUser
	_ = DB.Where("day_shift_assignments_id = ?", asg.ID).Find(&users).Error
	if len(users) == 0 {
		return nil
	}
	// Discord webhook notify (simple message)
	hook := env("DISCORD_WEBHOOK_URL", "")
	if hook == "" {
		return nil
	}
	content := fmt.Sprintf("Day shift for %s: %d user(s) assigned.", ts.Format("2006-01-02"), len(users))
	payload := fmt.Sprintf(`{"content": %q}`, content)
	req, _ := http.NewRequest("POST", hook, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err == nil && resp != nil {
		_ = resp.Body.Close()
	}
	return err
}

func performDayShiftRotation(now time.Time) error {
	// Determine target business day >= today
	d := now
	for !isBusinessDay(d) {
		d = d.Add(24 * time.Hour)
	}
	// find current cycle covering day
	cyc, err := getCurrentCycle(d)
	if err != nil {
		return err
	}
	if cyc == nil {
		return nil
	}
	// carryover: if previous business day's assignment is unfulfilled, reuse the same pair
	prev := prevBusinessDay(d)
	if !prev.IsZero() {
		var prevAsg DayShiftAssignment
		if err := DB.Where("date = ?", prev.Format("2006-01-02")).First(&prevAsg).Error; err == nil {
			if strings.EqualFold(prevAsg.Status, "unfulfilled") {
				// upsert today with same users
				var today DayShiftAssignment
				if err := DB.Where("date = ?", d.Format("2006-01-02")).First(&today).Error; err != nil {
					today = DayShiftAssignment{Date: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location()), Status: "assigned", CycleID: cyc.ID}
					if err := DB.Create(&today).Error; err != nil {
						return err
					}
				}
				// copy users
				var prevUsers []DayShiftAssignmentUser
				_ = DB.Where("day_shift_assignments_id = ?", prevAsg.ID).Find(&prevUsers).Error
				_ = DB.Where("day_shift_assignments_id = ?", today.ID).Delete(&DayShiftAssignmentUser{}).Error
				for _, pu := range prevUsers {
					_ = DB.Create(&DayShiftAssignmentUser{DayShiftAssignmentsID: today.ID, UserID: pu.UserID}).Error
				}
				return nil
			}
		}
	}
	// Otherwise assign next pair based on rotation within cycle
	var today DayShiftAssignment
	if err := DB.Where("date = ?", d.Format("2006-01-02")).First(&today).Error; err != nil {
		today = DayShiftAssignment{Date: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location()), Status: "assigned", CycleID: cyc.ID}
		if err := DB.Create(&today).Error; err != nil {
			return err
		}
	}
	// Prefer explicit pairs configured on the cycle
	var cfgPairs []DayShiftCyclePair
	_ = DB.Where("cycle_id = ?", cyc.ID).Order("position ASC, id ASC").Find(&cfgPairs).Error
	var pairs [][]uint
	if len(cfgPairs) > 0 {
		for _, p := range cfgPairs {
			pairs = append(pairs, []uint{p.User1ID, p.User2ID})
		}
	} else {
		// compute pairs from active users (IsDayShiftActive) at runtime
		actUsers, err := listActiveUsers()
		if err != nil || len(actUsers) < 2 {
			return err
		}
		pairs = computePairs(actUsers)
	}
	if len(pairs) == 0 {
		return nil
	}
	// compute day index in cycle (business days since cycle begin up to today inclusive)
	dayIdx := businessDaysBetween(cyc.BeginDate, d)
	pair := pairs[dayIdx%len(pairs)]
	// apply to assignment
	_ = DB.Where("day_shift_assignments_id = ?", today.ID).Delete(&DayShiftAssignmentUser{}).Error
	for _, uid := range pair {
		_ = DB.Create(&DayShiftAssignmentUser{DayShiftAssignmentsID: today.ID, UserID: uid}).Error
	}
	return nil
}

func ensureNextCycleIfNearEnd(now time.Time) error {
	// if cycle ending within 3 days
	var cyc DayShiftCycle
	if err := DB.Order("finish_date DESC").First(&cyc).Error; err != nil {
		return nil
	}
	if now.After(cyc.FinishDate.AddDate(0, 0, -3)) {
		// is there a next cycle defined?
		var next DayShiftCycle
		if err := DB.Where("begin_date > ?", cyc.FinishDate).Order("begin_date ASC").First(&next).Error; err != nil {
			// no next cycle => Discord error
			hook := env("DISCORD_WEBHOOK_URL", "")
			if hook != "" {
				content := fmt.Sprintf("[DayShift] No next cycle defined after %s", cyc.FinishDate.Format("2006-01-02"))
				payload := fmt.Sprintf(`{"content": %q}`, content)
				req, _ := http.NewRequest("POST", hook, strings.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
				client := &http.Client{Timeout: 5 * time.Second}
				resp, err := client.Do(req)
				if err == nil && resp != nil {
					_ = resp.Body.Close()
				}
			}
			return nil
		}
		// ensure skeleton assignments exist for each business day in next cycle
		for dt := dateOnly(next.BeginDate); !dt.After(dateOnly(next.FinishDate)); dt = dt.Add(24 * time.Hour) {
			if !isBusinessDay(dt) {
				continue
			}
			var cnt int64
			_ = DB.Model(&DayShiftAssignment{}).Where("date = ?", dt.Format("2006-01-02")).Count(&cnt).Error
			if cnt == 0 {
				_ = DB.Create(&DayShiftAssignment{Date: time.Date(dt.Year(), dt.Month(), dt.Day(), 0, 0, 0, 0, dt.Location()), CycleID: next.ID, Status: "assigned"}).Error
			}
		}
	}
	return nil
}

// Day shift cycle helpers
func getCurrentCycle(d time.Time) (*DayShiftCycle, error) {
	var cyc DayShiftCycle
	if err := DB.Where("begin_date <= ? AND finish_date >= ?", d.Format("2006-01-02"), d.Format("2006-01-02")).First(&cyc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &cyc, nil
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func prevBusinessDay(d time.Time) time.Time {
	x := d.Add(-24 * time.Hour)
	for i := 0; i < 10; i++ { // search up to 10 days back
		if isBusinessDay(x) {
			return dateOnly(x)
		}
		x = x.Add(-24 * time.Hour)
	}
	return time.Time{}
}

func nextBusinessDay(d time.Time) time.Time {
	x := d.Add(24 * time.Hour)
	for i := 0; i < 30; i++ { // search up to 30 days forward
		if isBusinessDay(x) {
			return dateOnly(x)
		}
		x = x.Add(24 * time.Hour)
	}
	return time.Time{}
}

func listActiveUsers() ([]User, error) {
	var users []User
	if err := DB.Where("is_deleted = 0 AND is_day_shift_active = 1").Order("id ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// returns list of 2-user pairs represented by user IDs; drops last if odd
func computePairs(users []User) [][]uint {
	out := [][]uint{}
	// if odd number, drop last and log warning
	n := len(users)
	if n < 2 {
		return out
	}
	if n%2 == 1 {
		log.Printf("[DayShift] odd number of active users (%d), dropping last (id=%d)", n, users[n-1].ID)
		n = n - 1
	}
	for i := 0; i < n; i += 2 {
		out = append(out, []uint{users[i].ID, users[i+1].ID})
	}
	return out
}

// count business days from start to end inclusive
func businessDaysBetween(start time.Time, end time.Time) int {
	s := dateOnly(start)
	e := dateOnly(end)
	cnt := 0
	for d := s; !d.After(e); d = d.Add(24 * time.Hour) {
		if isBusinessDay(d) {
			cnt++
		}
	}
	if cnt > 0 {
		return cnt - 1
	} // zero-based index
	return 0
}
