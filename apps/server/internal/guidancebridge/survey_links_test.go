package guidancebridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	"gorm.io/gorm"
)

const scopeA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const scopeB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func pairBody(completed bool, scope string) string {
	return fmt.Sprintf(`{"completed":%t,"link_assigned_visit":true,"assignment_scope":%q}`, completed, scope)
}
func requireMySQL(t *testing.T) {
	t.Helper()
	if os.Getenv("TEST_SCHEMA_DB") != "1" {
		t.Skip("requires disposable MySQL schema")
	}
}
func assertRow(t *testing.T, f *fixture, key string, present bool, autoScope string) {
	t.Helper()
	b := bridge{db: f.db}
	row, exists, err := b.record(f.user.ID, f.bindings[key])
	if err != nil || exists != present {
		t.Fatalf("%s exists=%v expected%v err=%v", key, exists, present, err)
	}
	if !present {
		return
	}
	if autoScope == "" {
		if row.AutoEventID != nil || row.AutoSourceTaskID != nil || row.AutoAssignmentScope != nil {
			t.Fatalf("%s not manually owned", key)
		}
		return
	}
	if row.AutoEventID == nil || *row.AutoEventID != "synthetic-event" || row.AutoSourceTaskID == nil || *row.AutoSourceTaskID != f.bindings["visit-survey-2"] || row.AutoAssignmentScope == nil || *row.AutoAssignmentScope != autoScope {
		t.Fatalf("%s ownership mismatch", key)
	}
}
func pairRequest(t *testing.T, f *fixture, completed bool, scope string, expected int) taskState {
	t.Helper()
	status, body := f.request(t, "PUT", "visit-survey-2", pairBody(completed, scope), nil)
	if status != expected {
		t.Fatalf("pair status%d want%d: %s", status, expected, body)
	}
	if status != 200 {
		return taskState{}
	}
	var state taskState
	if err := json.Unmarshal(body, &state); err != nil || len(state.RelatedTasks) != 1 || state.RelatedTasks[0].Key != "assigned-visit-2" {
		t.Fatalf("pair response %s %v", body, err)
	}
	return state
}

func TestSurveyLinkInputAndOwnershipProjection(t *testing.T) {
	f := newFixture(t, false)
	for _, body := range []string{`{"completed":true,"link_assigned_visit":true}`, `{"completed":true,"assignment_scope":"` + scopeA + `"}`, `{"completed":true,"link_assigned_visit":true,"assignment_scope":"bad"}`, `{"completed":true,"link_assigned_visit":true,"assignment_scope":"` + strings.ToUpper(scopeA) + `"}`} {
		if status, _ := f.request(t, "PUT", "visit-survey-2", body, nil); status != 400 {
			t.Fatalf("invalid link%d", status)
		}
	}
	if status, _ := f.request(t, "PUT", "assigned-visit-2", pairBody(true, scopeA), nil); status != 400 {
		t.Fatalf("non-survey%d", status)
	}
	if status, _ := f.request(t, "PUT", "visit-survey-3", pairBody(true, scopeA), nil); status != 404 {
		t.Fatalf("unbound%d", status)
	}
	if err := f.db.Exec("INSERT INTO tasks_users_completions(user_id,task_id,auto_event_id,auto_source_task_id,auto_assignment_scope) VALUES(?,?,?,?,?)", f.user.ID, f.task.ID, "synthetic-event", f.bindings["visit-survey-2"], scopeA).Error; err != nil {
		t.Fatal(err)
	}
	status, body := f.request(t, "GET", "", "", nil)
	if status != 200 || !strings.Contains(string(body), `"auto_completed_by":"visit-survey-2"`) {
		t.Fatalf("projection %d %s", status, body)
	}
	for _, sql := range []string{"UPDATE tasks_users_completions SET auto_event_id='other'", "UPDATE tasks_users_completions SET auto_event_id='synthetic-event',auto_source_task_id=999", "UPDATE tasks_users_completions SET auto_source_task_id=" + fmt.Sprint(f.bindings["visit-survey-2"]) + ",auto_assignment_scope='invalid'"} {
		if err := f.db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
		_, body = f.request(t, "GET", "", "", nil)
		if strings.Contains(string(body), "auto_completed_by") {
			t.Fatal("invalid origin exposed")
		}
	}
}

func TestSurveyUndoRespectsScopeAndManualOwnership(t *testing.T) {
	for _, mode := range []string{"owned", "changed roster", "other event", "other source", "manual"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t, false)
			row := map[string]any{"user_id": f.user.ID, "task_id": f.task.ID}
			scope := scopeA
			if mode != "manual" {
				row["auto_event_id"] = "synthetic-event"
				row["auto_source_task_id"] = f.bindings["visit-survey-2"]
				row["auto_assignment_scope"] = scopeA
			}
			if mode == "other event" {
				row["auto_event_id"] = "other"
			}
			if mode == "other source" {
				row["auto_source_task_id"] = 999
			}
			if mode == "changed roster" {
				scope = scopeB
			}
			if err := f.db.Table("tasks_users_completions").Create(row).Error; err != nil {
				t.Fatal(err)
			}
			result := pairRequest(t, f, false, scope, 200)
			want := mode != "owned"
			if result.RelatedTasks[0].Completed != want {
				t.Fatal("wrong visit state")
			}
			if mode == "changed roster" || mode == "manual" {
				assertRow(t, f, "assigned-visit-2", true, "")
			}
		})
	}
}

func TestSurveyDisabledOrForbiddenPairDoesNotWrite(t *testing.T) {
	for _, statement := range []string{"UPDATE tasks SET is_completion_disabled=1 WHERE title='Synthetic assigned-visit-2'", "UPDATE tasks SET is_completion_disabled=1 WHERE title='Synthetic visit-survey-2'", "UPDATE tasks SET is_hidden=1 WHERE title='Synthetic assigned-visit-2'"} {
		f := newFixture(t, false)
		if err := f.db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
		expected := 409
		if strings.Contains(statement, "hidden") {
			expected = 403
		}
		pairRequest(t, f, true, scopeA, expected)
		assertRow(t, f, "assigned-visit-2", false, "")
		assertRow(t, f, "visit-survey-2", false, "")
	}
}

func TestMySQLGuidanceSurveyLifecycle(t *testing.T) {
	requireMySQL(t)
	f := newFixture(t, true)
	reset := func() {
		t.Helper()
		for _, key := range []string{"assigned-visit-2", "visit-survey-2"} {
			if err := domain.UncompleteTask(f.db, f.user.ID, f.bindings[key]); err != nil {
				t.Fatal(err)
			}
		}
	}
	state := pairRequest(t, f, true, scopeA, 200)
	if !state.Completed || !state.RelatedTasks[0].Completed || state.RelatedTasks[0].AutoCompletedBy != "visit-survey-2" {
		t.Fatal("pair state missing")
	}
	assertRow(t, f, "assigned-visit-2", true, scopeA)
	assertRow(t, f, "visit-survey-2", true, "")
	pairRequest(t, f, true, scopeA, 200)
	assertRow(t, f, "assigned-visit-2", true, scopeA)
	pairRequest(t, f, false, scopeA, 200)
	pairRequest(t, f, false, scopeA, 200)
	assertRow(t, f, "assigned-visit-2", false, "")
	for _, mode := range []string{"preexisting", "native recheck", "guidance recheck", "native undo retry", "native survey prechecked", "scope changed"} {
		t.Run(mode, func(t *testing.T) {
			reset()
			if mode == "preexisting" {
				if err := domain.CompleteTask(f.db, f.user.ID, f.task.ID); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "native survey prechecked" {
				if err := domain.CompleteTask(f.db, f.user.ID, f.bindings["visit-survey-2"]); err != nil {
					t.Fatal(err)
				}
			}
			pairRequest(t, f, true, scopeA, 200)
			switch mode {
			case "native recheck":
				if err := domain.CompleteTask(f.db, f.user.ID, f.task.ID); err != nil {
					t.Fatal(err)
				}
			case "guidance recheck":
				if code, _ := f.request(t, "PUT", "assigned-visit-2", `{"completed":true}`, nil); code != 200 {
					t.Fatalf("manual guidance%d", code)
				}
			case "native undo retry":
				if err := domain.UncompleteTask(f.db, f.user.ID, f.task.ID); err != nil {
					t.Fatal(err)
				}
				pairRequest(t, f, true, scopeA, 200)
				assertRow(t, f, "assigned-visit-2", false, "")
				return
			case "native survey prechecked":
				assertRow(t, f, "assigned-visit-2", false, "")
				return
			case "scope changed":
				pairRequest(t, f, false, scopeB, 200)
				assertRow(t, f, "assigned-visit-2", true, "")
				return
			}
			pairRequest(t, f, false, scopeA, 200)
			assertRow(t, f, "assigned-visit-2", true, "")
		})
	}
}

func TestMySQLGuidanceSurveyRollbackAndIsolation(t *testing.T) {
	requireMySQL(t)
	for _, late := range []bool{false, true} {
		t.Run(fmt.Sprint(late), func(t *testing.T) {
			f := newFixture(t, true)
			written := false
			name := "test:reject-link"
			f.db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
				if tx.Statement.Table == "tasks_users_completions" && !late {
					tx.AddError(errors.New("synthetic write rejected"))
				}
			})
			defer f.db.Callback().Create().Remove(name)
			if late {
				f.db.Callback().Create().After("gorm:create").Register("test:link-created", func(tx *gorm.DB) {
					if tx.Statement.Table == "tasks_users_completions" {
						written = true
					}
				})
				defer f.db.Callback().Create().Remove("test:link-created")
				f.db.Callback().Query().Before("gorm:query").Register("test:reject-link-read", func(tx *gorm.DB) {
					if written && tx.Statement.Table == "tasks_users_completions" {
						tx.AddError(errors.New("synthetic read rejected"))
					}
				})
				defer f.db.Callback().Query().Remove("test:reject-link-read")
			}
			pairRequest(t, f, true, scopeA, 503)
			written = false
			assertRow(t, f, "assigned-visit-2", false, "")
			assertRow(t, f, "visit-survey-2", false, "")
		})
	}
	f := newFixture(t, true)
	pairRequest(t, f, true, scopeA, 200)
	other := domain.User{Email: "other-survey@example.test"}
	if err := f.db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	if err := legal.Accept(f.db, other.ID, legal.Version, legal.Version, legal.Version, "ja"); err != nil {
		t.Fatal(err)
	}
	f.identity["sub"] = fmt.Sprint(other.ID)
	pairRequest(t, f, false, scopeA, 200)
	assertRow(t, f, "assigned-visit-2", true, scopeA)
}

func reverseRequest(t *testing.T, f *fixture, scope string, expected int) taskState {
	t.Helper()
	code, body := f.request(t, "PUT", "assigned-visit-2", pairBody(false, scope), nil)
	if code != expected {
		t.Fatalf("reverse%d want%d: %s", code, expected, body)
	}
	if code != 200 {
		return taskState{}
	}
	var result taskState
	if err := json.Unmarshal(body, &result); err != nil || len(result.RelatedTasks) != 1 || result.RelatedTasks[0].Key != "visit-survey-2" {
		t.Fatalf("reverse response %s %v", body, err)
	}
	return result
}

func TestMySQLGuidanceReverseUndoProtectsBothManualEndpoints(t *testing.T) {
	requireMySQL(t)
	for _, mode := range []string{"automatic", "manual source recheck", "manual target recheck", "manual target before", "scope mismatch", "native source undo"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t, true)
			if mode == "manual target before" {
				if err := domain.CompleteTask(f.db, f.user.ID, f.task.ID); err != nil {
					t.Fatal(err)
				}
			}
			pairRequest(t, f, true, scopeA, 200)
			switch mode {
			case "manual source recheck":
				if err := domain.CompleteTask(f.db, f.user.ID, f.bindings["visit-survey-2"]); err != nil {
					t.Fatal(err)
				}
				assertRow(t, f, "assigned-visit-2", true, "")
			case "manual target recheck":
				if err := domain.CompleteTask(f.db, f.user.ID, f.task.ID); err != nil {
					t.Fatal(err)
				}
			case "native source undo":
				if err := domain.UncompleteTask(f.db, f.user.ID, f.bindings["visit-survey-2"]); err != nil {
					t.Fatal(err)
				}
				assertRow(t, f, "assigned-visit-2", true, "")
			}
			scope := scopeA
			if mode == "scope mismatch" {
				scope = scopeB
			}
			result := reverseRequest(t, f, scope, 200)
			expectedSurvey := mode != "automatic" && mode != "native source undo"
			if result.Completed || result.RelatedTasks[0].Completed != expectedSurvey {
				t.Fatalf("reverse state mode%s %+v", mode, result)
			}
			assertRow(t, f, "assigned-visit-2", false, "")
			assertRow(t, f, "visit-survey-2", expectedSurvey, "")
		})
	}
}

func TestMySQLGuidanceManualCompletionLegacyNullUserFlag(t *testing.T) {
	requireMySQL(t)
	f := newFixture(t, true)
	if err := f.db.Model(&domain.User{}).Where("id = ?", f.user.ID).Update("is_deleted", nil).Error; err != nil {
		t.Fatal(err)
	}
	if err := domain.CompleteTask(f.db, f.user.ID, f.task.ID); err != nil {
		t.Fatalf("legacyNULLactiveuser rejected: %v", err)
	}
	assertRow(t, f, "assigned-visit-2", true, "")
	if err := f.db.Model(&domain.User{}).Where("id = ?", f.user.ID).Update("is_deleted", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := domain.UncompleteTask(f.db, f.user.ID, f.task.ID); err == nil {
		t.Fatal("deleted user was allowed to mutate completion")
	}
}
