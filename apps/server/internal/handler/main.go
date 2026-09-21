package handler

import (
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Handler に、このパッケージ内の処理が必要とする外部インスタンスをすべて持たせる
type Handler struct {
	db    *gorm.DB
	rdb   *redis.Client
	wAuth *webauthn.WebAuthn
	teams *teamsProcessor
}

// NewHandler で main.go から依存関係を受け取る
func NewHandler(db *gorm.DB, rdb *redis.Client, wAuth *webauthn.WebAuthn) *Handler {
	h := &Handler{
		db:    db,
		rdb:   rdb,
		wAuth: wAuth,
	}
	h.teams = newTeamsProcessor(db)
	return h
}

func (h *Handler) Start() {
	h.teams.Start()
}

func (h *Handler) Close() {
	h.teams.Stop()
}
