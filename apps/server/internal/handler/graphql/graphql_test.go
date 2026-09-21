package gqlH

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

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
