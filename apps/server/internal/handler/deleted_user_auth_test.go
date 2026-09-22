package handler

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	graphqlHandler "github.com/asari194617/taskly/apps/server/internal/handler/graphql"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	"github.com/asari194617/taskly/apps/server/internal/middleware"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDeletedUserCredentialsAreRejected(t *testing.T) {
	for _, deletion := range []string{"rest self", "graphql self", "graphql admin"} {
		t.Run(deletion, func(t *testing.T) {
			mr, err := miniredis.Run()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(mr.Close)
			rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = rdb.Close() })
			dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
			db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&domain.User{}, &domain.Session{}, &domain.APIKey{}, &domain.IcalKey{}, &domain.MCP{}, &domain.Block{}, &domain.RoleUser{}, &domain.Log{}, &legal.Consent{}); err != nil {
				t.Fatal(err)
			}
			user := domain.User{Email: "deleted@example.test"}
			if err := db.Create(&user).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&legal.Consent{UserID: user.ID, TermsVersion: legal.Version, PrivacyVersion: legal.Version, SecurityVersion: legal.Version, DisplayLanguage: "ja"}).Error; err != nil {
				t.Fatal(err)
			}
			const session, apiKey, mcpToken = "old-session", "old-api-key", "old-mcp-token"
			if err := db.Create(&domain.Session{UserID: user.ID, HashedToken: utils.Sha256Hex([]byte(session)), ClientPlatform: "ios"}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&domain.APIKey{UserID: user.ID, HashedToken: utils.Sha256Hex([]byte(apiKey)), IsValid: true}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&domain.MCP{UserID: user.ID, HashedToken: utils.Sha256Hex([]byte(mcpToken))}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&domain.IcalKey{UserID: user.ID, Token: "old-ical-token"}).Error; err != nil {
				t.Fatal(err)
			}
			mr.Set("session:"+session, fmt.Sprint(user.ID))
			_, _ = mr.SAdd(fmt.Sprintf("user_sessions:%d", user.ID), session)

			mw := middleware.NewMiddleware(db, rdb, nil)
			app := fiber.New()
			h := NewHandler(db, rdb, nil)
			h.RegisterUsersRoutes(app.Group("/users", mw.AuthRequired))
			graphqlHandler.NewHandler(db, rdb, nil).RegisterGraphQL(app.Group("/graphql"))
			app.Get("/protected", mw.AuthRequired, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
			status := func(header, value string) int {
				req := httptest.NewRequest(fiber.MethodGet, "/protected", nil)
				req.Header.Set(header, value)
				res, err := app.Test(req)
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				return res.StatusCode
			}
			credentials := []struct{ header, value string }{
				{"Authorization", "Bearer " + session},
				{"X-API-Key", apiKey},
				{"X-MCP-Token", mcpToken},
			}
			for _, credential := range credentials {
				if got := status(credential.header, credential.value); got != fiber.StatusNoContent {
					t.Fatalf("%s before deletion: %d", credential.header, got)
				}
			}
			// A database-backed session must also authenticate before deletion.
			mr.Del("session:" + session)
			if got := status("Authorization", "Bearer "+session); got != fiber.StatusNoContent {
				t.Fatalf("database session before deletion: %d", got)
			}
			switch deletion {
			case "rest self":
				req := httptest.NewRequest(fiber.MethodDelete, "/users/me", nil)
				req.Header.Set("Authorization", "Bearer "+session)
				res, err := app.Test(req)
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				if res.StatusCode != fiber.StatusNoContent {
					t.Fatalf("REST deletion: %d", res.StatusCode)
				}
			case "graphql self", "graphql admin":
				actorToken := session
				query := "mutation { deleteMe }"
				if deletion == "graphql admin" {
					admin := domain.User{Email: "admin@example.test"}
					if err := db.Create(&admin).Error; err != nil {
						t.Fatal(err)
					}
					if err := db.Create(&legal.Consent{UserID: admin.ID, TermsVersion: legal.Version, PrivacyVersion: legal.Version, SecurityVersion: legal.Version, DisplayLanguage: "ja"}).Error; err != nil {
						t.Fatal(err)
					}
					if err := db.Create(&domain.RoleUser{UserID: admin.ID, Role: "admin"}).Error; err != nil {
						t.Fatal(err)
					}
					actorToken = "admin-session"
					mr.Set("session:"+actorToken, fmt.Sprint(admin.ID))
					if err := db.Create(&domain.Session{UserID: admin.ID, HashedToken: utils.Sha256Hex([]byte(actorToken)), ClientPlatform: "ios"}).Error; err != nil {
						t.Fatal(err)
					}
					query = fmt.Sprintf("mutation { adminUserDelete(user_id: %d) }", user.ID)
				}
				rw, body, err := doJSONRequest(app, fiber.MethodPost, "/graphql/", map[string]string{"query": query}, map[string]string{"Authorization": "Bearer " + actorToken})
				if err != nil || rw.Code != fiber.StatusOK || !strings.Contains(string(body), "true") {
					t.Fatalf("GraphQL deletion: status=%d err=%v body=%s", rw.Code, err, body)
				}
			}
			for _, credential := range credentials {
				if got := status(credential.header, credential.value); got != fiber.StatusUnauthorized {
					t.Fatalf("%s after deletion: %d", credential.header, got)
				}
			}
			var closed domain.User
			if err := db.First(&closed, user.ID).Error; err != nil {
				t.Fatal(err)
			}
			if !closed.IsDeleted || closed.DeletedAt == nil {
				t.Fatalf("closure time missing: deleted=%t at=%v", closed.IsDeleted, closed.DeletedAt)
			}
			var sessions, keys, tokens, icalKeys int64
			if err := db.Model(&domain.Session{}).Where("user_id = ?", user.ID).Count(&sessions).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&domain.APIKey{}).Where("user_id = ? AND is_valid = 1", user.ID).Count(&keys).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&domain.MCP{}).Where("user_id = ?", user.ID).Count(&tokens).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&domain.IcalKey{}).Where("user_id = ?", user.ID).Count(&icalKeys).Error; err != nil {
				t.Fatal(err)
			}
			if sessions != 0 || keys != 0 || tokens != 0 || icalKeys != 0 || mr.Exists("session:"+session) {
				t.Fatalf("credentials remain: sessions=%d keys=%d mcp=%d ical=%d redis=%t", sessions, keys, tokens, icalKeys, mr.Exists("session:"+session))
			}
			// A stale cache entry or credential must still fail the user-state check.
			mr.Set("session:"+session, fmt.Sprint(user.ID))
			if err := db.Model(&domain.APIKey{}).Where("user_id = ?", user.ID).Update("is_valid", true).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&domain.MCP{UserID: user.ID, HashedToken: utils.Sha256Hex([]byte(mcpToken))}).Error; err != nil {
				t.Fatal(err)
			}
			for _, credential := range credentials {
				if got := status(credential.header, credential.value); got != fiber.StatusUnauthorized {
					t.Fatalf("%s with stale credential: %d", credential.header, got)
				}
			}
		})
	}
}
