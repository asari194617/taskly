package utils

import (
	"fmt"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"gorm.io/gorm"
)

// iCal builder and helpers
func BuildICSForUser(userID uint, DB *gorm.DB) (string, error) {
	var user domain.User
	if err := DB.Select("notification_lang").First(&user, userID).Error; err != nil {
		return "", fmt.Errorf("load iCal user: %w", err)
	}
	lang := strings.ToLower(strings.TrimSpace(user.NotificationLang))
	if lang == "" {
		lang = "ja"
	}

	// Include every task. A fixed LIMIT silently drops events from subscribed calendars.
	var tasks []struct {
		domain.Task   `gorm:"embedded"`
		SubjectNameJa *string `gorm:"column:subject_name_ja"`
		SubjectNameEn *string `gorm:"column:subject_name_en"`
	}
	if err := DB.Model(&domain.Task{}).
		Select("tasks.*, subjects.name_ja AS subject_name_ja, subjects.name_en AS subject_name_en").
		Joins("LEFT JOIN subjects ON subjects.id = tasks.subject_id").
		Where("tasks.added_user_id = ?", userID).
		Order(EffectiveDeadlineExpr("tasks") + " ASC, tasks.id ASC").
		Find(&tasks).Error; err != nil {
		return "", fmt.Errorf("load iCal tasks: %w", err)
	}

	b := &strings.Builder{}
	crlf := "\r\n"
	now := time.Now().UTC().Format("20060102T150405Z")
	b.WriteString("BEGIN:VCALENDAR" + crlf)
	b.WriteString("VERSION:2.0" + crlf)
	b.WriteString("PRODID:-//Taskly//EN" + crlf)
	b.WriteString("CALSCALE:GREGORIAN" + crlf)
	b.WriteString("METHOD:PUBLISH" + crlf)
	for _, row := range tasks {
		t := row.Task
		// Treat tasks as all-day events in the calendar (use date-only DTSTART/DTEND)
		// Use the task.Deadline (date only) as the day; DTEND is exclusive and set to next day
		if t.Deadline.IsZero() {
			continue
		}

		sName := ""
		if row.SubjectNameJa != nil {
			sName = *row.SubjectNameJa
		}
		if lang == "en" && row.SubjectNameEn != nil && *row.SubjectNameEn != "" {
			sName = *row.SubjectNameEn
		}
		b.WriteString("BEGIN:VEVENT" + crlf)
		b.WriteString(fmt.Sprintf("UID:tsk-%d@taskly\r\n", t.ID))
		b.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", now))
		b.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", ICalEscape("[課題] "+sName+": "+t.Title)))
		// Build description: include existing description and human-readable period label
		desc := ""
		if t.Description != nil && strings.TrimSpace(*t.Description) != "" {
			// Replace newlines with spaces per UX requirement
			desc = strings.TrimSpace(strings.ReplaceAll(*t.Description, "\n", " "))
		}
		// Build compact period label
		label := ""
		sIdx := periodIndex(t.DeadlineSince)
		uIdx := periodIndex(t.DeadlineUntil)
		if strings.EqualFold(strings.TrimSpace(t.DeadlineUntil), "MORNING") {
			label = "朝まで"
		} else if strings.EqualFold(strings.TrimSpace(t.DeadlineUntil), "LUNCH") {
			label = "昼休みまで"
		} else if strings.EqualFold(strings.TrimSpace(t.DeadlineUntil), "AFTER_SCHOOL") {
			label = "放課後まで"
		} else if sIdx > 0 && uIdx > 0 {
			if sIdx == uIdx {
				label = fmt.Sprintf("%d限まで", sIdx)
			} else if sIdx < uIdx {
				label = fmt.Sprintf("%d〜%d限まで", sIdx, uIdx)
			}
		} else if uIdx > 0 {
			label = fmt.Sprintf("%d限まで", uIdx)
		} else if sIdx > 0 {
			label = fmt.Sprintf("%d限以降", sIdx)
		}
		if strings.TrimSpace(label) != "" {
			if desc != "" {
				desc = desc + " " + label
			} else {
				desc = label
			}
		}
		b.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", ICalEscape(desc)))
		// All-day DTSTART/DTEND (DTEND is exclusive — next day)
		dtStart := t.Deadline.Format("20060102")
		dtEnd := t.Deadline.AddDate(0, 0, 1).Format("20060102")
		b.WriteString(fmt.Sprintf("DTSTART;VALUE=DATE:%s\r\n", dtStart))
		b.WriteString(fmt.Sprintf("DTEND;VALUE=DATE:%s\r\n", dtEnd))
		b.WriteString("END:VEVENT" + crlf)
	}
	b.WriteString("END:VCALENDAR" + crlf)
	return b.String(), nil
}

func ICalEscape(s string) string {
	r := strings.ReplaceAll(s, "\\", "\\\\")
	r = strings.ReplaceAll(r, ",", "\\,")
	r = strings.ReplaceAll(r, ";", "\\;")
	r = strings.ReplaceAll(r, "\n", "\\n")
	r = strings.ReplaceAll(r, "\r", "")
	return r
}
