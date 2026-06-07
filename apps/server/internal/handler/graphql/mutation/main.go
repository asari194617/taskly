package gqlMutation

import (
	gqlShared "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	"github.com/go-webauthn/webauthn/webauthn"
	graphql "github.com/graphql-go/graphql"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// buildRootMutation creates the GraphQL Mutation object using provided types.
func (h *Handler) BuildRootMutation(t *gqlShared.GqlTypes) *graphql.Object {
	fields := graphql.Fields{}

	// helper to merge maps
	merge := func(src graphql.Fields) {
		for k, v := range src {
			fields[k] = v
		}
	}

	// Assemble mutation fields from topic modules
	merge(h.mutationFieldsDayShift())
	merge(h.mutationFieldsSettings())
	merge(h.mutationFieldsAdminUsers())
	// Business calendar removed
	merge(h.mutationFieldsKeys())
	merge(h.mutationFieldsTasks(t))
	merge(h.mutationFieldsAuth())
	merge(h.mutationFieldsSubjects(t))
	// User reports
	merge(h.mutationFieldsReports())

	rootMutation := graphql.NewObject(graphql.ObjectConfig{
		Name:   "Mutation",
		Fields: fields,
	})
	return rootMutation
}

// Handler に、このパッケージ内の処理が必要とする外部インスタンスをすべて持たせる
type Handler struct {
	db    *gorm.DB
	rdb   *redis.Client
	wAuth *webauthn.WebAuthn
}

// NewHandler で main.go から依存関係を受け取る
func NewHandler(db *gorm.DB, rdb *redis.Client, wAuth *webauthn.WebAuthn) *Handler {
	return &Handler{
		db:    db,
		rdb:   rdb,
		wAuth: wAuth,
	}
}
