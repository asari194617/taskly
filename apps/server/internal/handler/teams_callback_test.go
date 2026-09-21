package handler

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestValidateTeamsCallbackURL(t *testing.T) {
	allowed := "https://callback.example.com,https://flow.example.org:8443"
	tests := []struct {
		url string
		ok  bool
	}{
		{"https://callback.example.com/status?task=1", true},
		{"https://flow.example.org:8443/status", true},
		{"http://callback.example.com/status", false},
		{"https://callback.example.com.evil.test/status", false},
		{"https://user:pass@callback.example.com/status", false},
		{"https://127.0.0.1/status", false},
		{"https://callback.example.com:8443/status", false},
		{"https://callback.example.com/status#fragment", false},
		{"not a URL", false},
	}
	for _, tt := range tests {
		err := validateTeamsCallbackURL(tt.url, allowed)
		if (err == nil) != tt.ok {
			t.Errorf("validateTeamsCallbackURL(%q) error = %v, want ok = %t", tt.url, err, tt.ok)
		}
	}
	if err := validateTeamsCallbackURL("https://callback.example.com/status", ""); err == nil {
		t.Fatal("empty allowlist accepted callback")
	}
}

func TestTeamsCallbackRejectsNonPublicDNS(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "192.0.2.1", "::1", "fc00::1", "2001:db8::1", "64:ff9b::a9fe:a9fe"} {
		if isPublicTeamsIP(net.ParseIP(address)) {
			t.Errorf("non-public address %s accepted", address)
		}
	}
	if !isPublicTeamsIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public address rejected")
	}
	client := newTeamsCallbackClient(func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	})
	resp, err := client.Post("https://callback.example.com/status", "application/json", strings.NewReader("{}"))
	if err == nil || resp != nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("private DNS target: response=%v error=%v", resp, err)
	}
	if err := client.CheckRedirect(&http.Request{}, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("redirect policy error = %v", err)
	}
	mixed := newTeamsCallbackClient(func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("10.0.0.1")}}, nil
	})
	resp, err = mixed.Post("https://callback.example.com/status", "application/json", strings.NewReader("{}"))
	if err == nil || resp != nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("mixed DNS targets: response=%v error=%v", resp, err)
	}
}

func TestTeamsInboundRejectsUnapprovedCallbackBeforeDB(t *testing.T) {
	t.Setenv("TEAMS_INTEGRATION_SECRET", "test-secret")
	t.Setenv("TEAMS_CALLBACK_ALLOWED_ORIGINS", "https://callback.example.com")
	app := fiber.New()
	h := &Handler{}
	h.registerTeamsIntegrationRoutes(app.Group("/integrations"))
	req, err := http.NewRequest(http.MethodPost, "http://localhost/integrations/teams/inbound", strings.NewReader(`{"email":"user@example.com","title":"task","subject_id":1,"status_callback":"http://127.0.0.1/private"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-teams-integration-secret", "test-secret")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}
