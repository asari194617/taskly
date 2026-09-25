package gqlMutation

import (
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
)

func TestLegacyEmailSettingsMutationBlocksAfterOwnershipTransfer(t *testing.T) {
	t.Setenv("AUTH_EMAIL_SETTINGS_EXTERNAL", "1")
	field := (&Handler{}).mutationFieldsAuth()["setAuthSettings"]
	if field == nil || field.Resolve == nil {
		t.Fatal("missing setAuthSettings resolver")
	}
	_, err := field.Resolve(graphql.ResolveParams{})
	if err == nil || !strings.Contains(err.Error(), "managed by auth service") {
		t.Fatalf("setAuthSettings did not enforce external ownership: %v", err)
	}
}
