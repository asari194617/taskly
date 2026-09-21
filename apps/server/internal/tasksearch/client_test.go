package tasksearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSearchBuildsVisibleDeadlineCursorQuery(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+indexName+"/_search" || r.Method != http.MethodPost {
			t.Errorf("request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"hits":[{"_id":"42","sort":["2026-09-30",42]}]}}`))
	}))
	defer server.Close()
	c := &Client{base: server.URL, http: server.Client()}
	hits, err := c.Search(context.Background(), SearchOptions{Query: "課題", Today: "2026-09-21", VisibleOnly: true,
		CursorDeadline: "2026-09-28", CursorID: 12, Size: 21})
	if err != nil || len(hits) != 1 || hits[0].ID != 42 {
		t.Fatalf("hits=%v err=%v", hits, err)
	}
	if !reflect.DeepEqual(body["search_after"], []any{"2026-09-28", float64(12)}) {
		t.Fatalf("cursor=%v", body["search_after"])
	}
	query := body["query"].(map[string]any)["bool"].(map[string]any)
	filters := query["filter"].([]any)
	if len(filters) != 2 || filters[1].(map[string]any)["term"].(map[string]any)["is_hidden"] != false {
		t.Fatalf("filters=%v", filters)
	}
}

func TestDrainIndexesAndDeletesOutboxRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:search_outbox?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{
		"CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT, description TEXT, deadline DATE, is_hidden BOOLEAN)",
		"CREATE TABLE task_search_outbox (task_id INTEGER PRIMARY KEY, version INTEGER)",
		"INSERT INTO task_search_outbox VALUES (1, 2), (2, 1)",
		"INSERT INTO tasks (id, title, deadline, is_hidden) VALUES (1, 'exam', '2026-09-30', 0)",
	} {
		if err := db.Exec(schema).Error; err != nil {
			t.Fatal(err)
		}
	}
	methods := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusCreated)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()
	c := &Client{base: server.URL, http: &http.Client{Timeout: time.Second}}
	if err := c.drain(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(methods, []string{"PUT /" + indexName + "/_doc/1", "DELETE /" + indexName + "/_doc/2"}) {
		t.Fatalf("requests=%v", methods)
	}
	var count int64
	if err := db.Table("task_search_outbox").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("outbox=%d err=%v", count, err)
	}
}
