package common

import (
	"encoding/json"
	"strconv"
	"strings"
)

type Badge struct {
	Label   string
	Variant string
}

func PeriodIndex(s string) int {
	s = strings.ToUpper(strings.TrimSpace(s))
	if strings.HasPrefix(s, "P") {
		if num, err := strconv.Atoi(s[1:]); err == nil && num >= 1 {
			return num
		}
	}
	return 0
}

func DisplayTaskTypeJaLocal(e string) string {
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

func BuildBadges(taskType string, isRequireHandIn, isSelfCheckRequired bool) []Badge {
	out := make([]Badge, 0, 3)
	if lbl := DisplayTaskTypeJaLocal(taskType); strings.TrimSpace(lbl) != "" && strings.ToUpper(strings.TrimSpace(taskType)) != "OTHER" {
		out = append(out, Badge{Label: lbl, Variant: "gray"})
	}
	if isRequireHandIn {
		out = append(out, Badge{Label: "提出必須", Variant: "red"})
	}
	if isSelfCheckRequired {
		out = append(out, Badge{Label: "自己採点", Variant: "blue"})
	}
	return out
}

// BadgeLabels returns the labels of the badges in order.
func BadgeLabels(badges []Badge) []string {
	out := make([]string, 0, len(badges))
	for _, b := range badges {
		out = append(out, b.Label)
	}
	return out
}

// BadgesToJSON marshals badges to a compact JSON string containing label and variant.
func BadgesToJSON(badges []Badge) string {
	type jBadge struct {
		Label   string `json:"label"`
		Variant string `json:"variant,omitempty"`
	}
	out := make([]jBadge, 0, len(badges))
	for _, b := range badges {
		out = append(out, jBadge{Label: b.Label, Variant: b.Variant})
	}
	if jb, err := json.Marshal(out); err == nil {
		return string(jb)
	}
	return "[]"
}
