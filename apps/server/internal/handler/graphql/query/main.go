package gqlQuery

import (
	"context"

	gqlShared "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	"github.com/go-webauthn/webauthn/webauthn"
	graphql "github.com/graphql-go/graphql"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// BuildRootQuery creates the GraphQL Query object using provided types.
func (h *Handler) BuildRootQuery(t *gqlShared.GqlTypes) *graphql.Object {
	fields := graphql.Fields{}
	merge := func(src graphql.Fields) {
		for k, v := range src {
			fields[k] = v
		}
	}
	merge(h.QueryFieldsDayShift(t))
	merge(h.QueryFieldsAdminUsers(t))
	merge(h.QueryFieldsSettings(t))
	merge(h.QueryFieldsTasks(t))
	merge(h.QueryFieldsMisc(t))
	return graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: fields})
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

// withContext gives each resolver its own DB session bound to the GraphQL request.
// The original handler is shared by concurrent requests and must stay unchanged.
func (h *Handler) withContext(ctx context.Context) *Handler {
	request := *h
	if h.db != nil {
		request.db = h.db.WithContext(ctx)
	}
	return &request
}
