package main

import (
	"testing"
	"time"
)

func TestWhenLabel(t *testing.T) {
	base := time.Date(2025, 10, 16, 9, 0, 0, 0, time.Local)
	cases := []struct {
		target time.Time
		want   string
	}{
		{base, "今日"},
		{base.Add(24 * time.Hour), "あす"},
		{base.Add(48 * time.Hour), "あさって"},
		{time.Date(2025, 11, 1, 0, 0, 0, 0, time.Local), "11月01日"},
		{time.Date(2026, 1, 3, 0, 0, 0, 0, time.Local), "2026年01月03日"},
	}
	for _, c := range cases {
		got := whenLabel(base, c.target)
		if got != c.want {
			t.Errorf("whenLabel(%v)->%q want %q", c.target, got, c.want)
		}
	}
}

func TestDeadlineLabel(t *testing.T) {
	baseDate := time.Date(2025, 10, 16, 0, 0, 0, 0, time.Local)
	mk := func(since, until string) Task { return Task{DeadlineSince: since, DeadlineUntil: until} }
	cases := []struct {
		task Task
		want string
	}{
		{mk("", "P5"), "10/16 5限まで"},
		{mk("P3", "P5"), "10/16 3〜5限まで"},
		{mk("", "MORNING"), "10/16 朝まで"},
		{mk("", ""), "10/16 まで"},
	}
	for _, c := range cases {
		got := deadlineLabel(c.task, baseDate)
		if got != c.want {
			t.Errorf("deadlineLabel(%v)->%q want %q", c.task, got, c.want)
		}
	}
}
