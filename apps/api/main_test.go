package main

import (
	"testing"
)

func TestComputePairs(t *testing.T) {
	users := []User{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}}
	pairs := computePairs(users)
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
	if pairs[0][0] != 1 || pairs[0][1] != 2 {
		t.Fatalf("unexpected first pair: %+v", pairs[0])
	}
	if pairs[1][0] != 3 || pairs[1][1] != 4 {
		t.Fatalf("unexpected second pair: %+v", pairs[1])
	}

	users = []User{{ID: 1}, {ID: 2}, {ID: 3}}
	pairs = computePairs(users)
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair when odd, got %d", len(pairs))
	}
}

func TestIcalEscape(t *testing.T) {
	in := "Hello, World; Line\\Break\nNext"
	out := icalEscape(in)
	if out == in {
		t.Fatalf("expected escaped string, got same")
	}
	if out == "" {
		t.Fatalf("expected non-empty")
	}
}

func TestGenerateIcalToken(t *testing.T) {
	a := generateIcalToken()
	b := generateIcalToken()
	if a == b {
		t.Fatalf("expected different tokens")
	}
}
