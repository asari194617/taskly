package tasklist

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/tasksearch"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"gorm.io/gorm"
)

const MaxLimit = 200

type Options struct {
	Limit          int
	Overdue        bool
	Search         string
	CursorDeadline string
	CursorID       string
	OwnerID        *uint
	MCP            bool
	Offset         int
}

type Cursor struct {
	CursorDeadline string `json:"cursor_deadline"`
	CursorID       string `json:"cursor_id"`
}

// List applies the same visibility, ordering and cursor rules for REST and GraphQL.
func List(db *gorm.DB, userID uint, options Options) ([]domain.Task, *Cursor, error) {
	if options.MCP && options.OwnerID == nil {
		return nil, nil, fmt.Errorf("mcp owner required")
	}
	limit := options.Limit
	maxLimit := MaxLimit
	if options.MCP {
		maxLimit = 500
	}
	if limit <= 0 || limit > maxLimit {
		limit = 20
	}
	var cursorDeadline *time.Time
	if options.CursorDeadline != "" {
		parsed, err := time.Parse("2006-01-02", options.CursorDeadline)
		if err != nil {
			// Accept cursors emitted by older GraphQL deployments during rollout.
			parsed, err = time.Parse(time.RFC3339, options.CursorDeadline)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid cursor_deadline")
			}
		}
		day := utils.DateOnly(parsed)
		cursorDeadline = &day
	}
	var cursorID uint64
	if options.CursorID != "" {
		parsed, err := strconv.ParseUint(options.CursorID, 10, 64)
		if err != nil || parsed == 0 {
			return nil, nil, fmt.Errorf("invalid cursor_id")
		}
		cursorID = parsed
	}
	if cursorDeadline != nil && cursorID == 0 {
		return nil, nil, fmt.Errorf("cursor_id required")
	}

	query := db.Model(&domain.Task{})
	if options.OwnerID != nil {
		query = query.Where("added_user_id = ? AND is_hidden = 0", *options.OwnerID)
	}
	isAdmin, err := utils.UserHasRole(userID, "admin", db)
	if err != nil {
		return nil, nil, err
	}
	isManager, err := utils.UserHasRole(userID, "manager", db)
	if err != nil {
		return nil, nil, err
	}
	if !isAdmin && !isManager {
		query = query.Where("is_hidden = 0")
	}
	today := utils.DateOnly(time.Now()).Format("2006-01-02")
	if options.MCP {
		// MCP keeps its established owner-only, ID-descending offset contract.
	} else if options.Overdue {
		query = query.Where("effective_deadline < ?", today)
	} else {
		query = query.Where("effective_deadline >= ?", today)
	}
	search := strings.TrimSpace(options.Search)
	if client := tasksearch.Current(); search != "" && !options.MCP && !strings.ContainsAny(search, "%_") && client.Ready() && (cursorID == 0 || cursorDeadline != nil) {
		items, next, err := searchIndex(client, query, search, options, today, !isAdmin && !isManager, cursorDeadline, cursorID, limit)
		if err == nil {
			return items, next, nil
		}
		// Keep search available while the index is unavailable or being repaired.
		client.MarkUnavailable()
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("(title LIKE ? OR description LIKE ?)", like, like)
	}
	if options.MCP {
		if options.Offset > 0 {
			query = query.Offset(options.Offset)
		}
	} else if cursorDeadline != nil {
		cursorDate := cursorDeadline.Format("2006-01-02")
		if options.Overdue {
			query = query.Where("(effective_deadline < ?) OR (effective_deadline = ? AND id < ?)", cursorDate, cursorDate, cursorID)
		} else {
			query = query.Where("(effective_deadline > ?) OR (effective_deadline = ? AND id > ?)", cursorDate, cursorDate, cursorID)
		}
	} else if cursorID != 0 {
		if options.Overdue {
			query = query.Where("id < ?", cursorID)
		} else {
			query = query.Where("id > ?", cursorID)
		}
	}
	if options.MCP {
		query = query.Order("id DESC")
	} else if options.Overdue {
		query = query.Order("effective_deadline DESC").Order("id DESC")
	} else {
		query = query.Order("effective_deadline ASC").Order("id ASC")
	}
	var items []domain.Task
	if err := query.Limit(limit + 1).Find(&items).Error; err != nil {
		return nil, nil, err
	}
	if options.MCP {
		if len(items) > limit {
			items = items[:limit]
		}
		return items, nil, nil
	}
	if len(items) <= limit {
		return items, nil, nil
	}
	items = items[:limit]
	last := items[len(items)-1]
	return items, &Cursor{CursorDeadline: last.Deadline.Format("2006-01-02"), CursorID: strconv.FormatUint(uint64(last.ID), 10)}, nil
}

func searchIndex(client *tasksearch.Client, query *gorm.DB, search string, options Options, today string, visibleOnly bool, cursorDeadline *time.Time, cursorID uint64, limit int) ([]domain.Task, *Cursor, error) {
	like := "%" + search + "%"
	request := tasksearch.SearchOptions{Query: search, Overdue: options.Overdue, Today: today,
		VisibleOnly: visibleOnly, Size: limit*2 + 1}
	if cursorDeadline != nil {
		request.CursorDeadline = cursorDeadline.Format("2006-01-02")
		request.CursorID = cursorID
	}
	items := make([]domain.Task, 0, limit+1)
	for len(items) <= limit {
		hits, err := client.Search(context.Background(), request)
		if err != nil {
			return nil, nil, err
		}
		if len(hits) == 0 {
			break
		}
		ids := make([]uint, 0, len(hits))
		for _, hit := range hits {
			ids = append(ids, hit.ID)
		}
		var matched []domain.Task
		if err := query.Where("id IN ? AND (title LIKE ? OR description LIKE ?)", ids, like, like).Find(&matched).Error; err != nil {
			return nil, nil, err
		}
		byID := make(map[uint]domain.Task, len(matched))
		for _, task := range matched {
			byID[task.ID] = task
		}
		for _, hit := range hits {
			if task, ok := byID[hit.ID]; ok {
				items = append(items, task)
			}
			if len(items) > limit {
				break
			}
		}
		if len(items) > limit || len(hits) < request.Size {
			break
		}
		request.After = hits[len(hits)-1].Sort
	}
	if len(items) <= limit {
		return items, nil, nil
	}
	items = items[:limit]
	last := items[len(items)-1]
	return items, &Cursor{CursorDeadline: last.Deadline.Format("2006-01-02"), CursorID: strconv.FormatUint(uint64(last.ID), 10)}, nil
}
