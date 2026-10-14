package gqlH

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminResolverAuthorization(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin-authorization?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.RoleUser{}, &domain.Subject{}, &legal.Consent{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&legal.Consent{UserID: 1, TermsVersion: legal.Version, PrivacyVersion: legal.Version, SecurityVersion: legal.Version, DisplayLanguage: "ja"}).Error; err != nil {
		t.Fatal(err)
	}
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	var actor *domain.User
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if actor != nil {
			c.Locals("user", actor)
		}
		return c.Next()
	})
	NewHandler(db, rdb, nil).RegisterGraphQL(app.Group("/graphql"))

	query := "query { adminSubjects { id } }"
	mutation := `mutation { adminSubjectCreate(name_ja: "test") { id } }`
	adminOnly := `mutation { adminSetConfig(key: "test", value: "x") }`
	completionDefault := `mutation { adminSetTaskCompletionDefault(type: "QUIZ", is_completion_disabled: true) }`
	tests := []struct {
		name, role, operation, wantError string
		wantStatus                       int
	}{
		{"anonymous task default", "", completionDefault, "unauthorized", 401},
		{"user task default", "user", completionDefault, "forbidden", 400},
		{"manager task default", "manager", completionDefault, "forbidden", 400},
		{"invalid task default type", "admin", `mutation { adminSetTaskCompletionDefault(type: "invalid", is_completion_disabled: true) }`, "invalid task type", 400},
		{"anonymous query", "", query, "unauthorized", 401},
		{"anonymous mutation", "", mutation, "unauthorized", 401},
		{"user query", "user", query, "forbidden", 400},
		{"user mutation", "user", mutation, "forbidden", 400},
		{"manager query", "manager", query, "", 200},
		{"manager mutation", "manager", mutation, "", 200},
		{"manager admin only", "manager", adminOnly, "forbidden", 400},
		{"admin query", "admin", query, "", 200},
		{"admin mutation", "admin", mutation, "", 200},
		{"admin admin only", "admin", `mutation { adminSetConfig(key: "", value: "x") }`, "invalid key", 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actor = nil
			if tt.role != "" {
				actor = &domain.User{ID: 1}
				if err := db.Where("user_id = ?", actor.ID).Delete(&domain.RoleUser{}).Error; err != nil {
					t.Fatal(err)
				}
				if tt.role != "user" {
					if err := db.Create(&domain.RoleUser{UserID: actor.ID, Role: tt.role}).Error; err != nil {
						t.Fatal(err)
					}
				}
			}
			assertAdminGraphQLResult(t, app, tt.operation, tt.wantStatus, tt.wantError)
		})
	}

	actor = &domain.User{ID: 1}
	if err := db.Migrator().DropTable(&domain.RoleUser{}); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{query, mutation, adminOnly, completionDefault} {
		t.Run("role lookup failure "+operation, func(t *testing.T) {
			assertAdminGraphQLResult(t, app, operation, 400, "authorization:")
		})
	}
}

func assertAdminGraphQLResult(t *testing.T, app *fiber.App, operation string, wantStatus int, wantError string) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"query": operation})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/graphql/", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != wantStatus {
		t.Fatalf("status = %d, want %d; errors = %+v", res.StatusCode, wantStatus, result.Errors)
	}
	if wantError == "" {
		if len(result.Errors) != 0 {
			t.Fatalf("unexpected errors: %+v", result.Errors)
		}
		return
	}
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[0].Message, wantError) {
		t.Fatalf("errors = %+v, want %q", result.Errors, wantError)
	}
}
