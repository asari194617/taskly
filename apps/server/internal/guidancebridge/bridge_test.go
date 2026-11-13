package guidancebridge

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type fixture struct {
	db             *gorm.DB
	app            *fiber.App
	user           domain.User
	task           domain.Task
	bindings       map[string]uint
	identity       map[string]any
	issuer         *httptest.Server
	upstreamStatus int
	upstreamBody   string
}

func fixtureDB(t *testing.T, mysqlDB bool) *gorm.DB {
	t.Helper()
	var db *gorm.DB
	var err error
	if mysqlDB {
		cfg := mysqldriver.NewConfig()
		cfg.User = os.Getenv("MYSQL_USER")
		cfg.Passwd = os.Getenv("MYSQL_PASSWORD")
		cfg.Net = "tcp"
		cfg.Addr = os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT")
		cfg.DBName = os.Getenv("MYSQL_DATABASE")
		cfg.ParseTime = true
		db, err = gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	} else {
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	}
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if mysqlDB {
		tx := db.Begin()
		if tx.Error != nil {
			t.Fatal(tx.Error)
		}
		t.Cleanup(func() { tx.Rollback() })
		return tx
	}
	if err = db.AutoMigrate(&domain.User{}, &domain.Block{}, &domain.RoleUser{}, &domain.Permission{}, &legal.Consent{}); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE TABLE tasks (id integer primary key, added_user_id integer, title text, subject_id integer, description text, type text, is_require_hand_in boolean, is_self_check_required boolean, is_completion_disabled boolean default 0, is_hidden boolean default 0, deadline date, deadline_since text, deadline_until text, created_at datetime, updated_at datetime)`,
		`CREATE TABLE tasks_users_completions (user_id integer, task_id integer, completed_at datetime DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY(user_id,task_id))`,
	} {
		if err = db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func newFixture(t *testing.T, mysqlDB bool) *fixture {
	t.Helper()
	f := &fixture{db: fixtureDB(t, mysqlDB), upstreamStatus: 200}
	f.user = domain.User{Email: "bridge-synthetic@example.test", DisplayName: "Synthetic"}
	if err := f.db.Create(&f.user).Error; err != nil {
		t.Fatal(err)
	}
	if err := legal.Accept(f.db, f.user.ID, legal.Version, legal.Version, legal.Version, "ja"); err != nil {
		t.Fatal(err)
	}
	var subjectID uint = 1
	if mysqlDB {
		s := domain.Subject{NameJa: "Synthetic bridge fixture"}
		if err := f.db.Create(&s).Error; err != nil {
			t.Fatal(err)
		}
		subjectID = s.ID
	}
	f.bindings = map[string]uint{}
	for _, key := range []string{"assigned-visit-2", "visit-survey-2", "overall-survey", "sheet-submission"} {
		task := domain.Task{AddedUserID: f.user.ID, Title: "Synthetic " + key, SubjectID: subjectID, Type: "OTHER", Deadline: time.Date(2030, 5, 14, 0, 0, 0, 0, time.UTC), DeadlineSince: "NONE", DeadlineUntil: "NONE"}
		if err := f.db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
		f.bindings[key] = task.ID
		if key == "assigned-visit-2" {
			f.task = task
		}
	}
	f.identity = map[string]any{"active": true, "client_id": guidanceApplication, "sub": strconv.Itoa(int(f.user.ID)), "scope": "taskly:login", "aud": "taskly-api", "exp": time.Now().Add(time.Hour).Unix(), "token_type": "Bearer"}
	f.issuer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, secret, ok := r.BasicAuth()
		if !ok || id != "taskly-confidential" || secret != "synthetic-secret" || r.Method != "POST" || r.URL.Path != "/oauth/introspect" {
			t.Errorf("unexpected introspection request")
			w.WriteHeader(401)
			return
		}
		r.ParseForm()
		if r.Form.Get("token") != "synthetic-access" || r.Form.Get("token_type_hint") != "access_token" {
			t.Error("wrong forwarded token")
		}
		w.WriteHeader(f.upstreamStatus)
		if f.upstreamBody != "" {
			io.WriteString(w, f.upstreamBody)
			return
		}
		json.NewEncoder(w).Encode(f.identity)
	}))
	t.Cleanup(f.issuer.Close)
	f.identity["iss"] = f.issuer.URL
	t.Setenv("AUTH_ISSUER", f.issuer.URL)
	t.Setenv("AUTH_CLIENT_ID", "taskly-confidential")
	t.Setenv("AUTH_CLIENT_SECRET", "synthetic-secret")
	t.Setenv("WEB_PUBLIC_URL", "https://taskly.example.test")
	t.Setenv("NNCT_GUIDANCE_EVENT_ID", "synthetic-event")
	t.Setenv("NODE_ENV", "test")
	encoded, _ := json.Marshal(f.bindings)
	t.Setenv("NNCT_GUIDANCE_TASK_BINDINGS", string(encoded))
	f.app = fiber.New()
	Register(f.app.Group("/integrations/nnct-guidance"), f.db)
	return f
}

func (f *fixture) request(t *testing.T, method, key, body string, headers map[string]string) (int, []byte) {
	t.Helper()
	path := "/integrations/nnct-guidance/tasks"
	if key != "" {
		path += "/" + key
	}
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer synthetic-access")
	r.Header.Set("X-Guidance-Event-ID", "synthetic-event")
	r.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	res, err := f.app.Test(r, 10000)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Error("missing no-store")
	}
	return res.StatusCode, data
}

func TestBridgeAuthenticationAndIdentityClaims(t *testing.T) {
	cases := []struct {
		name, field string
		value       any
	}{{"inactive", "active", false}, {"other client", "client_id", "other-app"}, {"wrong audience", "aud", "another-api"}, {"wrong issuer", "iss", "https://another.example.test"}, {"expired", "exp", int64(1)}, {"wrong scope", "scope", "unrelated"}, {"wrong type", "token_type", "other"}, {"bad subject", "sub", "not-a-user"}, {"zero subject", "sub", "0"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, false)
			f.identity[tc.field] = tc.value
			if code, _ := f.request(t, "GET", "", "", nil); code != 401 {
				t.Fatalf("status=%d", code)
			}
		})
	}
	for _, headers := range []map[string]string{{"Authorization": ""}, {"Authorization": "synthetic-access"}, {"Authorization": "Bearer a b"}, {"Cookie": "taskly_session=x"}, {"X-API-Key": "other"}, {"X-MCP-Token": "other"}} {
		f := newFixture(t, false)
		if code, _ := f.request(t, "GET", "", "", headers); code != 401 {
			t.Fatalf("credentials status=%d", code)
		}
	}
}

func TestBridgeUpstreamFailuresAndEventNamespace(t *testing.T) {
	for _, test := range []struct {
		status int
		body   string
	}{{503, ""}, {302, ""}, {200, "invalid-json"}, {200, strings.Repeat("x", 16385)}} {
		f := newFixture(t, false)
		f.upstreamStatus = test.status
		f.upstreamBody = test.body
		if code, _ := f.request(t, "GET", "", "", nil); code != 503 {
			t.Fatalf("upstream status=%d", code)
		}
	}
	f := newFixture(t, false)
	if code, _ := f.request(t, "PUT", "assigned-visit-2", `{"completed":true}`, map[string]string{"X-Guidance-Event-ID": "other-event"}); code != 409 {
		t.Fatalf("event status=%d", code)
	}
	f.issuer.Close()
	if code, _ := f.request(t, "GET", "", "", nil); code != 503 {
		t.Fatalf("outage status=%d", code)
	}
}

func TestBridgeLocalPolicyAndTaskAccess(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
		status    int
	}{
		{"deleted", "UPDATE users SET is_deleted=1", 401}, {"missing consent", "DELETE FROM policy_consents", 403}, {"blocked", "INSERT INTO blocks(user_id) SELECT id FROM users", 403}, {"hidden", "UPDATE tasks SET is_hidden=1", 403}, {"denied", "INSERT INTO permissions(user_id,target,method,value) SELECT id,'tasks','read','none' FROM users", 403},
		{"missing user", "DELETE FROM users", 401}, {"missing task", "DELETE FROM tasks", 404}, {"broken completion store", "DROP TABLE tasks_users_completions", 503}, {"broken consent store", "DROP TABLE policy_consents", 503}, {"broken block store", "DROP TABLE blocks", 503}, {"broken access store", "DROP TABLE role_users", 503}, {"broken permission store", "DROP TABLE permissions", 503}, {"broken user store", "DROP TABLE users", 503}, {"broken task store", "DROP TABLE tasks", 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, false)
			if err := f.db.Exec(tc.sql).Error; err != nil {
				t.Fatal(err)
			}
			if code, _ := f.request(t, "GET", "", "", nil); code != tc.status {
				t.Fatalf("status=%d want%d", code, tc.status)
			}
		})
	}
	f := newFixture(t, false)
	if err := f.db.Exec("INSERT INTO blocks(user_id,expires_at) VALUES (?,?)", f.user.ID, time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if code, _ := f.request(t, "GET", "", "", nil); code != 200 {
		t.Fatalf("expired block status=%d", code)
	}
	f.db.Exec("UPDATE tasks SET is_hidden=1")
	f.db.Create(&domain.RoleUser{UserID: f.user.ID, Role: "admin"})
	if code, _ := f.request(t, "GET", "", "", nil); code != 200 {
		t.Fatalf("admin hidden status=%d", code)
	}
}

func TestBridgeReadsCanonicalStateAndUndoesOnlyOwnCompletion(t *testing.T) {
	f := newFixture(t, false)
	f.db.Exec("INSERT INTO tasks_users_completions(user_id,task_id) VALUES (?,?),(?,?)", f.user.ID, f.task.ID, f.user.ID+1, f.task.ID)
	code, body := f.request(t, "GET", "", "", nil)
	if code != 200 {
		t.Fatalf("status %d: %s", code, body)
	}
	var response struct {
		EventID string      `json:"event_id"`
		Tasks   []taskState `json:"tasks"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if response.EventID != "synthetic-event" || len(response.Tasks) != 4 || !response.Tasks[0].Completed || response.Tasks[0].TaskID != f.task.ID || response.Tasks[0].TasklyURL != "https://taskly.example.test/" {
		t.Fatalf("wrong canonical response %s", body)
	}
	f.db.Exec("UPDATE tasks SET is_completion_disabled=1 WHERE id=?", f.task.ID)
	_, body = f.request(t, "GET", "", "", nil)
	if err := json.Unmarshal(body, &response); err != nil || response.Tasks[0].Completed || !response.Tasks[0].CompletionDisabled {
		t.Fatalf("disabled completion must match Taskly reader: %s, %v", body, err)
	}
	if code, _ := f.request(t, "PUT", "assigned-visit-2", `{"completed":true}`, nil); code != 409 {
		t.Fatalf("disabled=%d", code)
	}
	if code, _ := f.request(t, "PUT", "assigned-visit-2", `{"completed":false}`, nil); code != 200 {
		t.Fatalf("undo=%d", code)
	}
	var marks int64
	f.db.Table("tasks_users_completions").Where("user_id=?", f.user.ID+1).Count(&marks)
	if marks != 1 {
		t.Fatal("other user's mark changed")
	}
	if code, _ := f.request(t, "PUT", "assigned-visit-2", `{"completed":false}`, nil); code != 200 {
		t.Fatalf("repeat undo=%d", code)
	}
}

func TestBridgeRejectsArbitraryWrites(t *testing.T) {
	f := newFixture(t, false)
	for _, body := range []string{`{}`, `null`, `{"completed":null}`, `{"completed":"true"}`, `{"completed":true,"user_id":2}`, `{"completed":false} {}`, `{`, strings.Repeat(" ", 1025)} {
		if code, _ := f.request(t, "PUT", "assigned-visit-2", body, nil); code != 400 {
			t.Fatalf("body %q status=%d", body, code)
		}
	}
	for _, key := range []string{"unlisted-task", "123"} {
		if code, _ := f.request(t, "PUT", key, `{"completed":false}`, nil); code != 404 {
			t.Fatalf("key status=%d", code)
		}
	}
}

// This exercises the unchanged canonical MySQL upsert, including retry/undo,
// against the same schema and always-run disposable database job as Taskly.
func TestMySQLGuidanceBridgeCanonicalRoundTrip(t *testing.T) {
	if os.Getenv("TEST_SCHEMA_DB") != "1" {
		t.Skip("requires disposable MySQL schema")
	}
	f := newFixture(t, true)
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { rdb.Close() })
	sessionToken := "synthetic-taskly-session"
	expires := time.Now().Add(time.Hour)
	if err := f.db.Create(&domain.Session{UserID: f.user.ID, HashedToken: utils.Sha256Hex([]byte(sessionToken)), ExpiresAt: &expires}).Error; err != nil {
		t.Fatal(err)
	}
	// Exercise the actual authenticated Taskly GraphQL route and resolvers,
	// independently of the guidance client's OAuth authorization path.
	gqlH.NewHandler(f.db, rdb, nil).RegisterGraphQL(f.app.Group("/graphql"))
	graphqlRequest := func(query string) []byte {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"query": query})
		req := httptest.NewRequest("POST", "/graphql/", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+sessionToken)
		req.Header.Set("Content-Type", "application/json")
		res, err := f.app.Test(req, 10000)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		result, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 || bytes.Contains(result, []byte(`"errors"`)) {
			t.Fatalf("Taskly GraphQL %d: %s", res.StatusCode, result)
		}
		return result
	}
	for _, completed := range []bool{true, true, false, true, false} {
		code, body := f.request(t, "PUT", "assigned-visit-2", `{"completed":`+strconv.FormatBool(completed)+`}`, nil)
		if code != 200 {
			t.Fatalf("write %v status %d: %s", completed, code, body)
		}
		var state taskState
		if err := json.Unmarshal(body, &state); err != nil || state.Completed != completed {
			t.Fatalf("state %s %v", body, err)
		}
		var count int64
		f.db.Table("tasks_users_completions").Where("user_id=? AND task_id=?", f.user.ID, f.task.ID).Count(&count)
		if (count == 1) != completed {
			t.Fatalf("canonical marks=%d", count)
		}
		graphqlBody := graphqlRequest(`{ tasks(q:"Synthetic assigned-visit-2") { items { id is_completed } } }`)
		var result struct {
			Data struct {
				Tasks struct {
					Items []struct {
						ID        int  `json:"id"`
						Completed bool `json:"is_completed"`
					} `json:"items"`
				} `json:"tasks"`
			} `json:"data"`
		}
		if err := json.Unmarshal(graphqlBody, &result); err != nil || len(result.Data.Tasks.Items) != 1 || result.Data.Tasks.Items[0].Completed != completed {
			t.Fatalf("guidance-to-Taskly state %s: %v", graphqlBody, err)
		}
	}
	for _, completed := range []bool{true, false, true} {
		mutation := "uncompleteTask"
		if completed {
			mutation = "completeTask"
		}
		graphqlRequest("mutation { " + mutation + "(id:" + strconv.Itoa(int(f.task.ID)) + ") }")
		_, body := f.request(t, "GET", "", "", nil)
		var result struct {
			Tasks []taskState `json:"tasks"`
		}
		if err := json.Unmarshal(body, &result); err != nil || len(result.Tasks) != 4 || result.Tasks[0].Completed != completed {
			t.Fatalf("Taskly-to-guidance state %s: %v", body, err)
		}
	}
	f.db.Model(&f.task).Update("is_completion_disabled", true)
	_, body := f.request(t, "GET", "", "", nil)
	var disabled struct {
		Tasks []taskState `json:"tasks"`
	}
	if err := json.Unmarshal(body, &disabled); err != nil || len(disabled.Tasks) != 4 || disabled.Tasks[0].Completed || !disabled.Tasks[0].CompletionDisabled {
		t.Fatalf("disabled bridge state: %s %v", body, err)
	}
	graphqlBody := graphqlRequest(`{ tasks(q:"Synthetic assigned-visit-2") { items { is_completed is_completion_disabled } } }`)
	if !bytes.Contains(graphqlBody, []byte(`"is_completed":false`)) || !bytes.Contains(graphqlBody, []byte(`"is_completion_disabled":true`)) {
		t.Fatalf("disabled Taskly reader parity: %s", graphqlBody)
	}
	if code, _ := f.request(t, "PUT", "assigned-visit-2", `{"completed":true}`, nil); code != 409 {
		t.Fatalf("disabled=%d", code)
	}
	if code, _ := f.request(t, "PUT", "assigned-visit-2", `{"completed":false}`, nil); code != 200 {
		t.Fatalf("undo disabled=%d", code)
	}
}
