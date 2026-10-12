package gqlMutation

import "testing"

func TestParseTaskImageKeys(t *testing.T) {
	keys, err := parseTaskImageKeys([]any{" first ", "second"})
	if err != nil || len(keys) != 2 || keys[0] != "first" || keys[1] != "second" {
		t.Fatalf("keys=%v err=%v", keys, err)
	}
	for name, value := range map[string]any{
		"duplicates": []any{"same", "same"},
		"too many":   []string{"1", "2", "3", "4", "5", "6"},
		"blank":      []any{""},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseTaskImageKeys(value); err == nil {
				t.Fatal("expected invalid image keys")
			}
		})
	}
}

func TestRemovedTaskImageKeys(t *testing.T) {
	removed := removedTaskImageKeys([]string{"keep", "remove"}, []string{"keep", "new"})
	if len(removed) != 1 || removed[0] != "remove" {
		t.Fatalf("removed=%v", removed)
	}
}
