package utils

import (
	"context"
	"fmt"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/getsentry/sentry-go"
	"gorm.io/gorm"
	"strings"
	"time"
)

// resolveDayShiftTodayAndNextMembers returns the users for today and the
// "next" day based on the provided base index and current cycle. baseIndex is
// interpreted the same way as in the Airflow DAGs: it is 0-based and used with
// modulo over the ordered groups.
func ResolveDayShiftTodayAndNextMembers(today time.Time, baseIndex int, DB *gorm.DB) (todayMembers []*domain.User, nextMembers []*domain.User, err error) {
	// Detect current cycle by date (same as DAG behavior)
	todayDate := today.Format("2006-01-02")
	var cidRow struct{ ID uint }
	if err = DB.Raw("SELECT id AS id FROM day_shift_cycles WHERE begin_date <= ? AND finish_date >= ? ORDER BY finish_date DESC, id DESC LIMIT 1", todayDate, todayDate).Scan(&cidRow).Error; err != nil {
		return nil, nil, err
	}
	if cidRow.ID == 0 {
		return []*domain.User{}, []*domain.User{}, nil
	}
	cycleID := cidRow.ID

	// Load ordered groups for the cycle
	var groups []struct {
		ID       uint
		Position int
	}
	if err = DB.Raw("SELECT id AS id, position AS position FROM day_shift_cycle_groups WHERE cycle_id=? ORDER BY position ASC, id ASC", cycleID).Scan(&groups).Error; err != nil {
		return nil, nil, err
	}
	if len(groups) == 0 {
		return []*domain.User{}, []*domain.User{}, nil
	}

	// configs.day_shift_current_index は 0 始まりのインデックスですが、
	// 実運用上「現在の担当」を 1 始まりのカウンタとして扱っているため、
	// ここでは 1 を引いた値を基準として today/next を解決する。
	// これにより、Airflow 側のインクリメントロジックと突き合わせたときに
	// 「今日」と「次のメンバー」が 1 つ後ろにずれる問題を防ぐ。
	idx := baseIndex - 1
	if idx < 0 {
		idx = 0
	}
	giToday := idx % len(groups)
	giNext := (idx + 1) % len(groups)
	gidToday := groups[giToday].ID
	gidNext := groups[giNext].ID

	// helper to load members for a group
	loadMembers := func(gid uint) ([]*domain.User, error) {
		var memberRows []struct {
			UserID uint
		}
		if err := DB.Raw("SELECT user_id AS user_id FROM day_shift_cycle_group_members WHERE group_id=? ORDER BY position ASC, id ASC", gid).Scan(&memberRows).Error; err != nil {
			return nil, err
		}
		if len(memberRows) == 0 {
			return []*domain.User{}, nil
		}
		ids := make([]uint, 0, len(memberRows))
		for _, mr := range memberRows {
			ids = append(ids, mr.UserID)
		}
		var users []*domain.User
		if err := DB.Where("id IN ? AND is_deleted = 0", ids).Find(&users).Error; err != nil {
			return nil, err
		}
		return users, nil
	}

	if todayMembers, err = loadMembers(gidToday); err != nil {
		return nil, nil, err
	}
	if nextMembers, err = loadMembers(gidNext); err != nil {
		return nil, nil, err
	}
	return todayMembers, nextMembers, nil
}

// Joins display names (falling back to email) for logs/Discord/email.
func JoinUserNames(users []*domain.User) string {
	names := make([]string, 0, len(users))
	for _, u := range users {
		name := strings.TrimSpace(u.DisplayName)
		if name == "" {
			name = u.Email
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

// SendDayShiftSkipEmail is currently a lightweight stub. The actual email
// rendering is handled by the email-compose service; here we simply log the
// intended recipients and dates for observability. Wiring a full mail send
// pipeline can be done later by following the composeLoginCodeGRPC pattern.
func SendDayShiftSkipEmail(ctx context.Context, today, next time.Time, todayMembers, nextMembers []*domain.User) error {
	subject := fmt.Sprintf("[Taskly] 日直スキップ: %s", today.Format("2006-01-02"))
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "dayshift_skip_email",
		Message:  subject,
		Data: map[string]any{
			"today":        today.Format("2006-01-02"),
			"next":         next.Format("2006-01-02"),
			"todayMembers": JoinUserNames(todayMembers),
			"nextMembers":  JoinUserNames(nextMembers),
		},
		Level: sentry.LevelInfo,
	})
	return nil
}

func PtrTime(t time.Time) *time.Time { return &t }
