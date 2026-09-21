package tasklist

import (
	"fmt"
	"strings"
)

var periodValues = map[string]bool{
	"NONE": true, "P1": true, "P2": true, "P3": true, "P4": true, "P5": true,
	"P6": true, "P7": true, "P8": true, "P9": true, "P10": true,
}

// NormalizeDeadlineWindow applies the database enum contract to all Go APIs.
func NormalizeDeadlineWindow(since, until string) (string, string, error) {
	since = strings.ToUpper(strings.TrimSpace(since))
	until = strings.ToUpper(strings.TrimSpace(until))
	if since == "" {
		since = "NONE"
	}
	if until == "" {
		until = "NONE"
	}
	if !periodValues[since] {
		return "", "", fmt.Errorf("invalid deadline_since")
	}
	if !periodValues[until] && until != "MORNING" && until != "LUNCH" && until != "AFTER_SCHOOL" {
		return "", "", fmt.Errorf("invalid deadline_until")
	}
	return since, until, nil
}
