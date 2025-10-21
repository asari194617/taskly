package main

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

// registerGraphQL builds the schema via modular builders and mounts the /graphql endpoint.
func registerGraphQL(app *fiber.App) {
	// Build types, query and mutation via modular helpers
	types := createGQLTypes()
	rootQuery := buildRootQuery(types)
	rootMutation := buildRootMutation(types)

	// Build schema and mount endpoint
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    rootQuery,
		Mutation: rootMutation,
	})
	if err != nil {
		return
	}

	// GraphQL POST-only endpoint with security features
	app.Post("/graphql", func(c *fiber.Ctx) error {
		// Rate limit: max 30 req/min per IP
		ipKey := "graphql_rate:" + c.IP()
		ipCount, _ := RDB.Incr(c.Context(), ipKey).Result()
		if ipCount == 1 {
			_ = RDB.Expire(c.Context(), ipKey, 60)
		}
		if ipCount > 30 {
			return c.Status(429).JSON(fiber.Map{"error": "rate limit"})
		}

		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = c.BodyParser(&body)

		// Query depth/complexity limit (simple check)
		maxDepth := 6
		maxComplexity := 100
		if depth := estimateGraphQLDepth(body.Query); depth > maxDepth {
			return c.Status(400).JSON(fiber.Map{"error": "query too deep"})
		}
		if complexity := estimateGraphQLComplexity(body.Query); complexity > maxComplexity {
			return c.Status(400).JSON(fiber.Map{"error": "query too complex"})
		}

		// Timeout context (5s)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Disable introspection in production
		nodeEnv := strings.ToLower(env("NODE_ENV", "development"))
		if nodeEnv != "development" && isIntrospectionQuery(body.Query) {
			return c.Status(403).JSON(fiber.Map{"error": "introspection disabled"})
		}

		params := graphql.Params{
			Schema:         schema,
			RequestString:  body.Query,
			VariableValues: body.Variables,
			Context:        context.WithValue(ctx, ctxKeyFiberCtx, c),
		}
		result := graphql.Do(params)
		if len(result.Errors) > 0 {
			return c.Status(400).JSON(result)
		}
		return c.JSON(result)
	})

	// Optionally: block GET requests to /graphql
	app.All("/graphql", func(c *fiber.Ctx) error {
		if c.Method() != fiber.MethodPost {
			return c.Status(405).JSON(fiber.Map{"error": "POST only"})
		}
		return c.Next()
	})
}

// Estimate query depth (very simple, not a full parser)
func estimateGraphQLDepth(query string) int {
	depth := 0
	max := 0
	for _, c := range query {
		if c == '{' {
			depth++
			if depth > max {
				max = depth
			}
		}
		if c == '}' {
			depth--
		}
	}
	return max
}

// Estimate query complexity (count fields)
func estimateGraphQLComplexity(query string) int {
	cnt := 0
	for _, c := range query {
		if c == '{' || c == '}' {
			cnt++
		}
	}
	return cnt
}

// Detect introspection query
func isIntrospectionQuery(query string) bool {
	return strings.Contains(query, "__schema") || strings.Contains(query, "__type")
}
