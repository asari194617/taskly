package gqlMutation

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/go-webauthn/webauthn/webauthn"
	graphql "github.com/graphql-go/graphql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// mutationFieldsAuth groups user/profile/authentication related mutations.
func (h *Handler) mutationFieldsAuth() graphql.Fields {
	fields := graphql.Fields{}

	fields["agreePolicies"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"terms_version":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"privacy_version":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"security_version": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"display_language": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				if err := legal.Accept(h.db, u.ID,
					p.Args["terms_version"].(string), p.Args["privacy_version"].(string),
					p.Args["security_version"].(string), p.Args["display_language"].(string)); err != nil {
					return false, err
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["updateMe"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"display_name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			displayName, _ := p.Args["display_name"].(string)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				dn := strings.TrimSpace(displayName)
				if len(dn) > 255 {
					dn = dn[:255]
				}
				if err := h.db.Model(&domain.User{}).Where("id = ?", u.ID).Update("display_name", dn).Error; err != nil {
					return false, err
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["setAuthSettings"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"email_login_enabled": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if utils.Env("AUTH_EMAIL_SETTINGS_EXTERNAL", "") == "1" {
				return false, fmt.Errorf("email code settings managed by auth service")
			}
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				val, _ := p.Args["email_login_enabled"].(bool)
				var fresh domain.User
				if err := h.db.First(&fresh, u.ID).Error; err != nil {
					return false, err
				}
				hasPassword := fresh.HashedPassword != nil && *fresh.HashedPassword != ""
				totpEnabled, _, err := utils.GetActiveTOTP(fresh.ID, h.db)
				if err != nil {
					return false, err
				}
				var pkCount int64
				if err := h.db.Model(&domain.Passkey{}).Where("user_id = ?", fresh.ID).Count(&pkCount).Error; err != nil {
					return false, err
				}
				if !val && !(hasPassword || totpEnabled || pkCount > 0) {
					return false, fmt.Errorf("cannot disable without another auth factor")
				}
				if err := h.db.Model(&domain.User{}).Where("id = ?", fresh.ID).Update("email_code_allowed", val).Error; err != nil {
					return false, err
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["logout"] = &graphql.Field{
		Type: graphql.Boolean,
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					utils.ClearSessionCookie(c)
					return false, fmt.Errorf("unauthorized")
				}
				if tok, ok := c.Locals("token").(string); ok && tok != "" {
					if err := utils.RevokeSession(c, tok, c.Locals("user").(*domain.User).ID, h.db, h.rdb); err != nil {
						utils.CaptureError(err)
						return false, fmt.Errorf("session revocation failed")
					}
				}
				utils.ClearSessionCookie(c)
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["deleteMe"] = &graphql.Field{
		Type: graphql.Boolean,
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				if err := utils.SoftDeleteUser(p.Context, u.ID, h.db, h.rdb); err != nil {
					return false, err
				}
				utils.ClearSessionCookie(c)
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["totpSetup"] = &graphql.Field{
		Type: graphql.NewObject(graphql.ObjectConfig{
			Name: "TOTPSetup",
			Fields: graphql.Fields{
				"secret":  &graphql.Field{Type: graphql.String},
				"otpauth": &graphql.Field{Type: graphql.String},
			},
		}),
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				secret, err := utils.GenerateBase32Secret()
				if err != nil {
					return nil, fmt.Errorf("server")
				}
				key := fmt.Sprintf("totp-setup:%d", u.ID)
				if err := h.rdb.Set(p.Context, key, secret, 10*time.Minute).Err(); err != nil {
					return nil, fmt.Errorf("server")
				}
				issuer := "Taskly"
				label := fmt.Sprintf("%s:%s", issuer, u.Email)
				otpauth := fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s", utils.UrlEncode(label), secret, utils.UrlEncode(issuer))
				return map[string]any{"secret": secret, "otpauth": otpauth}, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["totpActivate"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"code": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				code, _ := p.Args["code"].(string)
				key := fmt.Sprintf("totp-setup:%d", u.ID)
				secret, _ := h.rdb.Get(p.Context, key).Result()
				if secret == "" {
					return false, fmt.Errorf("setup expired")
				}
				if !utils.ValidateTOTP(secret, code) {
					return false, fmt.Errorf("invalid code")
				}
				enc, err := utils.EncryptString(secret)
				if err != nil {
					return false, fmt.Errorf("server")
				}
				var ts domain.TOTPSecret
				if err := h.db.Where("user_id = ?", u.ID).First(&ts).Error; err == nil {
					ts.Secret = enc
					ts.IsActive = true
					if err := h.db.Save(&ts).Error; err != nil {
						return false, fmt.Errorf("db: %w", err)
					}
				} else if errors.Is(err, gorm.ErrRecordNotFound) {
					if err := h.db.Create(&domain.TOTPSecret{UserID: u.ID, Secret: enc, IsActive: true}).Error; err != nil {
						return false, fmt.Errorf("db: %w", err)
					}
				} else {
					return false, fmt.Errorf("db: %w", err)
				}
				// The temporary setup key can expire naturally if cleanup fails.
				// Report cleanup failures to Sentry when configured.
				if err := h.rdb.Del(p.Context, key).Err(); err != nil {
					utils.CaptureError(err)
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["totpDisable"] = &graphql.Field{
		Type: graphql.Boolean,
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				var ts domain.TOTPSecret
				if err := h.db.Where("user_id = ?", u.ID).First(&ts).Error; errors.Is(err, gorm.ErrRecordNotFound) {
					return false, fmt.Errorf("not enabled")
				} else if err != nil {
					return false, fmt.Errorf("db: %w", err)
				}
				ts.IsActive = false
				if err := h.db.Save(&ts).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["recoveryGenerate"] = &graphql.Field{
		Type: graphql.NewList(graphql.String),
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				codes := make([]string, 0, 10)
				if err := h.db.Transaction(func(tx *gorm.DB) error {
					if err := tx.Where("user_id = ? AND is_used = 0", u.ID).Delete(&domain.RecoveryCode{}).Error; err != nil {
						return err
					}
					for i := 0; i < 10; i++ {
						plain, err := utils.GenerateRecoveryCode()
						if err != nil {
							return err
						}
						hashed := utils.Sha256Hex([]byte(plain))
						if err := tx.Create(&domain.RecoveryCode{UserID: u.ID, HashedCode: hashed}).Error; err != nil {
							return err
						}
						codes = append(codes, plain)
					}
					return nil
				}); err != nil {
					return nil, fmt.Errorf("db: %w", err)
				}
				return codes, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["webauthnRegisterBegin"] = &graphql.Field{
		Type: graphql.String,
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if utils.Env("AUTH_PASSKEYS_EXTERNAL", "") == "1" {
				return "", fmt.Errorf("passkeys managed by auth service")
			}
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return "", fmt.Errorf("unauthorized")
				}
				if h.wAuth == nil {
					return "", fmt.Errorf("webauthn not configured")
				}
				u := c.Locals("user").(*domain.User)
				// wrap the domain.User so we can provide h.db access for credential loading
				wUser := utils.NewWebauthnUserWrapper(u, h.db)
				creation, sd, err := h.wAuth.BeginRegistration(wUser)
				if err != nil {
					return "", fmt.Errorf("begin failed")
				}
				bsd, _ := json.Marshal(sd)
				_ = h.rdb.Set(p.Context, fmt.Sprintf("webauthn:reg:%d", u.ID), string(bsd), 10*time.Minute).Err()
				b, _ := json.Marshal(creation)
				return string(b), nil
			}
			return "", fmt.Errorf("context missing")
		},
	}

	fields["webauthnRegisterFinish"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if utils.Env("AUTH_PASSKEYS_EXTERNAL", "") == "1" {
				return false, fmt.Errorf("passkeys managed by auth service")
			}
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				if h.wAuth == nil {
					return false, fmt.Errorf("webauthn not configured")
				}
				u := c.Locals("user").(*domain.User)
				raw, _ := h.rdb.Get(p.Context, fmt.Sprintf("webauthn:reg:%d", u.ID)).Result()
				if raw == "" {
					return false, fmt.Errorf("session expired")
				}
				var sd webauthn.SessionData
				_ = json.Unmarshal([]byte(raw), &sd)
				payload, _ := p.Args["input"].(string)
				req, _ := http.NewRequestWithContext(p.Context, "POST", "", strings.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
				// wrap so FinishRegistration can read credentials via h.db if needed
				wUser := utils.NewWebauthnUserWrapper(u, h.db)
				cred, err := h.wAuth.FinishRegistration(wUser, sd, req)

				if err != nil {
					return false, fmt.Errorf("finish failed")
				}
				devName := "Passkey"
				rec, err := utils.NewPasskeyRecord(u.ID, devName, cred)
				if err != nil {
					return false, fmt.Errorf("credential storage: %w", err)
				}
				if err := h.db.Create(&rec).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				_ = h.rdb.Del(p.Context, fmt.Sprintf("webauthn:reg:%d", u.ID)).Err()
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	// Passkey management
	fields["renamePasskey"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if utils.Env("AUTH_PASSKEYS_EXTERNAL", "") == "1" {
				return false, fmt.Errorf("passkeys managed by auth service")
			}
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				id, _ := p.Args["id"].(int)
				name, _ := p.Args["name"].(string)
				name = strings.TrimSpace(name)
				if len(name) > 255 {
					name = name[:255]
				}
				var pk domain.Passkey
				if err := h.db.Where("id = ? AND user_id = ?", id, u.ID).First(&pk).Error; err != nil {
					return false, fmt.Errorf("not found")
				}
				pk.DeviceName = &name
				if err := h.db.Save(&pk).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["deletePasskey"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if utils.Env("AUTH_PASSKEYS_EXTERNAL", "") == "1" {
				return false, fmt.Errorf("passkeys managed by auth service")
			}
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				id, _ := p.Args["id"].(int)
				if err := h.db.Where("id = ? AND user_id = ?", id, u.ID).Delete(&domain.Passkey{}).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["setPassword"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"current": &graphql.ArgumentConfig{Type: graphql.String},
			"next":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if utils.Env("AUTH_PASSWORD_EXTERNAL", "") == "1" {
				return false, fmt.Errorf("password managed by auth service")
			}
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				curr, _ := p.Args["current"].(string)
				next, _ := p.Args["next"].(string)
				if len(strings.TrimSpace(next)) < 8 {
					return false, fmt.Errorf("password too short")
				}
				var fresh domain.User
				if err := h.db.First(&fresh, u.ID).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				if fresh.HashedPassword != nil && *fresh.HashedPassword != "" {
					if err := bcrypt.CompareHashAndPassword([]byte(*fresh.HashedPassword), []byte(curr)); err != nil {
						return false, fmt.Errorf("current password invalid")
					}
				}
				hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
				if err != nil {
					return false, fmt.Errorf("hash")
				}
				hs := string(hash)
				curTok, _ := c.Locals("token").(string)
				if err := h.db.Transaction(func(tx *gorm.DB) error {
					if err := tx.Model(&domain.User{}).Where("id = ?", u.ID).Update("hashed_password", &hs).Error; err != nil {
						return err
					}
					return utils.RevokeOtherSessions(c, u.ID, curTok, tx, h.rdb)
				}); err != nil {
					utils.CaptureError(err)
					return false, fmt.Errorf("db")
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	return fields
}
