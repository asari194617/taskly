package main

import (
	"fmt"
	"strings"

	dcCommon "github.com/asari194617/taskly/batches/notifications/delivery-channels/common"
)

// helpers
func parseInt(s string) int {
	i := 0
	fmt.Sscanf(strings.TrimSpace(s), "%d", &i)
	if i <= 0 {
		i = 60
	}
	return i
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func buildBodyFromTask(t dcCommon.Task) string {
	d := dcCommon.DeadlineLabel(t.DeadlineSince, t.DeadlineUntil, t.Deadline)
	return fmt.Sprintf("締切: %s\n", d)
}

// buildBadgesJSON builds badges JSON based on task attributes
func buildBadgesJSON(t dcCommon.Task) string {
	return dcCommon.BadgesToJSON(dcCommon.BuildBadges(t.Type, t.IsRequireHandIn, t.IsSelfCheckRequired))
}
