package utils

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWebauthnUserWrapperRestoresLegacyCredential(t *testing.T) {
	db := openPasskeyTestDB(t)
	user := &domain.User{ID: 42, Email: "safari@example.test"}
	credentialID := []byte("legacy-credential-id")
	publicKey := []byte{0xa5, 0x01, 0x02, 0x03, 0x26}
	passkey := domain.Passkey{
		UserID:       user.ID,
		CredentialID: base64.RawStdEncoding.EncodeToString(credentialID),
		PublicKey:    base64.RawStdEncoding.EncodeToString(publicKey),
		SignCount:    7,
	}
	if err := db.Create(&passkey).Error; err != nil {
		t.Fatalf("create passkey: %v", err)
	}

	credentials := NewWebauthnUserWrapperForAssertion(user, db, true).WebAuthnCredentials()
	if len(credentials) != 1 {
		t.Fatalf("got %d credentials, want 1", len(credentials))
	}
	got := credentials[0]
	if !bytes.Equal(got.ID, credentialID) {
		t.Fatalf("credential ID = %q, want %q", got.ID, credentialID)
	}
	if !bytes.Equal(got.PublicKey, publicKey) {
		t.Fatalf("public key = %x, want %x", got.PublicKey, publicKey)
	}
	if got.Authenticator.SignCount != 7 {
		t.Fatalf("sign count = %d, want 7", got.Authenticator.SignCount)
	}
	if !got.Flags.BackupEligible {
		t.Fatal("legacy credential should use the assertion backup-eligible hint")
	}
}

func TestWebauthnCredentialStorageRoundTrip(t *testing.T) {
	db := openPasskeyTestDB(t)
	user := &domain.User{ID: 7, Email: "passkey@example.test"}
	want := &webauthn.Credential{
		ID:                []byte("credential-id"),
		PublicKey:         []byte{0xa5, 0x01, 0x02, 0x03, 0x26},
		AttestationType:   "none",
		AttestationFormat: "none",
		Transport:         []protocol.AuthenticatorTransport{protocol.Internal, protocol.Hybrid},
		Flags: webauthn.NewCredentialFlags(
			protocol.FlagUserPresent |
				protocol.FlagUserVerified |
				protocol.FlagBackupEligible |
				protocol.FlagBackupState,
		),
		Authenticator: webauthn.Authenticator{
			AAGUID:    bytes.Repeat([]byte{0x11}, 16),
			SignCount: 9,
		},
	}
	record, err := NewPasskeyRecord(user.ID, "Safari", want)
	if err != nil {
		t.Fatalf("new passkey record: %v", err)
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("create passkey: %v", err)
	}

	// A false assertion hint must not override registration-time metadata in
	// the complete storage format.
	credentials := NewWebauthnUserWrapperForAssertion(user, db, false).WebAuthnCredentials()
	if len(credentials) != 1 {
		t.Fatalf("got %d credentials, want 1", len(credentials))
	}
	got := credentials[0]
	if !bytes.Equal(got.ID, want.ID) || !bytes.Equal(got.PublicKey, want.PublicKey) {
		t.Fatalf("credential bytes did not round trip: %#v", got)
	}
	if got.Authenticator.SignCount != want.Authenticator.SignCount {
		t.Fatalf("sign count = %d, want %d", got.Authenticator.SignCount, want.Authenticator.SignCount)
	}
	if !got.Flags.BackupEligible || !got.Flags.BackupState {
		t.Fatalf("backup flags did not round trip: %#v", got.Flags)
	}
	if got.Flags.ProtocolValue()&protocol.FlagBackupEligible == 0 {
		t.Fatal("raw credential flags were not reconstructed")
	}
}

func openPasskeyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&domain.Passkey{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return db
}
