package guidancebridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	"github.com/asari194617/taskly/apps/server/internal/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func delegatedCompletionCount(t *testing.T, f *fixture) int64 {
	t.Helper()
	var count int64
	if err := f.db.Table("tasks_users_completions").Where("user_id = ?", f.user.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func TestMySQLGuidanceDelegatedScopeAndParentIsolation(t *testing.T) {
	requireMySQL(t)
	for _, tc := range []struct {
		name, method, scope string
		claims              map[string]any
		expected            int
	}{
		{"read cannot mutate", "PUT", "guidance:tasks:read", nil, 403},
		{"write cannot enumerate", "GET", "guidance:tasks:write", nil, 403},
		{"wrong integration audience", "PUT", "guidance:tasks:write", map[string]any{"aud": "https://api.taskly.example.test/integrations/another/tasks"}, 401},
		{"wrong parent resource", "PUT", "guidance:tasks:write", map[string]any{"original_resource": "https://another.example.test/mcp"}, 401},
		{"direct MCP parent token", "PUT", "guidance:tasks:write", map[string]any{"client_id": "nnct-guidance-chatgpt", "aud": "https://api.guidance.example.test/mcp"}, 401},
		{"mixed read and write", "PUT", "guidance:tasks:read guidance:tasks:write", nil, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDelegatedFixture(t, true, tc.scope)
			for key, value := range tc.claims {
				f.identity[key] = value
			}
			key, body := "", ""
			if tc.method == "PUT" {
				key, body = "visit-survey-2", pairBody(true, scopeA)
			}
			code, data := f.request(t, tc.method, key, body, nil)
			if code != tc.expected {
				t.Fatalf("rejected delegated request status%d: %s", code, data)
			}
			if delegatedCompletionCount(t, f) != 0 {
				t.Fatal("rejected authorization changed canonical completions")
			}
			assertRow(t, f, "assigned-visit-2", false, "")
			assertRow(t, f, "visit-survey-2", false, "")
		})
	}
}

func TestMySQLGuidanceDelegatedCanonicalPairAndManualOwnership(t *testing.T) {
	requireMySQL(t)
	f := newDelegatedFixture(t, true, "guidance:tasks:write")
	state := pairRequest(t, f, true, scopeA, 200)
	if !state.Completed || !state.RelatedTasks[0].Completed || state.RelatedTasks[0].AutoCompletedBy != "visit-survey-2" {
		t.Fatalf("missing canonical linked state: %+v", state)
	}
	assertRow(t, f, "assigned-visit-2", true, scopeA)
	assertRow(t, f, "visit-survey-2", true, "")
	f.identity["scope"] = "guidance:tasks:read"
	code, data := f.request(t, "GET", "", "", nil)
	if code != 200 || !bytes.Contains(data, []byte(`"auto_completed_by":"visit-survey-2"`)) {
		t.Fatalf("delegated canonical read %d: %s", code, data)
	}
	f.identity["scope"] = "guidance:tasks:write"
	reverse := reverseRequest(t, f, scopeA, 200)
	if reverse.Completed || reverse.RelatedTasks[0].Completed {
		t.Fatalf("automatic reverse did not undo both: %+v", reverse)
	}
	assertRow(t, f, "assigned-visit-2", false, "")
	assertRow(t, f, "visit-survey-2", false, "")
	pairRequest(t, f, true, scopeA, 200)
	if err := domain.CompleteTask(f.db, f.user.ID, f.task.ID); err != nil {
		t.Fatal(err)
	}
	assertRow(t, f, "assigned-visit-2", true, "")
	pairRequest(t, f, false, scopeA, 200)
	assertRow(t, f, "assigned-visit-2", true, "")
	assertRow(t, f, "visit-survey-2", false, "")
}

func TestMySQLGuidanceDelegatedSubjectIsolation(t *testing.T) {
	requireMySQL(t)
	f := newDelegatedFixture(t, true, "guidance:tasks:write")
	pairRequest(t, f, true, scopeA, 200)
	other := domain.User{Email: "delegated-other@example.test", DisplayName: "Other synthetic"}
	if err := f.db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	if err := legal.Accept(f.db, other.ID, legal.Version, legal.Version, legal.Version, "ja"); err != nil {
		t.Fatal(err)
	}
	if err := domain.CompleteTask(f.db, other.ID, f.task.ID); err != nil {
		t.Fatal(err)
	}
	f.identity["sub"] = strconv.FormatUint(uint64(other.ID), 10)
	f.identity["scope"] = "guidance:tasks:read"
	code, body := f.request(t, "GET", "", "", nil)
	if code != 200 {
		t.Fatalf("other subject read%d: %s", code, body)
	}
	var result struct {
		Tasks []taskState `json:"tasks"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	for _, row := range result.Tasks {
		if row.Key == "assigned-visit-2" && (!row.Completed || row.AutoCompletedBy != "") {
			t.Fatalf("other subject inherited ownership %+v", row)
		}
		if row.Key == "visit-survey-2" && row.Completed {
			t.Fatal("other subject inherited survey completion")
		}
	}
	f.identity["scope"] = "guidance:tasks:write"
	pairRequest(t, f, false, scopeA, 200)
	assertRow(t, f, "assigned-visit-2", true, scopeA)
	assertRow(t, f, "visit-survey-2", true, "")
	var otherMarks int64
	if err := f.db.Table("tasks_users_completions").Where("user_id = ? AND task_id = ?", other.ID, f.task.ID).Count(&otherMarks).Error; err != nil || otherMarks != 1 {
		t.Fatalf("other manual completion lost count%d err%v", otherMarks, err)
	}
}

func TestMySQLGuidanceDelegatedLocalPolicyAndAtomicRejection(t *testing.T) {
	requireMySQL(t)
	for _, tc := range []struct {
		name, statement string
		args            func(*fixture) []any
		expected        int
	}{
		{"missing consent", "DELETE FROM policy_consents WHERE user_id = ?", func(f *fixture) []any { return []any{f.user.ID} }, 403},
		{"blocked account", "INSERT INTO blocks(user_id) VALUES (?)", func(f *fixture) []any { return []any{f.user.ID} }, 403},
		{"denied task read", "INSERT INTO permissions(user_id,target,method,value) VALUES (?,'tasks','read','none')", func(f *fixture) []any { return []any{f.user.ID} }, 403},
		{"hidden linked visit", "UPDATE tasks SET is_hidden=1 WHERE id = ?", func(f *fixture) []any { return []any{f.task.ID} }, 403},
		{"disabled linked visit", "UPDATE tasks SET is_completion_disabled=1 WHERE id = ?", func(f *fixture) []any { return []any{f.task.ID} }, 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDelegatedFixture(t, true, "guidance:tasks:write")
			if err := f.db.Exec(tc.statement, tc.args(f)...).Error; err != nil {
				t.Fatal(err)
			}
			code, data := f.request(t, "PUT", "visit-survey-2", pairBody(true, scopeA), nil)
			if code != tc.expected {
				t.Fatalf("policy status%d want%d: %s", code, tc.expected, data)
			}
			if delegatedCompletionCount(t, f) != 0 {
				t.Fatal("rejected pair left a partial canonical completion")
			}
		})
	}
	t.Run("late transaction failure", func(t *testing.T) {
		f := newDelegatedFixture(t, true, "guidance:tasks:write")
		written := false
		afterName, queryName := "test:delegated-created", "test:delegated-final-read"
		if err := f.db.Callback().Create().After("gorm:create").Register(afterName, func(tx *gorm.DB) {
			if tx.Statement.Table == "tasks_users_completions" {
				written = true
			}
		}); err != nil {
			t.Fatal(err)
		}
		defer f.db.Callback().Create().Remove(afterName)
		if err := f.db.Callback().Query().Before("gorm:query").Register(queryName, func(tx *gorm.DB) {
			if written && tx.Statement.Table == "tasks_users_completions" {
				tx.AddError(errors.New("synthetic delegated transaction failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		defer f.db.Callback().Query().Remove(queryName)
		pairRequest(t, f, true, scopeA, 503)
		written = false
		assertRow(t, f, "assigned-visit-2", false, "")
		assertRow(t, f, "visit-survey-2", false, "")
	})
}

func TestMySQLGuidanceDelegatedBearerCannotAuthenticateGeneralAPI(t *testing.T) {
	requireMySQL(t)
	f := newDelegatedFixture(t, true, "guidance:tasks:write")
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { rdb.Close() })
	gqlH.NewHandler(f.db, rdb, nil).RegisterGraphQL(f.app.Group("/graphql"))
	mw := middleware.NewMiddleware(f.db, rdb, nil)
	f.app.Get("/tasks", mw.AuthRequired, func(c *fiber.Ctx) error {
		t.Error("bridge Bearer reached general REST handler")
		return c.SendStatus(200)
	})
	for _, path := range []string{"/graphql/", "/tasks"} {
		method, body := "GET", ""
		if path == "/graphql/" {
			method, body = "POST", `{"query":"{ tasks { items { id } } }"}`
		}
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer synthetic-access")
		req.Header.Set("Content-Type", "application/json")
		res, err := f.app.Test(req, 10000)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("general API %s status%d: %s", path, res.StatusCode, data)
		}
	}
	if delegatedCompletionCount(t, f) != 0 {
		t.Fatal("general API rejection changed completions")
	}
	// The same opaque Bearer remains valid only on its delegated integration route.
	state := pairRequest(t, f, true, scopeA, 200)
	if !state.Completed {
		t.Fatal(fmt.Sprintf("delegated integration failed: %+v", state))
	}
}
