package utils

import (
	"strconv"
	"testing"
)

func TestToUintRejectsOutOfRangeAndInvalidIDs(t *testing.T) {
	max := ^uint(0)
	if got := ToUint(strconv.FormatUint(uint64(max), 10)); got != max {
		t.Fatalf("maximum uint: got %d, want %d", got, max)
	}
	for _, raw := range []string{"-1", "1.5", "12invalid", "18446744073709551616", ""} {
		if got := ToUint(raw); got != 0 {
			t.Errorf("ToUint(%q) = %d, want 0", raw, got)
		}
	}
	if strconv.IntSize == 32 && ToUint("4294967296") != 0 {
		t.Fatal("32-bit overflow must not wrap")
	}
}
