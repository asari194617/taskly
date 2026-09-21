package domain

import "time"

// NotificationTypePref stores per-user on/off for specific notification kinds
type NotificationTypePref struct {
	ID               uint      `gorm:"primaryKey"`
	UserID           uint      `gorm:"index;not null"`
	NotificationType string    `gorm:"size:16;not null"` // TASK | DAYSHIFT
	IsEnabled        bool      `gorm:"not null;default:true"`
	CreatedAt        time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

// NotificationDevice stores per-user devices/channels (email, webpush, mobile)
type NotificationDevice struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	Name      *string   `gorm:"size:255"`
	Platform  string    `gorm:"size:16;not null"` // EMAIL | WEBPUSH | IOS | ANDROID
	Payload   *string   `gorm:"type:json"`        // optional JSON payload/metadata
	IsEnabled bool      `gorm:"not null;default:true"`
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

// VAPID keys for Web Push (single row)
type WebPushVAPID struct {
	ID         uint      `gorm:"primaryKey"`
	PublicKey  string    `gorm:"size:255;not null"`
	PrivateKey string    `gorm:"size:255;not null"`
	CreatedAt  time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

// TableName keeps the API and the notification delivery batch on the same
// canonical table. GORM otherwise expands the VAPID acronym to
// "web_push_v_api_ds", while the schema and batch use "web_push_vapid".
func (WebPushVAPID) TableName() string {
	return "web_push_vapid"
}

// Web Push subscription for PWA notifications
type WebPushSubscription struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	Endpoint  string    `gorm:"size:1024;not null"`
	P256dh    string    `gorm:"size:1024;not null"`
	Auth      string    `gorm:"size:255;not null"`
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}
