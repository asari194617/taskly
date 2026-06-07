package gqlH

import (
	"context"

	gqlShared "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	"github.com/gofiber/fiber/v2"
)

// WithFiberCtx stores Fiber request context into GraphQL execution context.
func WithFiberCtx(ctx context.Context, c *fiber.Ctx) context.Context {
	return gqlShared.WithFiberCtx(ctx, c)
}
