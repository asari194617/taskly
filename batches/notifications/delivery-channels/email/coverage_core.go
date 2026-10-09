package main

import (
	"encoding/json"
	"regexp"
	"strings"

	dcCommon "github.com/asari194617/taskly/batches/notifications/delivery-channels/common"
)

// This file contains deterministic digest composition. The network transport
// remains in composer.go and stays outside the unit-coverage percentage.
func buildComposePayload(locale, appName string, items []dcCommon.QueueItem) composePayload {
	out := composePayload{Locale: locale, AppName: appName}
	for _, it := range items {
		when := it.WhenLabel
		body := it.Body
		if locale == "en" {
			when = englishDeadlineLabel(when)
			body = strings.ReplaceAll(body, "締切: ", "Due: ")
			if it.WhenLabel != "" {
				body = strings.ReplaceAll(body, it.WhenLabel, when)
			}
		}
		m := map[string]any{
			"when_label": when,
			"subject":    it.Subject,
			"title":      it.Title,
			"body":       body,
			"completed":  it.Completed,
		}
		if it.Meta.Valid {
			m["meta"] = it.Meta.String
		}
		if it.Badges.Valid && strings.TrimSpace(it.Badges.String) != "" {
			var arr []map[string]any
			if err := json.Unmarshal([]byte(it.Badges.String), &arr); err == nil {
				if locale == "en" {
					labels := map[string]string{"オンライン": "Online", "対面": "In person", "小テスト": "Quiz", "発表": "Presentation", "自主学習": "Self study", "提出必須": "Submission required", "自己採点": "Self check"}
					for _, badge := range arr {
						if label, ok := badge["label"].(string); ok {
							if translated, found := labels[label]; found {
								badge["label"] = translated
							}
						}
					}
				}
				m["badges"] = arr
			}
		}
		out.Items = append(out.Items, m)
	}
	return out
}

var periodLabel = regexp.MustCompile(`(\d+)(?:〜(\d+))?限まで`)

func englishDeadlineLabel(label string) string {
	label = strings.ReplaceAll(label, "朝まで", "by morning")
	label = strings.ReplaceAll(label, "昼休みまで", "by lunch")
	label = strings.ReplaceAll(label, "放課後まで", "after school")
	label = periodLabel.ReplaceAllStringFunc(label, func(period string) string {
		match := periodLabel.FindStringSubmatch(period)
		if match[2] != "" {
			return "by periods " + match[1] + "–" + match[2]
		}
		return "by period " + match[1]
	})
	return strings.ReplaceAll(label, "まで", "due")
}
