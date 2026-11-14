package guidancebridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func newDelegatedFixture(t *testing.T, mysqlDB bool, scope string) *fixture {
	t.Helper()
	f := newFixture(t, mysqlDB)
	t.Setenv("API_PUBLIC_URL", "https://api.taskly.example.test")
	t.Setenv("NNCT_GUIDANCE_MCP_PARENT_RESOURCE", "https://api.guidance.example.test/mcp")
	f.identity["client_id"] = guidanceMCPBackend
	f.identity["aud"] = "https://api.taskly.example.test" + guidanceTasksPath
	f.identity["scope"] = scope
	f.identity["exp"] = time.Now().Add(45 * time.Second).Unix()
	f.identity["original_client_id"] = guidanceMCPApplication
	f.identity["original_resource"] = "https://api.guidance.example.test/mcp"
	f.identity["act"] = map[string]any{"sub": guidanceMCPBackend}
	f.app = fiber.New()
	Register(f.app.Group("/integrations/nnct-guidance"), f.db)
	return f
}

func TestDelegatedBridgeClaimsAreNarrowAndLive(t *testing.T) {
	cases := []struct {
		name, field string
		value       any
	}{
		{"inactive parent or family", "active", false},
		{"public MCP token", "client_id", guidanceMCPApplication},
		{"other confidential client", "client_id", "another-backend"},
		{"parent audience", "aud", "https://api.guidance.example.test/mcp"},
		{"general API audience", "aud", "taskly-api"},
		{"resource prefix", "aud", "https://api.taskly.example.test" + guidanceTasksPath + "/other"},
		{"other resource origin", "aud", "https://other.example.test" + guidanceTasksPath},
		{"resource trailing slash", "aud", "https://api.taskly.example.test" + guidanceTasksPath + "/"},
		{"wrong original client", "original_client_id", guidanceApplication},
		{"missing original client", "original_client_id", ""},
		{"wrong original resource", "original_resource", "https://other.example.test/mcp"},
		{"parent resource alias", "original_resource", "https://api.guidance.example.test/mcp/"},
		{"missing original resource", "original_resource", ""},
		{"missing actor", "act", nil},
		{"wrong actor", "act", map[string]any{"sub": "another-backend"}},
		{"missing actor subject", "act", map[string]any{}},
		{"wrong issuer", "iss", "https://other.example.test"},
		{"expired", "exp", int64(1)},
		{"too long lived", "exp", time.Now().Add(2 * time.Minute).Unix()},
		{"wrong bearer type", "token_type", "other"},
		{"invalid subject", "sub", "invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newDelegatedFixture(t, false, guidanceTasksRead)
			f.identity[tc.field] = tc.value
			if code, _ := f.request(t, "GET", "", "", nil); code != 401 {
				t.Fatalf("status=%d", code)
			}
		})
	}
	f := newDelegatedFixture(t, false, guidanceTasksRead)
	if code, _ := f.request(t, "GET", "", "", nil); code != 200 {
		t.Fatalf("read=%d", code)
	}
	// Introspection is repeated for each operation, including revocation after use.
	f.identity["active"] = false
	if code, _ := f.request(t, "GET", "", "", nil); code != 401 {
		t.Fatalf("revoked=%d", code)
	}
}

func TestDelegatedBridgeScopesBindOneHTTPAction(t *testing.T) {
	for _, tc := range []struct {
		scope, method string
		status        int
	}{
		{guidanceTasksRead, "GET", 200}, {guidanceTasksWrite, "PUT", 200},
		{guidanceTasksRead, "PUT", 403}, {guidanceTasksWrite, "GET", 403},
		{guidanceTasksRead + " " + guidanceTasksWrite, "GET", 403},
		{guidanceTasksRead + " " + guidanceTasksRead, "GET", 403},
		{"guidance:read", "GET", 403}, {"taskly:login", "PUT", 403},
		{"", "GET", 403}, {guidanceTasksRead, "HEAD", 403},
	} {
		t.Run(tc.method+" "+tc.scope, func(t *testing.T) {
			f := newDelegatedFixture(t, false, tc.scope)
			cfg, err := loadConfig()
			if err != nil {
				t.Fatal(err)
			}
			b := &bridge{db: f.db, cfg: cfg}
			f.app = fiber.New()
			// Exercise policy independently of MySQL-only canonical mutation SQL.
			accepted := func(c *fiber.Ctx) error { return c.SendStatus(200) }
			f.app.Get(guidanceTasksPath, b.authenticate, accepted)
			f.app.Put(guidanceTasksPath+"/:key", b.authenticate, accepted)
			key := ""
			if tc.method == "PUT" {
				key = "assigned-visit-2"
			}
			code, _ := f.request(t, tc.method, key, `{"completed":true}`, nil)
			if code != tc.status {
				t.Fatalf("status=%d", code)
			}
			var count int64
			if err := f.db.Table("tasks_users_completions").Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("authentication changed completion records=%d", count)
			}
		})
	}
}

func TestDelegationConfigurationFailsClosedWithoutChangingLegacyPolicy(t *testing.T) {
	for _, tc := range []struct{ name, env, value string }{
		{"missing API origin", "API_PUBLIC_URL", ""},
		{"API userinfo", "API_PUBLIC_URL", "https://a:b@api.taskly.example.test"},
		{"API path", "API_PUBLIC_URL", "https://api.taskly.example.test/other"},
		{"API query", "API_PUBLIC_URL", "https://api.taskly.example.test/?x=1"},
		{"API empty query", "API_PUBLIC_URL", "https://api.taskly.example.test?"},
		{"API bad URL", "API_PUBLIC_URL", ":invalid"},
		{"API bad scheme", "API_PUBLIC_URL", "ftp://api.taskly.example.test"},
		{"API whitespace", "API_PUBLIC_URL", " https://api.taskly.example.test"},
		{"missing parent", "NNCT_GUIDANCE_MCP_PARENT_RESOURCE", ""},
		{"parent missing host", "NNCT_GUIDANCE_MCP_PARENT_RESOURCE", "https:/mcp"},
		{"parent path", "NNCT_GUIDANCE_MCP_PARENT_RESOURCE", "https://api.guidance.example.test/other"},
		{"parent slash", "NNCT_GUIDANCE_MCP_PARENT_RESOURCE", "https://api.guidance.example.test/mcp/"},
		{"parent query", "NNCT_GUIDANCE_MCP_PARENT_RESOURCE", "https://api.guidance.example.test/mcp?x=1"},
		{"parent fragment", "NNCT_GUIDANCE_MCP_PARENT_RESOURCE", "https://api.guidance.example.test/mcp#other"},
		{"parent encoded alias", "NNCT_GUIDANCE_MCP_PARENT_RESOURCE", "https://api.guidance.example.test/m%63p"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDelegatedFixture(t, false, guidanceTasksRead)
			t.Setenv(tc.env, tc.value)
			f.app = fiber.New()
			Register(f.app.Group("/integrations/nnct-guidance"), f.db)
			if code, _ := f.request(t, "GET", "", "", nil); code != 401 {
				t.Fatalf("delegated=%d", code)
			}
			f.identity["client_id"], f.identity["aud"], f.identity["scope"] = guidanceApplication, "taskly-api", "unrelated taskly:login"
			f.identity["exp"] = time.Now().Add(time.Hour).Unix()
			delete(f.identity, "act")
			delete(f.identity, "original_client_id")
			delete(f.identity, "original_resource")
			if code, _ := f.request(t, "GET", "", "", nil); code != 200 {
				t.Fatalf("legacy=%d", code)
			}
		})
	}
	t.Run("production HTTP origin rejected", func(t *testing.T) {
		newDelegatedFixture(t, false, guidanceTasksRead)
		t.Setenv("NODE_ENV", "production")
		t.Setenv("API_PUBLIC_URL", "http://127.0.0.1:3063")
		if resource, parent := delegatedResources(); resource != "" || parent != "" {
			t.Fatal("insecure delegation configured")
		}
	})
	t.Run("development HTTP and origin trailing slash", func(t *testing.T) {
		newDelegatedFixture(t, false, guidanceTasksRead)
		t.Setenv("API_PUBLIC_URL", "http://127.0.0.1:3063/")
		t.Setenv("NNCT_GUIDANCE_MCP_PARENT_RESOURCE", "http://127.0.0.1:3071/mcp")
		resource, parent := delegatedResources()
		if resource != "http://127.0.0.1:3063"+guidanceTasksPath || parent != "http://127.0.0.1:3071/mcp" {
			t.Fatal("invalid development resources")
		}
	})
}

func TestDelegatedBridgeIgnoresRequestHostAndNeverRedirectsIntrospection(t *testing.T) {
	f := newDelegatedFixture(t, false, guidanceTasksRead)
	req := httptest.NewRequest("GET", guidanceTasksPath, nil)
	req.Host = "attacker.example.test"
	req.Header.Set("Authorization", "Bearer synthetic-access")
	req.Header.Set("X-Guidance-Event-ID", "synthetic-event")
	res, err := f.app.Test(req, 10000)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("configured resource changed with Host: %d", res.StatusCode)
	}
	f.upstreamStatus = http.StatusTemporaryRedirect
	if code, _ := f.request(t, "GET", "", "", nil); code != 503 {
		t.Fatalf("redirect=%d", code)
	}
}

func TestIncomingMCPParentAndDelegatedWrongRouteNeverReachCompletion(t *testing.T) {
	f := newDelegatedFixture(t, false, guidanceTasksWrite)
	f.identity["client_id"], f.identity["aud"], f.identity["scope"] = guidanceMCPApplication, "https://api.guidance.example.test/mcp", "guidance:read guidance:tasks:write"
	if code, _ := f.request(t, "PUT", "assigned-visit-2", `{"completed":true}`, nil); code != 401 {
		t.Fatalf("parent bearer forwarded=%d", code)
	}
	f.identity["client_id"], f.identity["aud"], f.identity["scope"] = guidanceMCPBackend, "https://api.taskly.example.test"+guidanceTasksPath, guidanceTasksWrite
	body, _ := json.Marshal(map[string]any{"completed": true, "client_id": guidanceMCPBackend})
	if code, _ := f.request(t, "PUT", "assigned-visit-2", string(body), nil); code != 400 {
		t.Fatalf("untrusted request policy fields=%d", code)
	}
	var count int64
	if err := f.db.Table("tasks_users_completions").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("unexpected state count=%d err=%v", count, err)
	}
	// Scope values are case-sensitive, never substring grants.
	f.identity["scope"] = strings.ToUpper(guidanceTasksWrite)
	if code, _ := f.request(t, "PUT", "assigned-visit-2", `{"completed":true}`, nil); code != 403 {
		t.Fatalf("scope case=%d", code)
	}
}
