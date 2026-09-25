package gqlMutation

import (
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
)

func TestLegacyPasswordMutationBlocksAfterOwnershipTransfer(t *testing.T) {
	t.Setenv("AUTH_PASSWORD_EXTERNAL", "1")
	field := (&Handler{}).mutationFieldsAuth()["setPassword"]
	if field == nil || field.Resolve == nil {
		t.Fatal("missing setPassword resolver")
	}
	_, err := field.Resolve(graphql.ResolveParams{})
	if err == nil || !strings.Contains(err.Error(), "managed by auth service") {
		t.Fatalf("setPassword did not enforce external ownership: %v", err)
	}
}
