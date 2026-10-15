package gqlH

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminUserRestoreAndListing(t *testing.T) {
	t.Setenv("AUTH_ACCOUNT_LIFECYCLE_EXTERNAL", "0")
	db, err := gorm.Open(sqlite.Open("file:admin-restore?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.User{}, &domain.RoleUser{}, &domain.Block{}, &domain.Session{}, &domain.APIKey{}, &domain.IcalKey{}, &domain.MCP{}, &legal.Consent{}, &domain.Log{}); err != nil {
		t.Fatal(err)
	}
	deletedAt := time.Now().UTC()
	for _, row := range []any{
		&domain.User{ID: 1, Email: "admin@example.test"},
		&domain.User{ID: 42, Email: "deleted@example.test", IsDeleted: true, DeletedAt: &deletedAt},
		&domain.RoleUser{UserID: 1, Role: "admin"},
		&legal.Consent{UserID: 1, TermsVersion: legal.Version, PrivacyVersion: legal.Version, SecurityVersion: legal.Version, DisplayLanguage: "ja"},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", &domain.User{ID: 1}); return c.Next() })
	NewHandler(db, rdb, nil).RegisterGraphQL(app.Group("/graphql"))
	query := `query { adminUsers(q:"deleted@example.test"){ id is_deleted } adminUsersByIds(ids:[42]){ id is_deleted } adminUsersPaginated(q:"deleted@example.test"){ items{ id is_deleted } } }`
	assertListing := func(want bool) {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"query": query})
		req := httptest.NewRequest("POST", "/graphql/", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var result struct {
			Data struct {
				Users []struct {
					Deleted bool `json:"is_deleted"`
				} `json:"adminUsers"`
				ByIDs []struct {
					Deleted bool `json:"is_deleted"`
				} `json:"adminUsersByIds"`
				Paginated struct {
					Items []struct {
						Deleted bool `json:"is_deleted"`
					} `json:"items"`
				} `json:"adminUsersPaginated"`
			} `json:"data"`
			Errors []any `json:"errors"`
		}
		if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if len(result.Errors) != 0 || len(result.Data.Users) != 1 || len(result.Data.ByIDs) != 1 || len(result.Data.Paginated.Items) != 1 {
			t.Fatalf("result=%+v", result)
		}
		if result.Data.Users[0].Deleted != want || result.Data.ByIDs[0].Deleted != want || result.Data.Paginated.Items[0].Deleted != want {
			t.Fatalf("listing=%+v want=%v", result.Data, want)
		}
	}
	assertListing(true)
	assertAdminGraphQLResult(t, app, `mutation { adminUserRestore(user_id:42) }`, 200, "")
	assertListing(false)
	var user domain.User
	if err := db.First(&user, 42).Error; err != nil || user.IsDeleted || user.DeletedAt != nil {
		t.Fatalf("user=%+v error=%v", user, err)
	}
	assertAdminGraphQLResult(t, app, `mutation { adminUserRestore(user_id:42) }`, 200, "")
	assertAdminGraphQLResult(t, app, `mutation { adminUserRestore(user_id:999) }`, 400, "user not found")
	assertAdminGraphQLResult(t, app, `mutation { adminUserDelete(user_id:42, hard:true) }`, 200, "")
	assertAdminGraphQLResult(t, app, `mutation { adminUserRestore(user_id:42) }`, 400, "user not found")
}
