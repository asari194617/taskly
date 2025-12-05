package main

import (
	graphql "github.com/graphql-go/graphql"
)

// buildRootMutation creates the GraphQL Mutation object using provided types.
func buildRootMutation(t *gqlTypes) *graphql.Object {
	fields := graphql.Fields{}

	// helper to merge maps
	merge := func(src graphql.Fields) {
		for k, v := range src {
			fields[k] = v
		}
	}

	// Assemble mutation fields from topic modules
	merge(mutationFieldsDayShift(t))
	merge(mutationFieldsSettings(t))
	merge(mutationFieldsAdminUsers(t))
	// Business calendar removed
	merge(mutationFieldsKeys(t))
	merge(mutationFieldsTasks(t))
	merge(mutationFieldsAuth(t))
	merge(mutationFieldsSubjects(t))
	// User reports
	merge(mutationFieldsReports(t))

	rootMutation := graphql.NewObject(graphql.ObjectConfig{
		Name:   "Mutation",
		Fields: fields,
	})
	return rootMutation
}
