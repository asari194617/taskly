package gqlH

import (
	"fmt"
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

func TestGraphQLRequiresCurrentConsent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:graphql-consent?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.User{}, &domain.RoleUser{}, &legal.Consent{}); err != nil {
		t.Fatal(err)
	}
	u := domain.User{Email: "consent@example.test", IsAgreedPolicies: true}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", &u); return c.Next() })
	NewHandler(db, rdb, nil).RegisterGraphQL(app.Group("/graphql"))
	request := func(query string) int {
		t.Helper()
		req := httptest.NewRequest("POST", "/graphql/", strings.NewReader(fmt.Sprintf(`{"query":%q}`, query)))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if got := request("query { subjects { id } }"); got != 403 {
		t.Fatalf("protected query before consent = %d", got)
	}
	if got := request("query { me { is_agreed_policies } }"); got != 200 {
		t.Fatalf("self query before consent = %d", got)
	}
	if got := request(`mutation { agreePolicies(terms_version:"old",privacy_version:"2026-09-22.1",security_version:"2026-09-22.1",display_language:"ja") }`); got != 400 {
		t.Fatalf("stale consent = %d", got)
	}
	if got := request(fmt.Sprintf(`mutation { agreePolicies(terms_version:%q,privacy_version:%q,security_version:%q,display_language:"ja") }`, legal.Version, legal.Version, legal.Version)); got != 200 {
		t.Fatalf("current consent = %d", got)
	}
}
