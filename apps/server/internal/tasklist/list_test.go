package tasklist

import (
	"testing"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/utils"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListVisibilityAndCursor(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:tasklist?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE tasks (
		id integer primary key, added_user_id integer, title text, subject_id integer,
		description text, type text, is_require_hand_in boolean, is_self_check_required boolean,
		is_hidden boolean, deadline date, effective_deadline date, deadline_since text,
		deadline_until text, created_at datetime, updated_at datetime
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE role_users (id integer primary key, user_id integer, role text)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE permissions (id integer primary key, user_id integer, target text, method text, value text)").Error; err != nil {
		t.Fatal(err)
	}
	deadline := utils.DateOnly(time.Now().AddDate(0, 0, 1))
	for _, row := range []struct {
		id     uint
		owner  uint
		hidden bool
	}{{1, 1, false}, {2, 1, true}, {3, 1, false}, {4, 2, false}} {
		if err := db.Exec(`INSERT INTO tasks (id, added_user_id, title, subject_id, is_hidden,
			deadline, effective_deadline, deadline_since, deadline_until)
			VALUES (?, ?, 'task', 1, ?, ?, ?, 'NONE', 'NONE')`, row.id, row.owner, row.hidden, deadline.Format("2006-01-02"), deadline.Format("2006-01-02")).Error; err != nil {
			t.Fatal(err)
		}
	}
	items, cursor, err := List(db, 2, Options{Limit: 1})
	if err != nil || len(items) != 1 || items[0].ID != 1 || cursor == nil || cursor.CursorDeadline != deadline.Format("2006-01-02") {
		t.Fatalf("first page: items=%v cursor=%+v err=%v", items, cursor, err)
	}
	items, _, err = List(db, 2, Options{Limit: 1, CursorDeadline: cursor.CursorDeadline, CursorID: cursor.CursorID})
	if err != nil || len(items) != 1 || items[0].ID != 3 {
		t.Fatalf("second page: items=%v err=%v", items, err)
	}
	if err := db.Exec("INSERT INTO role_users (user_id, role) VALUES (3, 'admin')").Error; err != nil {
		t.Fatal(err)
	}
	items, _, err = List(db, 3, Options{Limit: 10})
	if err != nil || len(items) != 4 {
		t.Fatalf("admin list: items=%v err=%v", items, err)
	}
	ownerID := uint(1)
	mcpItems, _, err := List(db, 1, Options{MCP: true, OwnerID: &ownerID, Limit: 10})
	if err != nil || len(mcpItems) != 2 || mcpItems[0].ID != 3 || mcpItems[1].ID != 1 {
		t.Fatalf("MCP owner list: items=%v err=%v", mcpItems, err)
	}
	mcpItems, _, err = List(db, 1, Options{MCP: true, OwnerID: &ownerID, Limit: 1, Offset: 1})
	if err != nil || len(mcpItems) != 1 || mcpItems[0].ID != 1 {
		t.Fatalf("MCP owner page: items=%v err=%v", mcpItems, err)
	}
	if _, _, err := List(db, 2, Options{CursorDeadline: "bad", CursorID: "1"}); err == nil {
		t.Fatal("invalid cursor must be rejected")
	}
	visible := items[0]
	hidden := items[1]
	if allowed, err := CanAccessTask(db, 1, "update", visible); err != nil || !allowed {
		t.Fatalf("owner must update visible task: allowed=%v err=%v", allowed, err)
	}
	if allowed, err := CanAccessTask(db, 2, "update", visible); err != nil || allowed {
		t.Fatalf("other user must not update task: allowed=%v err=%v", allowed, err)
	}
	if allowed, err := CanAccessTask(db, 1, "update", hidden); err != nil || allowed {
		t.Fatalf("owner must not update hidden task: allowed=%v err=%v", allowed, err)
	}
	if allowed, err := CanAccessTask(db, 3, "delete", hidden); err != nil || !allowed {
		t.Fatalf("admin must manage hidden task: allowed=%v err=%v", allowed, err)
	}
}

func TestNormalizeDeadlineWindow(t *testing.T) {
	since, until, err := NormalizeDeadlineWindow("p10", "after_school")
	if err != nil || since != "P10" || until != "AFTER_SCHOOL" {
		t.Fatalf("normalized window = %q, %q, %v", since, until, err)
	}
	for _, pair := range [][2]string{{"MORNING", "NONE"}, {"P11", "NONE"}, {"NONE", "P11"}} {
		if _, _, err := NormalizeDeadlineWindow(pair[0], pair[1]); err == nil {
			t.Fatalf("invalid window accepted: %v", pair)
		}
	}
}
