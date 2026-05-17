package common

import (
	"fmt"
	"strings"
	"time"
)

func DeadlineLabel(deadlineSince, deadlineUntil string, d time.Time) string {
	date := d.Format("1/2")
	label := DeriveDeadlineLabelEnum(deadlineSince, deadlineUntil)
	if strings.TrimSpace(label) == "" {
		return date + " まで"
	}
	return date + " " + label
}

func DeriveDeadlineLabelEnum(since, until string) string {
	sIdx := PeriodIndex(since)
	uIdx := PeriodIndex(until)
	if strings.EqualFold(until, "MORNING") {
		return "朝まで"
	}
	if strings.EqualFold(until, "LUNCH") {
		return "昼休みまで"
	}
	if strings.EqualFold(until, "AFTER_SCHOOL") {
		return "放課後まで"
	}
	if uIdx > 0 && (sIdx == 0 || sIdx == uIdx) {
		return fmt.Sprintf("%d限まで", uIdx)
	}
	if sIdx > 0 && uIdx > 0 {
		if sIdx == uIdx {
			return fmt.Sprintf("%d限まで", uIdx)
		}
		if sIdx < uIdx {
			return fmt.Sprintf("%d〜%d限まで", sIdx, uIdx)
		}
	}
	return ""
}
