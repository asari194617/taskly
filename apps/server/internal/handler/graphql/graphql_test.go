package gqlH

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGraphQLCancelsSlowDBQuery(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:graphql-cancel?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	queryStarted := make(chan struct{}, 1)
	queryStopped := make(chan struct{}, 1)
	if err := db.Callback().Query().Before("gorm:query").Register("test:slow_query", func(tx *gorm.DB) {
		queryStarted <- struct{}{}
		select {
		case <-tx.Statement.Context.Done():
			queryStopped <- struct{}{}
		case <-time.After(time.Second):
			tx.AddError(fmt.Errorf("database query was not canceled"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	app := fiber.New()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	app.Use(func(c *fiber.Ctx) error {
		c.SetUserContext(ctx)
		c.Locals("user", &domain.User{ID: 1})
		return c.Next()
	})
	NewHandler(db, rdb, nil).RegisterGraphQL(app.Group("/graphql"))
	req := httptest.NewRequest("POST", "/graphql/", strings.NewReader(`{"query":"query { subjects { id } }"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != fiber.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504", res.StatusCode)
	}
	select {
	case <-queryStarted:
	default:
		t.Fatal("DB query did not start")
	}
	select {
	case <-queryStopped:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("DB query continued after cancellation")
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(200 * time.Millisecond)
	for sqlDB.Stats().InUse != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if inUse := sqlDB.Stats().InUse; inUse != 0 {
		t.Fatalf("DB connections still in use after cancellation: %d", inUse)
	}
	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "query timeout" {
		t.Fatalf("error = %q, want query timeout", body["error"])
	}
}

func TestGraphQLRejectsExpensiveRequests(t *testing.T) {
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	app := fiber.New()
	NewHandler(nil, rdb, nil).RegisterGraphQL(app.Group("/graphql"))

	aliases := make([]string, 101)
	for i := range aliases {
		aliases[i] = fmt.Sprintf("a%d: dayShiftIndex", i)
	}
	spreads := strings.Repeat("...F ", 101)
	lists := make([]string, 26)
	for i := range lists {
		lists[i] = fmt.Sprintf("s%d: subjects { id }", i)
	}
	tests := []struct {
		name   string
		query  string
		vars   map[string]any
		status int
	}{
		{"aliases", "query { " + strings.Join(aliases, " ") + " }", nil, 400},
		{"fragments", "query { " + spreads + " } fragment F on Query { dayShiftIndex }", nil, 400},
		{"nested list", "query { " + strings.Join(lists, " ") + " }", nil, 400},
		{"variable page size", "query($n: Int) { tasks(limit: $n) { items { id } } }", map[string]any{"n": 500}, 400},
		{"very large variable page size", "query($n: Int) { tasks(limit: $n) { items { id } } }", map[string]any{"n": 2000000}, 400},
		{"normal task page", "query { tasks(limit: 50) { items { id title subject_id deadline deadline_since deadline_until description type is_require_hand_in is_self_check_required is_completed is_hidden } next_cursor { cursor_deadline cursor_id } } }", nil, 401},
		{"query size", "query { dayShiftIndex } #" + strings.Repeat("x", maxGraphQLQueryBytes), nil, 413},
		{"variables size", "query($s: String) { dayShiftIndex }", map[string]any{"s": strings.Repeat("x", maxGraphQLVariablesBytes)}, 413},
		{"body size", "query { dayShiftIndex } #" + strings.Repeat("x", maxGraphQLBodyBytes), nil, 413},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{"query": tt.query, "variables": tt.vars})
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
			if res.StatusCode != tt.status {
				t.Fatalf("status = %d, want %d", res.StatusCode, tt.status)
			}
		})
	}
}

func TestMissingSessionReturnsUnauthorized(t *testing.T) {
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	app := fiber.New()
	NewHandler(nil, rdb, nil).RegisterGraphQL(app.Group("/graphql"))
	req := httptest.NewRequest("POST", "/graphql/", strings.NewReader(`{"query":"query { me { id } }"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", res.StatusCode)
	}
}

type failedGraphQLRandomReader struct{}

func (failedGraphQLRandomReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestSecretMutationsFailOnRandomError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.IcalKey{}, &domain.RecoveryCode{}); err != nil {
		t.Fatal(err)
	}
	u := domain.User{ID: 9, Email: "test@example.com"}
	if err := db.Create(&domain.RecoveryCode{UserID: u.ID, HashedCode: "old"}).Error; err != nil {
		t.Fatal(err)
	}
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", &u); return c.Next() })
	NewHandler(db, rdb, nil).RegisterGraphQL(app.Group("/graphql"))
	original := rand.Reader
	rand.Reader = failedGraphQLRandomReader{}
	t.Cleanup(func() { rand.Reader = original })
	for _, query := range []string{
		"mutation { icalCreate { id url } }",
		"mutation { totpSetup { secret otpauth } }",
		"mutation { recoveryGenerate }",
	} {
		req := httptest.NewRequest("POST", "/graphql/", strings.NewReader(fmt.Sprintf(`{"query":%q}`, query)))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != 400 {
			t.Errorf("%s: status = %d, want 400", query, res.StatusCode)
		}
	}
	var ical []domain.IcalKey
	if err := db.Find(&ical).Error; err != nil {
		t.Fatal(err)
	}
	var codes []domain.RecoveryCode
	if err := db.Where("user_id = ?", u.ID).Find(&codes).Error; err != nil {
		t.Fatal(err)
	}
	if len(ical) != 0 || len(codes) != 1 || codes[0].HashedCode != "old" {
		t.Fatalf("secret rows changed: ical=%+v recovery=%+v", ical, codes)
	}
	if redisServer.Exists(fmt.Sprintf("totp-setup:%d", u.ID)) {
		t.Fatal("TOTP secret persisted")
	}
}
