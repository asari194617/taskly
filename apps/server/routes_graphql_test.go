package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"net/http/httptest"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	gorm_sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestApp initializes in-memory DB/redis and returns a Fiber app with /graphql mounted plus a test user/session token.
func setupTestApp(t *testing.T) (*fiber.App, string) {
	t.Helper()
	// In-memory sqlite
	var err error
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	DB, err = gorm.Open(gorm_sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := DB.AutoMigrate(&User{}, &Session{}, &RoleUser{}, &Block{}, &APIKey{}, &Subject{} /* omit Task for sqlite */, &ApiActionLog{}, &TOTPSecret{}, &RecoveryCode{}, &Passkey{}, &LoginCode{}, &DayShiftCycle{}, &DayShiftAssignment{}, &DayShiftAssignmentUser{}, &BusinessCalendar{}, &IcalKey{}, &DayShiftCyclePair{}, &MCP{}, &Log{}, &DayShiftCycleGroup{}, &DayShiftCycleGroupMember{}, &Migration{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// miniredis for sessions
	mr := miniredis.RunT(t)
	RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// seed admin user and session token
	u := User{Email: "admin@example.com", DisplayName: "Admin", NotificationLang: "ja"}
	if err := DB.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := DB.Create(&RoleUser{UserID: u.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("seed role: %v", err)
	}
	token := "test_token"
	if err := RDB.Set(context.Background(), "session:"+token, fmt.Sprintf("%d", u.ID), time.Hour).Err(); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	app := fiber.New()
	registerGraphQL(app)
	return app, token
}

func doGraphQL(t *testing.T, app *fiber.App, token string, query string, vars map[string]any) map[string]any {
	t.Helper()
	body := map[string]any{"query": query, "variables": vars}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/graphql", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Client-Platform", "web")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	rb, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		t.Fatalf("http %d: %s", resp.StatusCode, string(rb))
	}
	var out struct {
		Data   map[string]any
		Errors []map[string]any
	}
	_ = json.Unmarshal(rb, &out)
	if len(out.Errors) > 0 {
		t.Fatalf("graphql returned errors: %v", out.Errors)
	}
	return out.Data
}

func TestAdminUsersByIds(t *testing.T) {
	app, tok := setupTestApp(t)
	// create two users
	u1 := User{Email: "a@ex.com", DisplayName: "A"}
	u2 := User{Email: "b@ex.com", DisplayName: "B"}
	if err := DB.Create(&u1).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&u2).Error; err != nil {
		t.Fatal(err)
	}
	q := `query($ids:[Int!]!){ adminUsersByIds(ids:$ids){ id email display_name } }`
	d := doGraphQL(t, app, tok, q, map[string]any{"ids": []int{int(u1.ID), int(u2.ID)}})
	arr := d["adminUsersByIds"].([]any)
	if len(arr) != 2 {
		t.Fatalf("expected 2, got %d", len(arr))
	}
}

func TestDayshiftGroupMutations(t *testing.T) {
	app, tok := setupTestApp(t)
	// seed cycle and users
	cyc := DayShiftCycle{BeginDate: time.Now(), FinishDate: time.Now().AddDate(0, 0, 10), Description: "test"}
	if err := DB.Create(&cyc).Error; err != nil {
		t.Fatal(err)
	}
	u1 := User{Email: "u1@ex.com", DisplayName: "U1"}
	u2 := User{Email: "u2@ex.com", DisplayName: "U2"}
	u3 := User{Email: "u3@ex.com", DisplayName: "U3"}
	_ = DB.Create(&u1).Error
	_ = DB.Create(&u2).Error
	_ = DB.Create(&u3).Error

	// create two groups
	mutCreate := `mutation($cid:Int!,$ids:[Int!]!){ adminDayshiftCreateGroup(cycle_id:$cid,user_ids:$ids){ id } }`
	d1 := doGraphQL(t, app, tok, mutCreate, map[string]any{"cid": int(cyc.ID), "ids": []int{int(u1.ID), int(u2.ID)}})
	g1 := int(d1["adminDayshiftCreateGroup"].(map[string]any)["id"].(float64))
	_ = g1
	d2 := doGraphQL(t, app, tok, mutCreate, map[string]any{"cid": int(cyc.ID), "ids": []int{int(u3.ID)}})
	g2 := int(d2["adminDayshiftCreateGroup"].(map[string]any)["id"].(float64))

	// reorder groups
	mutReorder := `mutation($cid:Int!,$ids:[Int!]!){ adminDayshiftReorderGroups(cycle_id:$cid, group_ids:$ids) }`
	_ = doGraphQL(t, app, tok, mutReorder, map[string]any{"cid": int(cyc.ID), "ids": []int{g2, g1}})
	var g DayShiftCycleGroup
	if err := DB.First(&g, g2).Error; err != nil {
		t.Fatal(err)
	}
	if g.Position != 1 {
		t.Fatalf("expected g2 at position 1, got %d", g.Position)
	}

	// update members for group 2 (set to only u2)
	mutUpdate := `mutation($gid:Int!,$ids:[Int!]!){ adminDayshiftUpdateGroupMembers(group_id:$gid,user_ids:$ids) }`
	_ = doGraphQL(t, app, tok, mutUpdate, map[string]any{"gid": g2, "ids": []int{int(u2.ID)}})
	var mems []DayShiftCycleGroupMember
	_ = DB.Where("group_id = ?", g2).Order("position asc").Find(&mems).Error
	if len(mems) != 1 || mems[0].UserID != u2.ID {
		t.Fatalf("unexpected members: %+v", mems)
	}
}
