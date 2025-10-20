package main

import (
	"context"

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

	app.Post("/graphql", func(c *fiber.Ctx) error {
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = c.BodyParser(&body)
		params := graphql.Params{
			Schema:         schema,
			RequestString:  body.Query,
			VariableValues: body.Variables,
			Context:        context.WithValue(context.Background(), ctxKeyFiberCtx, c),
		}
		result := graphql.Do(params)
		if len(result.Errors) > 0 {
			return c.Status(400).JSON(result)
		}
		return c.JSON(result)
	})
}
