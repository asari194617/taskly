package gqlMutation

import (
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
)

func TestLegacyRecoveryGenerationBlocksAfterOwnershipTransfer(t *testing.T) {
	t.Setenv("AUTH_RECOVERY_EXTERNAL", "1")
	field := (&Handler{}).mutationFieldsAuth()["recoveryGenerate"]
	if field == nil || field.Resolve == nil {
		t.Fatal("missing recoveryGenerate resolver")
	}
	_, err := field.Resolve(graphql.ResolveParams{})
	if err == nil || !strings.Contains(err.Error(), "managed by auth service") {
		t.Fatalf("recoveryGenerate did not enforce external ownership: %v", err)
	}
}
