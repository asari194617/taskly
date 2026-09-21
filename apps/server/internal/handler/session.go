package handler

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func validateLoginClient(c *fiber.Ctx, platform string) error {
	origin := strings.TrimSpace(c.Get("Origin"))
	if platform == "web" {
		// Browsers always send Origin for the cross-origin production login.
		// Permit an absent Origin only for local development and tests.
		if origin == "" && strings.ToLower(utils.Env("NODE_ENV", "development")) != "production" {
			return nil
		}
		if !utils.IsAllowedWebOrigin(origin) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "origin not allowed"})
		}
		return nil
	}
	// A browser must not be able to opt into a native platform and receive a
	// readable bearer token merely by changing X-Client-Platform.
	if origin != "" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "browser token login is not allowed"})
	}
	return nil
}

func (h *Handler) issueSession(c *fiber.Ctx, userID uint, platform string, now time.Time) error {
	token := utils.GenerateSessionToken(userID, platform)
	expires := now.Add(utils.SessionTTL)
	if err := h.rdb.Set(c.Context(), "session:"+token, fmt.Sprintf("%d", userID), utils.SessionTTL).Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "session"})
	}
	_ = h.rdb.SAdd(c.Context(), fmt.Sprintf("user_sessions:%d", userID), token).Err()
	_ = h.rdb.Expire(c.Context(), fmt.Sprintf("user_sessions:%d", userID), utils.SessionTTL).Err()
	if err := h.db.Create(&domain.Session{
		UserID:         userID,
		ClientPlatform: platform,
		HashedToken:    utils.Sha256Hex([]byte(token)),
		ExpiresAt:      utils.PtrTime(expires),
	}).Error; err != nil {
		utils.RevokeSession(c, token, userID, h.db, h.rdb)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "session"})
	}

	if platform == "web" {
		utils.SetSessionCookie(c, token, expires)
		return c.JSON(fiber.Map{"authenticated": true})
	}

	claims := jwt.MapClaims{"sub": userID, "exp": expires.Unix()}
	signed := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	jwtString, _ := signed.SignedString([]byte(os.Getenv("JWT_SECRET")))
	return c.JSON(fiber.Map{"token": token, "jwt": jwtString})
}
