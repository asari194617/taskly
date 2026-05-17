package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func hostPort(h, p string) string {
	if strings.TrimSpace(p) == "" {
		p = "25"
	}
	return fmt.Sprintf("%s:%s", valueOr(h, "localhost"), p)
}

func valueOr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// buildDSN builds a MySQL DSN from MySQLConfig
func buildDSN(cfg MySQLConfig) string {
	params := cfg.Params
	if strings.TrimSpace(params) == "" {
		params = "parseTime=true&charset=utf8mb4&loc=Local"
	}
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?%s",
		valueOr(cfg.User, ""), valueOr(cfg.Password, ""), hostPort(cfg.Host, cfg.Port), valueOr(cfg.Database, ""), params)
}

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

func buildBodyFromTask(t Task) string {
	d := deadlineLabel(t, t.Deadline)
	return fmt.Sprintf("締切: %s\n", d)
}

func deadlineLabel(t Task, d time.Time) string {
	date := d.Format("1/2")
	label := deriveDeadlineLabelEnum(t.DeadlineSince, t.DeadlineUntil)
	if strings.TrimSpace(label) == "" {
		return date + " まで"
	}
	return date + " " + label
}

func deriveDeadlineLabelEnum(since, until string) string {
	sIdx := periodIndex(since)
	uIdx := periodIndex(until)
	if strings.EqualFold(until, "MORNING") {
		return "朝まで"
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
	if strings.EqualFold(until, "LUNCH") {
		return "昼休みまで"
	}
	if strings.EqualFold(until, "AFTER_SCHOOL") {
		return "放課後まで"
	}
	return ""
}

func periodIndex(s string) int {
	s = strings.ToUpper(strings.TrimSpace(s))
	if strings.HasPrefix(s, "P") {
		if num, err := strconv.Atoi(s[1:]); err == nil && num >= 1 {
			return num
		}
	}
	return 0
}

// buildBadgesJSON builds badges JSON based on task attributes
func buildBadgesJSON(t Task) string {
	type badge struct {
		Label   string `json:"label"`
		Variant string `json:"variant,omitempty"`
	}
	out := make([]badge, 0, 3)
	if lbl := displayTaskTypeJaLocal(t.Type); strings.TrimSpace(lbl) != "" && strings.ToUpper(strings.TrimSpace(t.Type)) != "OTHER" {
		out = append(out, badge{Label: lbl, Variant: "gray"})
	}
	if t.IsRequireHandIn {
		out = append(out, badge{Label: "提出必須", Variant: "red"})
	}
	if t.IsSelfCheckRequired {
		out = append(out, badge{Label: "自己採点", Variant: "blue"})
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func displayTaskTypeJaLocal(e string) string {
	switch strings.ToUpper(strings.TrimSpace(e)) {
	case "ONLINE":
		return "オンライン"
	case "FACE_TO_FACE":
		return "対面"
	case "QUIZ":
		return "小テスト"
	case "PRESENTATION":
		return "発表"
	case "SELF_STUDY":
		return "自主学習"
	default:
		return ""
	}
}
