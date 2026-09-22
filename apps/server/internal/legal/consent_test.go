package legal

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestConsentRecordsAreVersionedAndAtomic(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:legal-consent?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, is_agreed_policies BOOLEAN NOT NULL DEFAULT 0)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Consent{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO users (id, is_agreed_policies) VALUES (1, 1)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Consent{UserID: 1, TermsVersion: "old", PrivacyVersion: "old", SecurityVersion: "old", DisplayLanguage: "ja"}).Error; err != nil {
		t.Fatal(err)
	}
	current, err := Current(db, 1)
	if err != nil || current {
		t.Fatalf("legacy or older version must not count as current consent: %v, %v", current, err)
	}
	if err := Accept(db, 1, "old", Version, Version, "ja"); err == nil {
		t.Fatal("accepted a stale document version")
	}
	if err := Accept(db, 1, Version, Version, Version, "fr"); err == nil {
		t.Fatal("accepted an invalid language")
	}
	var count int64
	if err := db.Model(&Consent{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("rejected submission wrote %d records: %v", count, err)
	}
	if err := Accept(db, 1, Version, Version, Version, "en"); err != nil {
		t.Fatal(err)
	}
	current, err = Current(db, 1)
	if err != nil || !current {
		t.Fatalf("accepted version should grant access: %v, %v", current, err)
	}
	var saved Consent
	if err := db.Order("id DESC").First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.UserID != 1 || saved.DisplayLanguage != "en" || saved.AcceptedAt.IsZero() {
		t.Fatalf("incomplete event: %+v", saved)
	}
}
