package utils

import "testing"

func TestICalEscape(t *testing.T) {
	in := "Hello, World; Line\\Break\nNext"
	out := ICalEscape(in)
	if out == in {
		t.Fatalf("expected escaped string, got same")
	}
	if out == "" {
		t.Fatalf("expected non-empty")
	}
}

func TestGenerateIcalToken(t *testing.T) {
	a := GenerateIcalToken()
	b := GenerateIcalToken()
	if a == b {
		t.Fatalf("expected different tokens")
	}
}