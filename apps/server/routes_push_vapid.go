package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
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
		u := c.Locals("user").(*User)
		if err := sendPushToUser(u.ID, map[string]any{"title": "Taskly", "body": "プッシュ通知のテスト"}); err != nil {
			log.Printf("push test error: %v", err)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "send"})
		}
		return c.SendStatus(http.StatusNoContent)
	})
}

// getOrInitVAPID returns base64url public, private keys. Generates and persists if missing.
func getOrInitVAPID() (string, string, error) {
	var rec WebPushVAPID
	if err := DB.First(&rec).Error; err == nil && rec.PublicKey != "" && rec.PrivateKey != "" {
		return rec.PublicKey, rec.PrivateKey, nil
	}
	// Generate new VAPID pair
	pk, sk, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", err
	}
	rec = WebPushVAPID{PublicKey: pk, PrivateKey: sk}
	_ = DB.Where("1 = 1").Delete(&WebPushVAPID{})
	if err := DB.Create(&rec).Error; err != nil {
		return "", "", err
	}
	return pk, sk, nil
}

// sendPushToUser sends a JSON payload to all subscriptions of the user.
func sendPushToUser(uid uint, payload map[string]any) error {
	// fetch subscriptions
	var subs []WebPushSubscription
	_ = DB.Where("user_id = ?", uid).Find(&subs).Error
	if len(subs) == 0 {
		return nil
	}
	pub, priv, err := getOrInitVAPID()
	if err != nil {
		return err
	}
	// Prepare payload JSON
	body, _ := json.Marshal(payload)
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
				_ = DB.Where("user_id = ? AND endpoint = ?", uid, s.Endpoint).Delete(&WebPushSubscription{}).Error
				continue
			}
			log.Printf("webpush send error: %v", err)
			continue
		}
		_ = resp.Body.Close()
	}
	return nil
}

// pushNotifyScheduler: send a compact digest similar to email
func pushNotifyScheduler(hhmm string) {
	parts := strings.SplitN(hhmm, ":", 2)
	hh, _ := strconv.Atoi(parts[0])
	mm, _ := strconv.Atoi(parts[1])
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), hh, mm, 0, 0, now.Location())
		if !now.Before(next) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(time.Until(next))
		go func() {
			// for each user with subs and daily push enabled, send count of tasks due today
			var users []User
			_ = DB.Where("is_deleted = 0 AND push_daily_enabled = 1").Find(&users).Error
			today := dateOnly(time.Now())
			tomorrow := dateOnly(time.Now().Add(24 * time.Hour)).Add(-time.Nanosecond)
			for _, u := range users {
				var cnt int64
				_ = DB.Model(&Task{}).Where("added_user_id = ? AND deadline BETWEEN ? AND ?", u.ID, today, tomorrow).Count(&cnt).Error
				if cnt <= 0 {
					continue
				}
				_ = sendPushToUser(u.ID, map[string]any{"title": "今日の課題", "body": fmt.Sprintf("本日締切の課題が%d件あります", cnt), "icon": "/icons/icon-192.png", "badge": "/icons/badge-72.png"})
			}
		}()
	}
}
