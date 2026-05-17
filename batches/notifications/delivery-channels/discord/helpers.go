package main

import (
	"time"

	dcCommon "github.com/asari194617/taskly/batches/notifications/delivery-channels/common"
)

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func isNextMonth(a, b time.Time) bool { return a.Month() != b.Month() && a.Year() == b.Year() }
func isNextYear(a, b time.Time) bool  { return a.Year() != b.Year() }

func whenLabel(base, target time.Time) string {
	b := dateOnly(base)
	t := dateOnly(target)
	if t.Equal(b) {
		return "今日"
	}
	if t.Equal(b.Add(24 * time.Hour)) {
		return "あす"
	}
	if t.Equal(b.Add(48 * time.Hour)) {
		return "あさって"
	}
	if isNextYear(b, t) {
		return t.Format("2006年01月02日")
	}
	if isNextMonth(b, t) {
		return t.Format("01月02日")
	}
	return t.Format("02日")
}

func deadlineLabel(t dcCommon.Task, d time.Time) string {
	return dcCommon.DeadlineLabel(t.DeadlineSince, t.DeadlineUntil, d)
}

func buildBadges(t dcCommon.Task) []string {
	return dcCommon.BadgeLabels(dcCommon.BuildBadges(t.Type, t.IsRequireHandIn, t.IsSelfCheckRequired))
}
