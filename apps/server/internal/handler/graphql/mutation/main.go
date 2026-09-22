package gqlMutation

import (
	"context"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	gqlShared "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/go-webauthn/webauthn/webauthn"
	graphql "github.com/graphql-go/graphql"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Mutation results reflect the state change. Audit records are best effort and
// failures are sent to Sentry when SENTRY_DSN is configured.
func (h *Handler) recordAuditLog(log *domain.Log) {
	if err := h.db.Create(log).Error; err != nil {
		utils.CaptureError(err)
	}
}

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

// withContext gives each resolver its own DB session bound to the GraphQL request.
// The original handler is shared by concurrent requests and must stay unchanged.
func (h *Handler) withContext(ctx context.Context) *Handler {
	request := *h
	if h.db != nil {
		request.db = h.db.WithContext(ctx)
	}
	return &request
}
