package handler

import (
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetOrInitVAPIDMigratesLegacyGORMTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:vapid-migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&domain.WebPushVAPID{}); err != nil {
		t.Fatalf("migrate canonical table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE web_push_v_api_ds (
		id integer PRIMARY KEY AUTOINCREMENT,
		public_key text NOT NULL,
		private_key text NOT NULL,
		created_at datetime
	)`).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	const publicKey = "legacy-public-key"
	const privateKey = "legacy-private-key"
	if err := db.Exec(
		"INSERT INTO web_push_v_api_ds (public_key, private_key) VALUES (?, ?)",
		publicKey,
		privateKey,
	).Error; err != nil {
		t.Fatalf("seed legacy key: %v", err)
	}

	h := NewHandler(db, nil, nil)
	gotPublic, gotPrivate, err := h.getOrInitVAPID()
	if err != nil {
		t.Fatalf("getOrInitVAPID: %v", err)
	}
	if gotPublic != publicKey || gotPrivate != privateKey {
		t.Fatalf("migrated keys = (%q, %q), want (%q, %q)", gotPublic, gotPrivate, publicKey, privateKey)
	}

	var canonical domain.WebPushVAPID
	if err := db.First(&canonical).Error; err != nil {
		t.Fatalf("read canonical key: %v", err)
	}
	if canonical.PublicKey != publicKey || canonical.PrivateKey != privateKey {
		t.Fatalf("canonical keys = (%q, %q), want (%q, %q)", canonical.PublicKey, canonical.PrivateKey, publicKey, privateKey)
	}
}
