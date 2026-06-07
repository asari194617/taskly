package gqlH

import (
	"context"

	gqlshared "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	"github.com/gofiber/fiber/v2"
)

// WithFiberCtx stores Fiber request context into GraphQL execution context.
func WithFiberCtx(ctx context.Context, c *fiber.Ctx) context.Context {
	return gqlshared.WithFiberCtx(ctx, c)
}

// FiberCtxFromContext retrieves Fiber request context from GraphQL execution context.
func FiberCtxFromContext(ctx context.Context) (*fiber.Ctx, bool) {
	return gqlshared.FiberCtxFromContext(ctx)
}
