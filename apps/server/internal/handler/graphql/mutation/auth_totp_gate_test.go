package gqlMutation

import (
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
)

func TestLegacyTOTPMutationsBlockAfterOwnershipTransfer(t *testing.T) {
	t.Setenv("AUTH_TOTP_EXTERNAL", "1")
	fields := (&Handler{}).mutationFieldsAuth()
	for _, name := range []string{"totpSetup", "totpActivate", "totpDisable"} {
		field := fields[name]
		if field == nil || field.Resolve == nil {
			t.Fatalf("missing %s resolver", name)
		}
		_, err := field.Resolve(graphql.ResolveParams{})
		if err == nil || !strings.Contains(err.Error(), "managed by auth service") {
			t.Fatalf("%s did not enforce external ownership: %v", name, err)
		}
	}
}
