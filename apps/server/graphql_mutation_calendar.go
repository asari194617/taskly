package main

import (
	graphql "github.com/graphql-go/graphql"
)

// Business calendar feature removed; return empty mutation set.
func mutationFieldsCalendar(t *gqlTypes) graphql.Fields {
	return graphql.Fields{}
}
