package gqlMutation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	graphql "github.com/graphql-go/graphql"
)

// mutationFieldsReports groups user report mutations.
func (h *Handler) mutationFieldsReports(t *gqlH.GqlTypes) graphql.Fields {
	fields := graphql.Fields{}

	// createUserReport: users can report a task with reason and optional comment
	fields["createUserReport"] = &graphql.Field{
		Type: graphql.NewObject(graphql.ObjectConfig{
			Name: "CreateUserReportResult",
			Fields: graphql.Fields{
				"id": &graphql.Field{Type: graphql.Int},
			},
		}),
		Args: graphql.FieldConfigArgument{
			"task_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"reason":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"comment": &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			// Fiber context for auth
			c, ok := gqlH.FiberCtxFromContext(p.Context)
			if !ok {
				return nil, fmt.Errorf("context missing")
			}
			if !utils.UserFromSession(c, h.db, h.rdb) {
				return nil, fmt.Errorf("unauthorized")
			}
			u := c.Locals("user").(*domain.User)
			taskID := uint(p.Args["task_id"].(int))
			reason := strings.TrimSpace(p.Args["reason"].(string))
			var comment *string = nil
			if v, ok := p.Args["comment"].(string); ok {
				vv := strings.TrimSpace(v)
				if vv != "" {
					comment = &vv
				}
			}
			if taskID == 0 || reason == "" || len(reason) > 255 {
				return nil, fmt.Errorf("validation")
			}
			rec := domain.UserReport{ReporterUserID: &u.ID, TaskID: taskID, Reason: reason, Comment: comment, CreatedAt: time.Now()}
			if err := h.db.Create(&rec).Error; err != nil {
				return nil, err
			}
			// Fire-and-forget Discord embeds with admin link
			go func(id uint, tID uint, rsn string) {
				hook := utils.Env("DISCORD_WEBHOOK_URL", "")
				if strings.TrimSpace(hook) == "" {
					return
				}
				// Admin reports URL base
				// Prefer WEB_PUBLIC_URL for web origin; fallback to API_PUBLIC_URL
				web := utils.Env("WEB_PUBLIC_URL", "")
				if strings.TrimSpace(web) == "" {
					web = utils.Env("API_PUBLIC_URL", "http://localhost:4321")
				}
				adminBase := strings.TrimRight(web+"/admin", "/")
				adminURL := fmt.Sprintf("%s/reports?id=%d", adminBase, id)
				embed := map[string]any{
					"title": "New User Report",
					"url":   adminURL,
					"color": 0xFF0000,
					"fields": []map[string]any{
						{"name": "Report ID", "value": fmt.Sprintf("%d", id), "inline": true},
						{"name": "Task ID", "value": fmt.Sprintf("%d", tID), "inline": true},
						{"name": "Reason", "value": rsn, "inline": false},
					},
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				}
				payload := map[string]any{
					"embeds": []any{embed},
				}
				b, _ := json.Marshal(payload)
				req, _ := http.NewRequest("POST", hook, strings.NewReader(string(b)))
				req.Header.Set("Content-Type", "application/json")
				client := &http.Client{Timeout: 5 * time.Second}
				if resp, err := client.Do(req); err == nil && resp != nil {
					_ = resp.Body.Close()
				}
			}(rec.ID, taskID, reason)

			_ = h.db.Create(&domain.Log{UserID: &u.ID, Action: "reports.create"}).Error
			return map[string]any{"id": int(rec.ID)}, nil
		},
	}

	return fields
}
