package gqlH

import (
	"context"
	"strings"

	gqlShared "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

// WithFiberCtx stores Fiber request context into GraphQL execution context.
func WithFiberCtx(ctx context.Context, c *fiber.Ctx) context.Context {
	return gqlShared.WithFiberCtx(ctx, c)
}

type resolverRequestContextKey struct{}

// graphql-go's Execute returns on Context.Done before its resolver goroutine
// finishes. Keep the executor alive until that worker has returned, while each
// application resolver receives the original cancellable request context.
func graphQLExecutionContext(request context.Context) context.Context {
	return context.WithValue(context.WithoutCancel(request), resolverRequestContextKey{}, request)
}

func requestBoundResolver(resolve graphql.FieldResolveFn) graphql.FieldResolveFn {
	return func(params graphql.ResolveParams) (any, error) {
		if params.Context == nil {
			params.Context = context.Background()
		}
		if request, ok := params.Context.Value(resolverRequestContextKey{}).(context.Context); ok {
			params.Context = request
		}
		if err := params.Context.Err(); err != nil {
			return nil, err
		}
		return resolve(params)
	}
}

// Called once, before serving requests, on the application's freshly built
// schema. Introspection objects/fields are library globals and must not change.
func bindRequestResolvers(schema *graphql.Schema) {
	for name, gqlType := range schema.TypeMap() {
		if strings.HasPrefix(name, "__") {
			continue
		}
		object, ok := gqlType.(*graphql.Object)
		if !ok {
			continue
		}
		for fieldName, field := range object.Fields() {
			if field.Resolve != nil && !strings.HasPrefix(fieldName, "__") {
				field.Resolve = requestBoundResolver(field.Resolve)
			}
		}
	}
}
