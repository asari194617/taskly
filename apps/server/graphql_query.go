package main

import (
	graphql "github.com/graphql-go/graphql"
)

// buildRootQuery creates the GraphQL Query object using provided types.
// It now delegates field definitions to topic-specific helpers to keep this file concise.
func buildRootQuery(t *gqlTypes) *graphql.Object {
	fields := graphql.Fields{}

	// Helper to merge maps
	merge := func(src graphql.Fields) {
		for k, v := range src {
			fields[k] = v
		}
	}

	// Assemble query fields from topic modules
	merge(queryFieldsDayShift(t))
	merge(queryFieldsAdminUsers(t))
	merge(queryFieldsCalendar(t))
	merge(queryFieldsSettings(t))
	merge(queryFieldsTasks(t))
	merge(queryFieldsMisc(t))

	rootQuery := graphql.NewObject(graphql.ObjectConfig{
		Name:   "Query",
		Fields: fields,
	})
	return rootQuery
}
