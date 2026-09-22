package legal

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// Version identifies the immutable, archived text displayed by the web client.
const Version = "2026-09-22.1"

// For a changed document, deploy the new archive and version before the
// effective date. Set PreviousVersion and a future EffectiveAt, then publish
// the notice in the web client. Empty PreviousVersion marks the first tracked
// edition; legacy boolean values are never treated as evidence of consent.
const PreviousVersion = ""
const EffectiveAt = "2026-09-22T00:00:00+09:00"

type Consent struct {
	ID              uint      `gorm:"primaryKey"`
	UserID          uint      `gorm:"not null;index:idx_policy_consents_user_time,priority:1"`
	TermsVersion    string    `gorm:"size:32;not null"`
	PrivacyVersion  string    `gorm:"size:32;not null"`
	SecurityVersion string    `gorm:"size:32;not null"`
	DisplayLanguage string    `gorm:"size:5;not null"`
	AcceptedAt      time.Time `gorm:"not null;index:idx_policy_consents_user_time,priority:2"`
}

func (Consent) TableName() string { return "policy_consents" }

// Current deliberately does not infer a version or timestamp from the legacy
// users.is_agreed_policies flag. Existing users must record an explicit event.
func Current(db *gorm.DB, userID uint) (bool, error) {
	var latest Consent
	err := db.Where("user_id = ?", userID).Order("accepted_at DESC, id DESC").First(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if latest.TermsVersion == Version &&
		latest.PrivacyVersion == Version &&
		latest.SecurityVersion == Version {
		return true, nil
	}
	effective, err := time.Parse(time.RFC3339, EffectiveAt)
	if err != nil {
		return false, err
	}
	return PreviousVersion != "" && time.Now().Before(effective) &&
		latest.TermsVersion == PreviousVersion &&
		latest.PrivacyVersion == PreviousVersion &&
		latest.SecurityVersion == PreviousVersion, nil
}

func Notice(db *gorm.DB, userID uint) (string, error) {
	if PreviousVersion == "" {
		return "", nil
	}
	effective, err := time.Parse(time.RFC3339, EffectiveAt)
	if err != nil || !time.Now().Before(effective) {
		return "", err
	}
	var latest Consent
	err = db.Where("user_id = ?", userID).Order("accepted_at DESC, id DESC").First(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if latest.TermsVersion == PreviousVersion && latest.PrivacyVersion == PreviousVersion &&
		latest.SecurityVersion == PreviousVersion {
		return EffectiveAt, nil
	}
	return "", nil
}

func Accept(db *gorm.DB, userID uint, terms, privacy, security, language string) error {
	if terms != Version || privacy != Version || security != Version {
		return errors.New("policy version changed; reload the consent page")
	}
	if language != "ja" && language != "en" {
		return errors.New("invalid display language")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&Consent{
			UserID: userID, TermsVersion: terms, PrivacyVersion: privacy,
			SecurityVersion: security, DisplayLanguage: language, AcceptedAt: time.Now(),
		}).Error; err != nil {
			return err
		}
		return tx.Table("users").Where("id = ?", userID).
			Update("is_agreed_policies", true).Error
	})
}
