package tasksearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"gorm.io/gorm"
)

const indexName = "taskly_tasks_v1"

var active atomic.Pointer[Client]

type Client struct {
	base     string
	http     *http.Client
	user     string
	password string
	ready    atomic.Bool
}

func Current() *Client { return active.Load() }

func Start(ctx context.Context, db *gorm.DB) {
	raw := strings.TrimRight(strings.TrimSpace(os.Getenv("OPENSEARCH_URL")), "/")
	if raw == "" {
		return
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		log.Printf("task search: invalid OPENSEARCH_URL; using SQL search")
		return
	}
	c := &Client{base: raw, http: &http.Client{Timeout: 3 * time.Second}, user: os.Getenv("OPENSEARCH_USERNAME"), password: os.Getenv("OPENSEARCH_PASSWORD")}
	active.Store(c)
	go c.run(ctx, db)
}

func (c *Client) Ready() bool      { return c != nil && c.ready.Load() }
func (c *Client) MarkUnavailable() { c.ready.Store(false) }

func (c *Client) request(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var input io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		input = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, input)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.user != "" {
		req.SetBasicAuth(c.user, c.password)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		return nil, resp.StatusCode, fmt.Errorf("OpenSearch %s: HTTP %d", path, resp.StatusCode)
	}
	return b, resp.StatusCode, nil
}

type SearchOptions struct {
	Query          string
	Overdue        bool
	Today          string
	VisibleOnly    bool
	CursorDeadline string
	CursorID       uint64
	Size           int
	After          []any
}

type Hit struct {
	ID   uint
	Sort []any
}

func (c *Client) Search(ctx context.Context, o SearchOptions) ([]Hit, error) {
	filters := []any{map[string]any{"range": map[string]any{"deadline": map[string]any{anyKey(o.Overdue): o.Today}}}}
	if o.VisibleOnly {
		filters = append(filters, map[string]any{"term": map[string]any{"is_hidden": false}})
	}
	query := map[string]any{"bool": map[string]any{
		"must":   []any{map[string]any{"multi_match": map[string]any{"query": o.Query, "fields": []string{"title", "description"}, "operator": "and"}}},
		"filter": filters,
	}}
	direction := "asc"
	if o.Overdue {
		direction = "desc"
	}
	body := map[string]any{"size": o.Size, "_source": false, "query": query,
		"sort": []any{map[string]any{"deadline": map[string]any{"order": direction, "format": "strict_date"}}, map[string]any{"task_id": direction}}}
	if len(o.After) == 2 {
		body["search_after"] = o.After
	} else if o.CursorID != 0 && o.CursorDeadline != "" {
		body["search_after"] = []any{o.CursorDeadline, o.CursorID}
	}
	b, status, err := c.request(ctx, http.MethodPost, "/"+indexName+"/_search?allow_partial_search_results=false", body)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("OpenSearch search: HTTP %d", status)
	}
	var result struct {
		TimedOut bool `json:"timed_out"`
		Shards   struct {
			Failed int `json:"failed"`
		} `json:"_shards"`
		Hits struct {
			Hits []struct {
				ID   string `json:"_id"`
				Sort []any  `json:"sort"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	if result.TimedOut || result.Shards.Failed != 0 {
		return nil, fmt.Errorf("OpenSearch search incomplete")
	}
	out := make([]Hit, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		id, err := strconv.ParseUint(hit.ID, 10, 64)
		if err != nil || id == 0 || len(hit.Sort) != 2 {
			return nil, fmt.Errorf("invalid OpenSearch hit")
		}
		out = append(out, Hit{ID: uint(id), Sort: hit.Sort})
	}
	return out, nil
}

func anyKey(overdue bool) string {
	if overdue {
		return "lt"
	}
	return "gte"
}

func (c *Client) index(ctx context.Context, task domain.Task) error {
	doc := map[string]any{"task_id": task.ID, "title": task.Title, "description": task.Description,
		"deadline": task.Deadline.Format("2006-01-02"), "is_hidden": task.IsHidden}
	_, status, err := c.request(ctx, http.MethodPut, "/"+indexName+"/_doc/"+strconv.FormatUint(uint64(task.ID), 10), doc)
	if err != nil {
		return err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return fmt.Errorf("OpenSearch index: HTTP %d", status)
	}
	return nil
}

func (c *Client) remove(ctx context.Context, id uint) error {
	_, status, err := c.request(ctx, http.MethodDelete, "/"+indexName+"/_doc/"+strconv.FormatUint(uint64(id), 10), nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusNotFound {
		return fmt.Errorf("OpenSearch delete: HTTP %d", status)
	}
	return nil
}

const mapping = `{"settings":{"number_of_shards":1,"number_of_replicas":0,"index.max_ngram_diff":2,"analysis":{"tokenizer":{"task_ngram":{"type":"ngram","min_gram":1,"max_gram":3}},"analyzer":{"task_ngram":{"type":"custom","tokenizer":"task_ngram","filter":["lowercase"]}}}},"mappings":{"properties":{"task_id":{"type":"long","doc_values":true},"title":{"type":"text","analyzer":"task_ngram"},"description":{"type":"text","analyzer":"task_ngram"},"deadline":{"type":"date","format":"strict_date","doc_values":true},"is_hidden":{"type":"boolean"}}}}`

func (c *Client) ensureIndex(ctx context.Context) error {
	_, status, err := c.request(ctx, http.MethodHead, "/"+indexName, nil)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		return nil
	}
	var payload any
	if err := json.Unmarshal([]byte(mapping), &payload); err != nil {
		return err
	}
	_, status, err = c.request(ctx, http.MethodPut, "/"+indexName, payload)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("OpenSearch create index: HTTP %d", status)
	}
	return nil
}
