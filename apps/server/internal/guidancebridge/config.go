// Package guidancebridge exposes a deliberately narrow, first-party OAuth bridge
// to existing Taskly completion records. It never creates shared class tasks.
package guidancebridge

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
)

const (
	guidanceApplication    = "nnct-corp-guidance-2026"
	guidanceMCPApplication = "nnct-guidance-chatgpt"
	guidanceMCPBackend     = "nnct-guidance-mcp-backend"
	guidanceTasksPath      = "/integrations/nnct-guidance/tasks"
	guidanceTasksRead      = "guidance:tasks:read"
	guidanceTasksWrite     = "guidance:tasks:write"
)

type config struct {
	eventID, issuer, clientID, clientSecret, webURL string
	delegatedResource, parentResource               string
	bindings                                        map[string]uint
}

var taskKeyPattern = regexp.MustCompile(`^(assigned-visit|visit-survey)-[1-9][0-9]?$`)

func (cfg config) keys() []string {
	result := make([]string, 0, len(cfg.bindings))
	for key := range cfg.bindings {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func loadConfig() (config, error) {
	cfg := config{
		eventID:  os.Getenv("NNCT_GUIDANCE_EVENT_ID"),
		issuer:   strings.TrimRight(os.Getenv("AUTH_ISSUER"), "/"),
		clientID: os.Getenv("AUTH_CLIENT_ID"), clientSecret: os.Getenv("AUTH_CLIENT_SECRET"),
		webURL: strings.TrimRight(os.Getenv("WEB_PUBLIC_URL"), "/"),
	}
	bad := errors.New("guidance integration not configured")
	if cfg.eventID == "" || len(cfg.eventID) > 100 || strings.TrimSpace(cfg.eventID) != cfg.eventID || cfg.clientID == "" || cfg.clientSecret == "" {
		return config{}, bad
	}
	for _, raw := range []string{cfg.issuer, cfg.webURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
			return config{}, bad
		}
		if strings.EqualFold(os.Getenv("NODE_ENV"), "production") && u.Scheme != "https" {
			return config{}, bad
		}
	}
	// Delegation is opt-in and independently fails closed. Incomplete MCP
	// configuration must not change the established guidance web-token policy.
	cfg.delegatedResource, cfg.parentResource = delegatedResources()

	rawBindings := os.Getenv("NNCT_GUIDANCE_TASK_BINDINGS")
	if len(rawBindings) > 16384 {
		return config{}, bad
	}
	if err := json.Unmarshal([]byte(rawBindings), &cfg.bindings); err != nil || len(cfg.bindings) < 4 || len(cfg.bindings) > 64 || cfg.bindings["overall-survey"] == 0 || cfg.bindings["sheet-submission"] == 0 {
		return config{}, bad
	}
	seen := map[uint]bool{}
	assigned, survey := false, false
	for key, id := range cfg.bindings {
		if key != "overall-survey" && key != "sheet-submission" && !taskKeyPattern.MatchString(key) {
			return config{}, bad
		}
		assigned = assigned || strings.HasPrefix(key, "assigned-visit-")
		survey = survey || strings.HasPrefix(key, "visit-survey-")
		if id == 0 || seen[id] {
			return config{}, bad
		}
		seen[id] = true
	}
	if !assigned || !survey {
		return config{}, bad
	}
	return cfg, nil
}

func delegatedResources() (string, string) {
	origin := strings.TrimRight(os.Getenv("API_PUBLIC_URL"), "/")
	parent := os.Getenv("NNCT_GUIDANCE_MCP_PARENT_RESOURCE")
	if origin == "" || parent == "" {
		return "", ""
	}
	for _, raw := range []string{origin, parent} {
		u, err := url.Parse(raw)
		if err != nil || strings.TrimSpace(raw) != raw || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" || (u.Scheme != "https" && u.Scheme != "http") {
			return "", ""
		}
		if strings.EqualFold(os.Getenv("NODE_ENV"), "production") && u.Scheme != "https" {
			return "", ""
		}
	}
	api, _ := url.Parse(origin)
	resource, _ := url.Parse(parent)
	if api.Path != "" || resource.Path != "/mcp" {
		return "", ""
	}
	return origin + guidanceTasksPath, parent
}
