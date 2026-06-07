package main

import (
	"testing"

	utils "github.com/asari194617/taskly/apps/server/internal/utils"
)

func TestICalEscape(t *testing.T) {
	in := "Hello, World; Line\\Break\nNext"
	out := utils.ICalEscape(in)
	if out == in {
		t.Fatalf("expected escaped string, got same")
	}
	if out == "" {
		t.Fatalf("expected non-empty")
	}
}

func TestGenerateIcalToken(t *testing.T) {
	a := utils.GenerateIcalToken()
	b := utils.GenerateIcalToken()
	if a == b {
		t.Fatalf("expected different tokens")
	}
}
