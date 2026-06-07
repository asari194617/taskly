package gqlshared

import (
	"context"

	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

type fiberCtxKey struct{}

var ctxKeyFiberCtx = fiberCtxKey{}

func WithFiberCtx(ctx context.Context, c *fiber.Ctx) context.Context {
	return context.WithValue(ctx, ctxKeyFiberCtx, c)
}

func FiberCtxFromContext(ctx context.Context) (*fiber.Ctx, bool) {
	c, ok := ctx.Value(ctxKeyFiberCtx).(*fiber.Ctx)
	return c, ok
}

type GqlTypes struct {
	AdminUserType             *graphql.Object
	UserReportType            *graphql.Object
	NotificationSettingsType  *graphql.Object
	NotificationDeviceType    *graphql.Object
	NotificationTypePrefsType *graphql.Object
	SubjectType               *graphql.Object
	TaskType                  *graphql.Object
	MeType                    *graphql.Object
	AuthSettingsType          *graphql.Object
	PasskeyType               *graphql.Object
	ApiKeyType                *graphql.Object
	IcalTokenType             *graphql.Object
	McpInfoType               *graphql.Object
	LogItemType               *graphql.Object
	DayShiftCycleType         *graphql.Object
	DayShiftGroupType         *graphql.Object
	DayShiftCycleListType     *graphql.Object
	DayShiftGroupListType     *graphql.Object
}
