// guidance-e2e starts real Taskly bridge/GraphQL routes with disposable synthetic
// fixtures. This command is never used by the production Docker entrypoint.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"encoding/json"
	"github.com/alicebob/miniredis/v2"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/guidancebridge"
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const eventID = "synthetic-guidance-2030"

var aliases = []string{"alice", "bob", "onboarding", "routing", "map", "outage", "mobile", "documents", "presentation", "refresh", "menu", "home", "home-denied", "home-failure", "home-other"}

type fixture struct {
	EventID  string            `json:"event_id"`
	Users    map[string]uint   `json:"users"`
	Sessions map[string]string `json:"sessions"`
	Bindings map[string]uint   `json:"bindings"`
}

func validateEnvironment(enabled, nodeEnv, addr, dsn, issuer string) (*mysqldriver.Config, error) {
	deny := errors.New("guidance E2E requires explicit opt-in, loopback services, and a disposable taskly_guidance_e2e database")
	if enabled != "1" || strings.EqualFold(strings.TrimSpace(nodeEnv), "production") {
		return nil, deny
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host != "127.0.0.1" {
		return nil, deny
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return nil, deny
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil || cfg.DBName != "taskly_guidance_e2e" || cfg.Net != "tcp" {
		return nil, deny
	}
	dbHost, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil || dbHost != "127.0.0.1" {
		return nil, deny
	}
	u, err := url.Parse(issuer)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return nil, deny
	}
	cfg.ParseTime = true
	cfg.MultiStatements = true
	return cfg, nil
}

func seed(db *gorm.DB) (fixture, error) {
	result := fixture{EventID: eventID, Users: map[string]uint{}, Sessions: map[string]string{}, Bindings: map[string]uint{}}
	err := db.Transaction(func(tx *gorm.DB) error {
		for index, alias := range aliases {
			user := domain.User{ID: uint(1001 + index), Email: alias + "@synthetic.example.test", DisplayName: "Synthetic " + alias, IsRegistered: true}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
			if err := legal.Accept(tx, user.ID, legal.Version, legal.Version, legal.Version, "ja"); err != nil {
				return err
			}
			token := "synthetic-taskly-e2e-" + alias
			expires := time.Now().Add(24 * time.Hour)
			if err := tx.Create(&domain.Session{UserID: user.ID, HashedToken: utils.Sha256Hex([]byte(token)), ExpiresAt: &expires}).Error; err != nil {
				return err
			}
			result.Users[alias] = user.ID
			result.Sessions[alias] = token
			if alias == "home-denied" {
				deny := "none"
				if err := tx.Create(&domain.Permission{UserID: user.ID, Target: "tasks", Method: "read", Value: &deny}).Error; err != nil {
					return err
				}
			}
		}
		subject := domain.Subject{ID: 9001, NameJa: "Synthetic guidance fixture"}
		if err := tx.Create(&subject).Error; err != nil {
			return err
		}
		keys := []string{}
		for _, prefix := range []string{"assigned-visit-", "visit-survey-"} {
			for term := 1; term <= 8; term++ {
				keys = append(keys, prefix+strconv.Itoa(term))
			}
		}
		keys = append(keys, "overall-survey", "sheet-submission")
		for index, key := range keys {
			task := domain.Task{ID: uint(5001 + index), AddedUserID: 1001, Title: "Synthetic " + key, SubjectID: subject.ID, Type: "OTHER", Deadline: time.Date(2030, 5, 14, 0, 0, 0, 0, time.UTC), DeadlineSince: "NONE", DeadlineUntil: "NONE"}
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
			result.Bindings[key] = task.ID
		}
		return nil
	})
	return result, err
}

func run() error {
	addr := os.Getenv("ADDR")
	cfg, err := validateEnvironment(os.Getenv("TASKLY_GUIDANCE_E2E"), os.Getenv("NODE_ENV"), addr, os.Getenv("TASKLY_E2E_MYSQL_DSN"), os.Getenv("AUTH_ISSUER"))
	if err != nil {
		return err
	}
	if os.Getenv("AUTH_CLIENT_ID") == "" || os.Getenv("AUTH_CLIENT_SECRET") == "" {
		return errors.New("synthetic identity client configuration required")
	}
	db, err := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return errors.New("synthetic Taskly DB unavailable")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	var tables int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE()").Scan(&tables).Error; err != nil {
		return err
	}
	if tables != 0 {
		return errors.New("refusing to initialize a nonempty E2E database; recreate the disposable database explicitly")
	}
	schemaPath := os.Getenv("TASKLY_E2E_SCHEMA_PATH")
	if schemaPath == "" {
		schemaPath = "schema.sql"
	}
	ddl, err := os.ReadFile(schemaPath)
	if err != nil {
		return errors.New("synthetic schema file unavailable")
	}
	if err := db.Exec(string(ddl)).Error; err != nil {
		return fmt.Errorf("initialize synthetic schema: %w", err)
	}
	fixtures, err := seed(db)
	if err != nil {
		return fmt.Errorf("seed synthetic fixtures: %w", err)
	}
	bindings, _ := json.Marshal(fixtures.Bindings)
	os.Setenv("NNCT_GUIDANCE_EVENT_ID", eventID)
	os.Setenv("NNCT_GUIDANCE_TASK_BINDINGS", string(bindings))
	os.Setenv("WEB_PUBLIC_URL", "http://"+addr)
	redisServer, err := miniredis.Run()
	if err != nil {
		return err
	}
	defer redisServer.Close()
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer rdb.Close()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/__test/ready", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), time.Second)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			return c.SendStatus(503)
		}
		return c.JSON(fiber.Map{"ready": true})
	})
	app.Get("/__test/fixture", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.JSON(fixtures) })
	guidancebridge.Register(app.Group("/integrations/nnct-guidance"), db)
	gqlH.NewHandler(db, rdb, nil).RegisterGraphQL(app.Group("/graphql"))
	return app.Listen(addr)
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
