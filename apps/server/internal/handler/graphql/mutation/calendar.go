package gqlMutation

import (
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	graphql "github.com/graphql-go/graphql"
)

// Business calendar feature removed; return empty mutation set.
func mutationFieldsCalendar(t *gqlH.GqlTypes) graphql.Fields {
	return graphql.Fields{}
}
