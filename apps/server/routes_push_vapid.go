package main

import (
	"encoding/json"
	"log"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/gofiber/fiber/v2"
)

// registerPushVAPIDRoutes exposes VAPID public key and test push endpoints.
func registerPushVAPIDRoutes(app *fiber.App) {
	app.Get("/push/vapidPublicKey", func(c *fiber.Ctx) error {
		pub, _, err := getOrInitVAPID()
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "vapid"})
		}
		return c.JSON(fiber.Map{"publicKey": pub})
	})
	app.Post("/push/test", authRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		sent, err := sendPushToUser(u.ID, map[string]any{
			"title": "Taskly",
			"body":  "プッシュ通知のテスト",
			"data":  map[string]any{"url": "/home"},
		})
		if err != nil {
			log.Printf("push test error: %v", err)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "send"})
		}
		return c.JSON(fiber.Map{"sent": sent})
	})

	// Debug: list current user's subscriptions (masked endpoints)
	app.Get("/push/subscriptions", authRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var subs []domain.WebPushSubscription
		if err := DB.Where("user_id = ?", u.ID).Find(&subs).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		out := make([]fiber.Map, 0, len(subs))
		for _, s := range subs {
			ep := s.Endpoint
			if len(ep) > 16 {
				ep = ep[:8] + "…" + ep[len(ep)-5:]
			}
			out = append(out, fiber.Map{"endpoint": ep, "created_at": s.CreatedAt})
		}
		return c.JSON(fiber.Map{"count": len(subs), "items": out})
	})
}

// getOrInitVAPID returns base64url public, private keys. Generates and persists if missing.
func getOrInitVAPID() (string, string, error) {
	var rec domain.WebPushVAPID
	if err := DB.First(&rec).Error; err == nil && rec.PublicKey != "" && rec.PrivateKey != "" {
		// If it looks valid (public long, private short), return
		if len(rec.PublicKey) >= 60 && len(rec.PrivateKey) <= 60 {
			return rec.PublicKey, rec.PrivateKey, nil
		}
		// Otherwise, regenerate
	}
	// Generate new VAPID pair (assume k1=private, k2=public for this library)
	k1, k2, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", err
	}
	// Assign with expected sizes: public should be long (~86-88), private short (~43-44)
	pub, priv := k2, k1
	// If sizes still look wrong, swap as fallback
	if len(pub) < len(priv) {
		pub, priv = priv, pub
	}
	rec = domain.WebPushVAPID{PublicKey: pub, PrivateKey: priv}
	_ = DB.Where("1 = 1").Delete(&domain.WebPushVAPID{})
	if err := DB.Create(&rec).Error; err != nil {
		return "", "", err
	}
	return pub, priv, nil
}

// sendPushToUser sends a JSON payload to all subscriptions of the user.
func sendPushToUser(uid uint, payload map[string]any) (int, error) {
	// fetch subscriptions
	var subs []domain.WebPushSubscription
	_ = DB.Where("user_id = ?", uid).Find(&subs).Error
	if len(subs) == 0 {
		return 0, nil
	}
	pub, priv, err := getOrInitVAPID()
	if err != nil {
		return 0, err
	}
	// Prepare payload JSON
	body, _ := json.Marshal(payload)
	sent := 0
	for _, s := range subs {
		sub := &webpush.Subscription{
			Endpoint: s.Endpoint,
			Keys: webpush.Keys{
				P256dh: s.P256dh,
				Auth:   s.Auth,
			},
		}
		resp, err := webpush.SendNotification(body, sub, &webpush.Options{
			Subscriber:      "mailto:noreply@taskly.local",
			VAPIDPublicKey:  pub,
			VAPIDPrivateKey: priv,
			TTL:             60,
		})
		if err != nil {
			// Delete gone subscriptions
			if resp != nil && (resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound) {
				_ = DB.Where("user_id = ? AND endpoint = ?", uid, s.Endpoint).Delete(&domain.WebPushSubscription{}).Error
				continue
			}
			log.Printf("webpush send error: %v", err)
			continue
		}
		_ = resp.Body.Close()
		sent++
	}
	return sent, nil
}

// (server no longer runs in-process push digest schedulers; handled by Airflow)
