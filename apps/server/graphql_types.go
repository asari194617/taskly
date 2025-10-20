package main

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

// gqlTypes groups GraphQL object types used across Query and Mutation builders.
type gqlTypes struct {
	AdminUserType          *graphql.Object
	SubjectType            *graphql.Object
	TaskType               *graphql.Object
	MeType                 *graphql.Object
	AuthSettingsType       *graphql.Object
	ApiKeyType             *graphql.Object
	IcalTokenType          *graphql.Object
	McpInfoType            *graphql.Object
	LogItemType            *graphql.Object
	DayShiftCycleType      *graphql.Object
	DayShiftAssignmentType *graphql.Object
	DayShiftGroupType      *graphql.Object
	DayShiftCycleListType  *graphql.Object
	DayShiftGroupListType  *graphql.Object
}

// createGQLTypes constructs all GraphQL types. Keep logic identical to original definitions.
func createGQLTypes() *gqlTypes {
	adminUserType := graphql.NewObject(graphql.ObjectConfig{
		Name: "AdminUser",
		Fields: graphql.Fields{
			"id":                  &graphql.Field{Type: graphql.Int},
			"email":               &graphql.Field{Type: graphql.String},
			"display_name":        &graphql.Field{Type: graphql.String},
			"notification_lang":   &graphql.Field{Type: graphql.String},
			"email_login_enabled": &graphql.Field{Type: graphql.Boolean},
			"roles":               &graphql.Field{Type: graphql.NewList(graphql.String)},
			"is_blocked":          &graphql.Field{Type: graphql.Boolean},
		},
	})

	subjectType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Subject",
		Fields: graphql.Fields{
			"id":      &graphql.Field{Type: graphql.Int},
			"name_ja": &graphql.Field{Type: graphql.String},
			"name_en": &graphql.Field{Type: graphql.String},
			"name": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
				// Resolve localized name based on current user's notification language
				var subj Subject
				if m, ok := p.Source.(map[string]any); ok {
					if idv, ok2 := m["id"]; ok2 {
						var idUint uint
						switch vv := idv.(type) {
						case int:
							idUint = uint(vv)
						case uint:
							idUint = vv
						case int64:
							idUint = uint(vv)
						case float64:
							idUint = uint(vv)
						}
						_ = DB.First(&subj, idUint).Error
					}
				}
				lang := "ja"
				if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
					if u, ok2 := c.Locals("user").(*User); ok2 && strings.TrimSpace(u.NotificationLang) != "" {
						lang = strings.ToLower(u.NotificationLang)
					}
				}
				if lang == "en" {
					if subj.NameEn != nil && *subj.NameEn != "" {
						return *subj.NameEn, nil
					}
				}
				if strings.TrimSpace(subj.NameJa) != "" {
					return subj.NameJa, nil
				}
				return "", nil
			}},
		},
	})

	taskType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Task",
		Fields: graphql.Fields{
			"id":                  &graphql.Field{Type: graphql.Int},
			"title":               &graphql.Field{Type: graphql.String},
			"subject_id":          &graphql.Field{Type: graphql.Int},
			"description":         &graphql.Field{Type: graphql.String},
			"type":                &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"is_require_hand_in":  &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"self_check_required": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"deadline":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)}, // YYYY-MM-DD
			"deadline_since":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)}, // ENUM
			"deadline_until":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)}, // ENUM
			"created_at":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"updated_at":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	meType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Me",
		Fields: graphql.Fields{
			"id":                 &graphql.Field{Type: graphql.Int},
			"email":              &graphql.Field{Type: graphql.String},
			"display_name":       &graphql.Field{Type: graphql.String},
			"is_admin":           &graphql.Field{Type: graphql.Boolean},
			"is_manager":         &graphql.Field{Type: graphql.Boolean},
			"is_registered":      &graphql.Field{Type: graphql.Boolean},
			"is_agreed_policies": &graphql.Field{Type: graphql.Boolean},
		},
	})

	authSettingsType := graphql.NewObject(graphql.ObjectConfig{
		Name: "AuthSettings",
		Fields: graphql.Fields{
			"email_login_enabled": &graphql.Field{Type: graphql.Boolean},
			"has_password":        &graphql.Field{Type: graphql.Boolean},
			"totp_enabled":        &graphql.Field{Type: graphql.Boolean},
		},
	})

	apiKeyType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ApiKey",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.Int},
			"description": &graphql.Field{Type: graphql.String},
			"is_valid":    &graphql.Field{Type: graphql.Boolean},
			"created_at":  &graphql.Field{Type: graphql.String},
		},
	})

	icalTokenType := graphql.NewObject(graphql.ObjectConfig{
		Name: "IcalToken",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.Int},
			"description": &graphql.Field{Type: graphql.String},
			"url":         &graphql.Field{Type: graphql.String},
			"created_at":  &graphql.Field{Type: graphql.String},
		},
	})

	mcpInfoType := graphql.NewObject(graphql.ObjectConfig{
		Name: "McpInfo",
		Fields: graphql.Fields{
			"enabled":   &graphql.Field{Type: graphql.Boolean},
			"has_token": &graphql.Field{Type: graphql.Boolean},
		},
	})

	logItemType := graphql.NewObject(graphql.ObjectConfig{
		Name: "LogItem",
		Fields: graphql.Fields{
			"action":     &graphql.Field{Type: graphql.String},
			"created_at": &graphql.Field{Type: graphql.String},
		},
	})

	dayShiftCycleType := graphql.NewObject(graphql.ObjectConfig{
		Name: "DayShiftCycle",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.Int},
			"begin_date":  &graphql.Field{Type: graphql.String},
			"finish_date": &graphql.Field{Type: graphql.String},
			"description": &graphql.Field{Type: graphql.String},
		},
	})
	dayShiftAssignmentType := graphql.NewObject(graphql.ObjectConfig{
		Name: "DayShiftAssignment",
		Fields: graphql.Fields{
			"id":       &graphql.Field{Type: graphql.Int},
			"date":     &graphql.Field{Type: graphql.String},
			"cycle_id": &graphql.Field{Type: graphql.Int},
			"status":   &graphql.Field{Type: graphql.String},
			"user_ids": &graphql.Field{Type: graphql.NewList(graphql.Int)},
		},
	})
	dayShiftGroupType := graphql.NewObject(graphql.ObjectConfig{
		Name: "DayShiftGroup",
		Fields: graphql.Fields{
			"id":       &graphql.Field{Type: graphql.Int},
			"cycle_id": &graphql.Field{Type: graphql.Int},
			"position": &graphql.Field{Type: graphql.Int},
			"user_ids": &graphql.Field{Type: graphql.NewList(graphql.Int)},
		},
	})

	// Pagination result types
	pageInfoFields := graphql.Fields{
		"items":     &graphql.Field{Type: graphql.NewList(graphql.String)}, // placeholder
		"page":      &graphql.Field{Type: graphql.Int},
		"page_size": &graphql.Field{Type: graphql.Int},
		"total":     &graphql.Field{Type: graphql.Int},
		"has_next":  &graphql.Field{Type: graphql.Boolean},
		"has_prev":  &graphql.Field{Type: graphql.Boolean},
	}
	dayShiftCycleListType := graphql.NewObject(graphql.ObjectConfig{
		Name: "DayShiftCycleList",
		Fields: graphql.Fields{
			"items":     &graphql.Field{Type: graphql.NewList(dayShiftCycleType)},
			"page":      pageInfoFields["page"],
			"page_size": pageInfoFields["page_size"],
			"total":     pageInfoFields["total"],
			"has_next":  pageInfoFields["has_next"],
			"has_prev":  pageInfoFields["has_prev"],
		},
	})
	dayShiftGroupListType := graphql.NewObject(graphql.ObjectConfig{
		Name: "DayShiftGroupList",
		Fields: graphql.Fields{
			"items":     &graphql.Field{Type: graphql.NewList(dayShiftGroupType)},
			"page":      pageInfoFields["page"],
			"page_size": pageInfoFields["page_size"],
			"total":     pageInfoFields["total"],
			"has_next":  pageInfoFields["has_next"],
			"has_prev":  pageInfoFields["has_prev"],
		},
	})

	return &gqlTypes{
		AdminUserType:          adminUserType,
		SubjectType:            subjectType,
		TaskType:               taskType,
		MeType:                 meType,
		AuthSettingsType:       authSettingsType,
		ApiKeyType:             apiKeyType,
		IcalTokenType:          icalTokenType,
		McpInfoType:            mcpInfoType,
		LogItemType:            logItemType,
		DayShiftCycleType:      dayShiftCycleType,
		DayShiftAssignmentType: dayShiftAssignmentType,
		DayShiftGroupType:      dayShiftGroupType,
		DayShiftCycleListType:  dayShiftCycleListType,
		DayShiftGroupListType:  dayShiftGroupListType,
	}
}
