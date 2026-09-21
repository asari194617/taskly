package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const storedWebauthnCredentialPrefix = "webauthn:v1:"

// EncodeWebauthnCredential preserves the complete credential record. Earlier
// versions stored only Credential.PublicKey and consequently discarded flags
// required to validate synced passkeys (including Safari/iCloud passkeys).
func EncodeWebauthnCredential(credential *webauthn.Credential) (string, error) {
	if credential == nil {
		return "", fmt.Errorf("credential is nil")
	}

	raw, err := json.Marshal(credential)
	if err != nil {
		return "", fmt.Errorf("marshal credential: %w", err)
	}

	return storedWebauthnCredentialPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

// NewPasskeyRecord creates a database record without losing WebAuthn metadata.
func NewPasskeyRecord(userID uint, deviceName string, credential *webauthn.Credential) (domain.Passkey, error) {
	encoded, err := EncodeWebauthnCredential(credential)
	if err != nil {
		return domain.Passkey{}, err
	}

	return domain.Passkey{
		UserID:       userID,
		CredentialID: base64.RawStdEncoding.EncodeToString(credential.ID),
		PublicKey:    encoded,
		SignCount:    int64(credential.Authenticator.SignCount),
		DeviceName:   &deviceName,
	}, nil
}

func (w *WebauthnUserWrapper) WebAuthnCredentials() []webauthn.Credential {
	var passkeys []domain.Passkey
	if err := w.db.Where("user_id = ?", w.ID).Find(&passkeys).Error; err != nil {
		return nil
	}

	credentials := make([]webauthn.Credential, 0, len(passkeys))
	for _, passkey := range passkeys {
		credential, err := decodeStoredWebauthnCredential(passkey, w.legacyBackupEligible)
		if err != nil {
			continue
		}
		credentials = append(credentials, credential)
	}

	return credentials
}

func decodeStoredWebauthnCredential(passkey domain.Passkey, legacyBackupEligible *bool) (webauthn.Credential, error) {
	if strings.HasPrefix(passkey.PublicKey, storedWebauthnCredentialPrefix) {
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(passkey.PublicKey, storedWebauthnCredentialPrefix))
		if err != nil {
			return webauthn.Credential{}, fmt.Errorf("decode stored credential: %w", err)
		}

		var credential webauthn.Credential
		if err := json.Unmarshal(raw, &credential); err != nil {
			return webauthn.Credential{}, fmt.Errorf("unmarshal stored credential: %w", err)
		}
		credential.Flags = normalizedCredentialFlags(credential.Flags)
		return credential, nil
	}

	id, err := decodeBase64(passkey.CredentialID)
	if err != nil {
		return webauthn.Credential{}, fmt.Errorf("decode credential ID: %w", err)
	}
	publicKey, err := decodeBase64(passkey.PublicKey)
	if err != nil {
		return webauthn.Credential{}, fmt.Errorf("decode public key: %w", err)
	}
	if passkey.SignCount < 0 || passkey.SignCount > math.MaxUint32 {
		return webauthn.Credential{}, fmt.Errorf("invalid sign count: %d", passkey.SignCount)
	}

	flags := webauthn.CredentialFlags{}
	if legacyBackupEligible != nil {
		flags.BackupEligible = *legacyBackupEligible
	}
	flags = normalizedCredentialFlags(flags)

	return webauthn.Credential{
		ID:        id,
		PublicKey: publicKey,
		Flags:     flags,
		Authenticator: webauthn.Authenticator{
			SignCount: uint32(passkey.SignCount),
		},
	}, nil
}

func normalizedCredentialFlags(flags webauthn.CredentialFlags) webauthn.CredentialFlags {
	var raw protocol.AuthenticatorFlags
	if flags.UserPresent {
		raw |= protocol.FlagUserPresent
	}
	if flags.UserVerified {
		raw |= protocol.FlagUserVerified
	}
	if flags.BackupEligible {
		raw |= protocol.FlagBackupEligible
	}
	if flags.BackupState {
		raw |= protocol.FlagBackupState
	}
	return webauthn.NewCredentialFlags(raw)
}

func decodeBase64(value string) ([]byte, error) {
	encodings := []*base64.Encoding{
		base64.RawStdEncoding,
		base64.RawURLEncoding,
		base64.StdEncoding,
		base64.URLEncoding,
	}
	for _, encoding := range encodings {
		if decoded, err := encoding.DecodeString(value); err == nil {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("unsupported base64 encoding")
}
