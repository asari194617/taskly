package main

import "testing"

func TestHarnessRefusesSharedOrExternalServices(t *testing.T) {
	goodDSN := "test:synthetic@tcp(127.0.0.1:3306)/taskly_guidance_e2e?parseTime=true"
	for _, tc := range []struct{ enabled, node, addr, dsn, issuer string }{
		{"", "test", "127.0.0.1:3374", goodDSN, "http://127.0.0.1:3372"},
		{"1", " PRODUCTION ", "127.0.0.1:3374", goodDSN, "http://127.0.0.1:3372"},
		{"1", "test", "0.0.0.0:3374", goodDSN, "http://127.0.0.1:3372"},
		{"1", "test", "127.0.0.1:0", goodDSN, "http://127.0.0.1:3372"},
		{"1", "test", "127.0.0.1:3374", "test:synthetic@tcp(127.0.0.1:3306)/taskly", "http://127.0.0.1:3372"},
		{"1", "test", "127.0.0.1:3374", "test:synthetic@tcp(db.example.test:3306)/taskly_guidance_e2e", "http://127.0.0.1:3372"},
		{"1", "test", "127.0.0.1:3374", goodDSN, "https://auth.example.test"},
		{"1", "test", "127.0.0.1:3374", goodDSN, "http://name:secret@127.0.0.1:3372"},
		{"1", "test", "127.0.0.1:3374", goodDSN, "http://127.0.0.1:3372?token=secret"},
	} {
		if _, err := validateEnvironment(tc.enabled, tc.node, tc.addr, tc.dsn, tc.issuer); err == nil {
			t.Fatalf("unsafe environment accepted: %s %s", tc.node, tc.addr)
		}
	}
	cfg, err := validateEnvironment("1", "test", "127.0.0.1:3374", goodDSN, "http://127.0.0.1:3372")
	if err != nil || !cfg.ParseTime || !cfg.MultiStatements {
		t.Fatalf("safe environment rejected %v", err)
	}
}
