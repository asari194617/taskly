package guidancebridge

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestConfigurationRequiresPrivateBoundedAllowlist(t *testing.T) {
	f := newFixture(t, false)
	if _, err := loadConfig(); err != nil {
		t.Fatal(err)
	}
	tests := []struct{ name, env, value string }{
		{"missing event", "NNCT_GUIDANCE_EVENT_ID", ""}, {"event whitespace", "NNCT_GUIDANCE_EVENT_ID", " x "}, {"long event", "NNCT_GUIDANCE_EVENT_ID", strings.Repeat("x", 101)},
		{"missing confidential client", "AUTH_CLIENT_ID", ""}, {"missing confidential secret", "AUTH_CLIENT_SECRET", ""}, {"missing issuer", "AUTH_ISSUER", ""},
		{"issuer query", "AUTH_ISSUER", "https://auth.example.test/?x=1"}, {"issuer userinfo", "AUTH_ISSUER", "https://a:b@auth.example.test"}, {"bad scheme", "AUTH_ISSUER", "ftp://auth.example.test"}, {"bad URL", "AUTH_ISSUER", ":invalid"}, {"web fragment", "WEB_PUBLIC_URL", "https://taskly.example.test/#x"},
		{"missing bindings", "NNCT_GUIDANCE_TASK_BINDINGS", ""}, {"malformed bindings", "NNCT_GUIDANCE_TASK_BINDINGS", "{"}, {"zero ID", "NNCT_GUIDANCE_TASK_BINDINGS", `{"assigned-visit-2":0,"visit-survey-2":2,"overall-survey":3,"sheet-submission":4}`},
		{"duplicate IDs", "NNCT_GUIDANCE_TASK_BINDINGS", `{"assigned-visit-2":1,"visit-survey-2":1,"overall-survey":3,"sheet-submission":4}`},
		{"unknown key", "NNCT_GUIDANCE_TASK_BINDINGS", `{"arbitrary":1,"visit-survey-2":2,"overall-survey":3,"sheet-submission":4}`},
		{"no assignment", "NNCT_GUIDANCE_TASK_BINDINGS", `{"visit-survey-1":1,"visit-survey-2":2,"overall-survey":3,"sheet-submission":4}`},
		{"no survey", "NNCT_GUIDANCE_TASK_BINDINGS", `{"assigned-visit-1":1,"assigned-visit-2":2,"overall-survey":3,"sheet-submission":4}`},
		{"missing sheet", "NNCT_GUIDANCE_TASK_BINDINGS", `{"assigned-visit-2":1,"visit-survey-2":2,"overall-survey":3,"visit-survey-3":4}`},
		{"invalid term", "NNCT_GUIDANCE_TASK_BINDINGS", `{"assigned-visit-0":1,"visit-survey-2":2,"overall-survey":3,"sheet-submission":4}`},
		{"large bindings", "NNCT_GUIDANCE_TASK_BINDINGS", strings.Repeat(" ", 16385)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.env, tc.value)
			if _, err := loadConfig(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	t.Run("production requires TLS", func(t *testing.T) {
		t.Setenv("NODE_ENV", "production")
		if _, err := loadConfig(); err == nil {
			t.Fatal("HTTP issuer accepted in production")
		}
	})
	t.Setenv("NNCT_GUIDANCE_EVENT_ID", "")
	// Config is captured on route registration; missing configuration fails closed.
	f.app = fiber.New()
	Register(f.app.Group("/integrations/nnct-guidance"), f.db)
	if code, _ := f.request(t, "GET", "", "", nil); code != 503 {
		t.Fatalf("unconfigured status=%d", code)
	}
}

func TestConfigurationAcceptsDifferentSyntheticTermSets(t *testing.T) {
	newFixture(t, false)
	bindings := map[string]uint{"assigned-visit-99": 41, "visit-survey-7": 42, "overall-survey": 43, "sheet-submission": 44}
	data, _ := json.Marshal(bindings)
	t.Setenv("NNCT_GUIDANCE_TASK_BINDINGS", string(data))
	cfg, err := loadConfig()
	if err != nil || len(cfg.keys()) != 4 {
		t.Fatalf("config=%v %v", cfg, err)
	}
}
