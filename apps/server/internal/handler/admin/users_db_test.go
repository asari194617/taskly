package adminHandler

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRoleGrantDoesNotReportDatabaseFailureAsSuccess(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.RoleUser{}); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	NewAdminHandler(db, nil, nil).RegisterAdminUsersRoutes(app.Group("/admin/users"))
	request := func(want int) {
		t.Helper()
		req := httptest.NewRequest("POST", "/admin/users/7/roles", strings.NewReader(`{"role":"manager"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("got %d, want %d", resp.StatusCode, want)
		}
	}
	request(201)
	if err := db.Exec("DROP TABLE role_users").Error; err != nil {
		t.Fatal(err)
	}
	request(500)
}
