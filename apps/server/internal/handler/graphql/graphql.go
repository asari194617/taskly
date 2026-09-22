package gqlH

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gqlMutation "github.com/asari194617/taskly/apps/server/internal/handler/graphql/mutation"
	gqlQuery "github.com/asari194617/taskly/apps/server/internal/handler/graphql/query"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/parser"
)

// RegisterGraphQL builds the schema via modular builders and mounts the /graphql endpoint.
func (h *Handler) RegisterGraphQL(router fiber.Router) {
	// Build types, query and mutation via modular helpers
	types := h.createGQLTypes()
	rootQuery := gqlQuery.NewHandler(h.db, h.rdb, h.wAuth).BuildRootQuery(types)
	rootMutation := gqlMutation.NewHandler(h.db, h.rdb, h.wAuth).BuildRootMutation(types)

	// Build schema and mount endpoint
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    rootQuery,
		Mutation: rootMutation,
	})
	if err != nil {
		return
	}

	// GraphQL POST-only endpoint with security features
	router.Post("/", func(c *fiber.Ctx) error {
		if len(c.Body()) > maxGraphQLBodyBytes {
			return c.Status(413).JSON(fiber.Map{"error": "request too large"})
		}
		// Rate limit: max 150 req/min per IP
		ipKey := "graphql_rate:" + c.IP()
		ipCount, _ := h.rdb.Incr(c.Context(), ipKey).Result()
		if ipCount == 1 {
			_ = h.rdb.Expire(c.Context(), ipKey, 60*time.Second)
		}
		if ipCount > 150 {
			return c.Status(429).JSON(fiber.Map{"error": "rate limit"})
		}

		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		if strings.TrimSpace(body.Query) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "missing query"})
		}
		if len(body.Query) > maxGraphQLQueryBytes {
			return c.Status(413).JSON(fiber.Map{"error": "query too large"})
		}
		variablesJSON, err := json.Marshal(body.Variables)
		if err != nil || len(variablesJSON) > maxGraphQLVariablesBytes {
			return c.Status(413).JSON(fiber.Map{"error": "variables too large"})
		}
		document, err := parser.Parse(parser.ParseParams{Source: body.Query})
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid query"})
		}
		cost, err := estimateGraphQLCost(document, schema, body.Variables)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid query"})
		}
		if cost.depth > maxGraphQLDepth {
			return c.Status(400).JSON(fiber.Map{"error": "query too deep"})
		}
		if cost.fields > maxGraphQLFields || cost.complexity > maxGraphQLComplexity {
			return c.Status(400).JSON(fiber.Map{"error": "query too complex"})
		}

		// Retain upstream cancellation while enforcing the GraphQL deadline.
		ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
		defer cancel()

		// Disable introspection in production
		nodeEnv := strings.ToLower(utils.Env("NODE_ENV", "development"))
		if nodeEnv != "development" && isIntrospectionQuery(body.Query) {
			return c.Status(403).JSON(fiber.Map{"error": "introspection disabled"})
		}

		params := graphql.Params{
			Schema:         schema,
			RequestString:  body.Query,
			VariableValues: body.Variables,
			Context:        WithFiberCtx(ctx, c),
		}
		result := graphql.Do(params)
		if ctx.Err() == context.DeadlineExceeded {
			utils.CaptureError(fmt.Errorf("graphql request timeout: %w", ctx.Err()))
			return c.Status(fiber.StatusGatewayTimeout).JSON(fiber.Map{"error": "query timeout"})
		}
		if ctx.Err() == context.Canceled {
			return ctx.Err()
		}
		if len(result.Errors) > 0 {
			// Preserve a transport-level authentication signal for clients. Resolver
			// errors are otherwise returned as 400 by this endpoint.
			for _, executionError := range result.Errors {
				if executionError.Message == "unauthorized" {
					return c.Status(fiber.StatusUnauthorized).JSON(result)
				}
			}
			return c.Status(400).JSON(result)
		}
		return c.JSON(result)
	})

	// Optionally: block GET requests to /graphql
	router.All("/", func(c *fiber.Ctx) error {
		if c.Method() != fiber.MethodPost {
			return c.Status(405).JSON(fiber.Map{"error": "POST only"})
		}
		return c.Next()
	})
}

// Detect introspection query
func isIntrospectionQuery(query string) bool {
	return strings.Contains(query, "__schema") || strings.Contains(query, "__type")
}
