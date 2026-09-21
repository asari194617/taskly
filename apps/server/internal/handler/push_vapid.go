package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/asari194617/taskly/apps/server/internal/domain"
)

// getOrInitVAPID returns base64url public, private keys. Generates and persists if missing.
func (h *Handler) getOrInitVAPID() (string, string, error) {
	var rec domain.WebPushVAPID
	if err := h.db.First(&rec).Error; err == nil && rec.PublicKey != "" && rec.PrivateKey != "" {
		// If it looks valid (public long, private short), return
		if len(rec.PublicKey) >= 60 && len(rec.PrivateKey) <= 60 {
			return rec.PublicKey, rec.PrivateKey, nil
		}
		// Otherwise, regenerate
	}

	// Before WebPushVAPID declared its canonical table name, GORM stored keys
	// in web_push_v_api_ds. Preserve that key pair so existing browser
	// subscriptions remain valid while moving it to web_push_vapid.
	var legacy domain.WebPushVAPID
	legacyExists := h.db.Migrator().HasTable("web_push_v_api_ds")
	if legacyExists {
		err := h.db.Table("web_push_v_api_ds").Order("id DESC").First(&legacy).Error
		if err != nil || legacy.PublicKey == "" || legacy.PrivateKey == "" {
			legacyExists = false
		}
	}
	if legacyExists {
		legacy.ID = 0
		legacy.CreatedAt = time.Time{}
		if err := h.db.Create(&legacy).Error; err != nil {
			return "", "", err
		}
		return legacy.PublicKey, legacy.PrivateKey, nil
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
	_ = h.db.Where("1 = 1").Delete(&domain.WebPushVAPID{})
	if err := h.db.Create(&rec).Error; err != nil {
		return "", "", err
	}
	return pub, priv, nil
}

// sendPushToUser sends a JSON payload to all subscriptions of the user.
func (h *Handler) sendPushToUser(uid uint, payload map[string]any) (int, error) {
	// fetch subscriptions
	var subs []domain.WebPushSubscription
	_ = h.db.Where("user_id = ?", uid).Find(&subs).Error
	if len(subs) == 0 {
		return 0, nil
	}
	pub, priv, err := h.getOrInitVAPID()
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
				_ = h.db.Where("user_id = ? AND endpoint = ?", uid, s.Endpoint).Delete(&domain.WebPushSubscription{}).Error
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
