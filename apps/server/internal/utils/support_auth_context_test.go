package utils

import (
	"context"
	"net"
	"reflect"
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/valyala/fasthttp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type sessionContextHook struct {
	request  context.Context
	t        *testing.T
	commands []string
}

func (h *sessionContextHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(context.Context, string, string) (net.Conn, error) {
		h.t.Error("context test unexpectedly attempted a Redis connection")
		return nil, context.Canceled
	}
}

func (h *sessionContextHook) ProcessHook(redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if ctx != h.request || ctx.Err() != context.Canceled {
			h.t.Errorf("%s lost the canceled request context", cmd.Name())
		}
		h.commands = append(h.commands, cmd.Name())
		if command, ok := cmd.(*redis.StringSliceCmd); ok {
			command.SetVal([]string{"current", "other"})
		}
		return nil
	}
}

func (h *sessionContextHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestSessionRedisOperationsKeepRequestCancellation(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*testing.T, *fiber.Ctx, *gorm.DB, *redis.Client)
		want []string
	}{
		{"revoke", func(t *testing.T, c *fiber.Ctx, db *gorm.DB, rdb *redis.Client) {
			if err := RevokeSession(c, "token", 1, db, rdb); err != nil {
				t.Fatal(err)
			}
		}, []string{"del", "srem"}},
		{"revoke-others", func(t *testing.T, c *fiber.Ctx, db *gorm.DB, rdb *redis.Client) {
			if err := RevokeOtherSessions(c, 1, "current", db, rdb); err != nil {
				t.Fatal(err)
			}
		}, []string{"smembers", "del", "srem"}},
		{"missing-session", func(t *testing.T, c *fiber.Ctx, db *gorm.DB, rdb *redis.Client) {
			c.Request().Header.Set("Authorization", "Bearer missing")
			if UserFromSession(c, db, rdb) {
				t.Error("missing session authenticated")
			}
		}, []string{"del"}},
		{"persisted-session", func(t *testing.T, c *fiber.Ctx, db *gorm.DB, rdb *redis.Client) {
			if err := db.Create(&domain.User{ID: 1, Email: "context@example.test"}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&domain.Session{UserID: 1, HashedToken: Sha256Hex([]byte("token"))}).Error; err != nil {
				t.Fatal(err)
			}
			c.Request().Header.Set("Authorization", "Bearer token")
			if !UserFromSession(c, db, rdb) {
				t.Error("persisted session did not authenticate")
			}
		}, []string{"set"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&domain.Session{}, &domain.User{}, &domain.Block{}); err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			app := fiber.New()
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(c)
			c.SetUserContext(ctx)
			hook := &sessionContextHook{request: ctx, t: t}
			rdb := redis.NewClient(&redis.Options{Addr: "unused.invalid:0"})
			t.Cleanup(func() { _ = rdb.Close() })
			rdb.AddHook(hook)
			test.run(t, c, db, rdb)
			if !reflect.DeepEqual(hook.commands, test.want) {
				t.Fatalf("Redis commands=%v, want %v", hook.commands, test.want)
			}
		})
	}
}
