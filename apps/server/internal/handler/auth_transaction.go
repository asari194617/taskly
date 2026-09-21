package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

const (
	authTransactionTTL         = 5 * time.Minute
	authTransactionMaxAttempts = 5
)

var (
	errAuthTransactionNotFound = errors.New("auth transaction not found")
	errAuthTransactionBusy     = errors.New("auth transaction busy")
)

type authTransaction struct {
	UserID         uint      `json:"user_id"`
	Email          string    `json:"email"`
	AllowedMethods []string  `json:"allowed_methods"`
	ClientContext  string    `json:"client_context"`
	FailedAttempts int       `json:"failed_attempts"`
	ExpiresAt      time.Time `json:"expires_at"`
}

func randomOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func authTransactionKey(rawID string) string {
	return "auth_transaction:" + utils.Sha256Hex([]byte(rawID))
}

func authTransactionContext(c *fiber.Ctx, platform string) string {
	// Binding to the platform and User-Agent prevents a copied transaction from
	// being replayed by a different client while allowing ordinary IP changes.
	return utils.Sha256Hex([]byte(platform + "\x00" + c.Get("User-Agent")))
}

func containsAuthMethod(methods []string, method string) bool {
	for _, allowed := range methods {
		if allowed == method {
			return true
		}
	}
	return false
}

func (h *Handler) enforceLoginGuards(c *fiber.Ctx, u domain.User, now time.Time) (bool, error) {
	ipKey := "login_attempt_ip:" + c.IP()
	ipCount, _ := h.rdb.Incr(c.Context(), ipKey).Result()
	if ipCount == 1 {
		_ = h.rdb.Expire(c.Context(), ipKey, 10*time.Minute)
	}
	if ipCount > 20 {
		return false, c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "ip rate limit"})
	}
	emailKey := "login_attempt_email:" + u.Email
	emailCount, _ := h.rdb.Incr(c.Context(), emailKey).Result()
	if emailCount == 1 {
		_ = h.rdb.Expire(c.Context(), emailKey, 10*time.Minute)
	}
	if emailCount > 10 {
		return false, c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "email rate limit"})
	}
	var count int64
	if err := h.db.Model(&domain.Block{}).
		Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", u.ID, now).
		Count(&count).Error; err == nil && count > 0 {
		return false, c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "blocked"})
	}
	return true, nil
}

func (h *Handler) createAuthTransaction(ctx context.Context, tx authTransaction) (string, error) {
	rawID, err := randomOpaqueToken()
	if err != nil {
		return "", err
	}
	tx.ExpiresAt = time.Now().Add(authTransactionTTL)
	payload, err := json.Marshal(tx)
	if err != nil {
		return "", err
	}
	if err := h.rdb.Set(ctx, authTransactionKey(rawID), payload, authTransactionTTL).Err(); err != nil {
		return "", err
	}
	return rawID, nil
}

// lockAuthTransaction serializes attempts so a successful transaction cannot
// be consumed twice by concurrent MFA requests.
func (h *Handler) lockAuthTransaction(ctx context.Context, rawID string) (*authTransaction, func(), error) {
	if strings.TrimSpace(rawID) == "" {
		return nil, nil, errAuthTransactionNotFound
	}
	key := authTransactionKey(rawID)
	lockKey := key + ":lock"
	lockValue, err := randomOpaqueToken()
	if err != nil {
		return nil, nil, err
	}
	locked, err := h.rdb.SetNX(ctx, lockKey, lockValue, 10*time.Second).Result()
	if err != nil {
		return nil, nil, err
	}
	if !locked {
		return nil, nil, errAuthTransactionBusy
	}
	release := func() {
		const compareAndDelete = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`
		_ = h.rdb.Eval(context.Background(), compareAndDelete, []string{lockKey}, lockValue).Err()
	}
	payload, err := h.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		release()
		return nil, nil, errAuthTransactionNotFound
	}
	if err != nil {
		release()
		return nil, nil, err
	}
	var tx authTransaction
	if err := json.Unmarshal(payload, &tx); err != nil {
		_ = h.rdb.Del(ctx, key).Err()
		release()
		return nil, nil, errAuthTransactionNotFound
	}
	if !tx.ExpiresAt.After(time.Now()) {
		_ = h.rdb.Del(ctx, key).Err()
		release()
		return nil, nil, errAuthTransactionNotFound
	}
	return &tx, release, nil
}

func (h *Handler) recordAuthTransactionFailure(ctx context.Context, rawID string, tx *authTransaction) (exhausted bool, err error) {
	tx.FailedAttempts++
	key := authTransactionKey(rawID)
	if tx.FailedAttempts >= authTransactionMaxAttempts {
		return true, h.rdb.Del(ctx, key).Err()
	}
	ttl := time.Until(tx.ExpiresAt)
	if ttl <= 0 {
		_ = h.rdb.Del(ctx, key).Err()
		return true, nil
	}
	payload, err := json.Marshal(tx)
	if err != nil {
		return false, err
	}
	return false, h.rdb.Set(ctx, key, payload, ttl).Err()
}

func (h *Handler) consumeAuthTransaction(ctx context.Context, rawID string) error {
	deleted, err := h.rdb.Del(ctx, authTransactionKey(rawID)).Result()
	if err != nil {
		return err
	}
	if deleted != 1 {
		return fmt.Errorf("auth transaction was already consumed")
	}
	return nil
}
