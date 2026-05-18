package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	utils "github.com/asari194617/taskly/apps/server/utils"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// webauthnUserWrapper wraps *utils.User and provides DB access for loading credentials
type webauthnUserWrapper struct {
	*utils.User
	db *gorm.DB
}

// Only override credentials loading; other methods are promoted from *utils.User
func (w *webauthnUserWrapper) WebAuthnCredentials() []webauthn.Credential {
	if w.db == nil || w.User == nil {
		return []webauthn.Credential{}
	}
	var pks []utils.Passkey
	_ = w.db.Where("user_id = ?", w.User.ID).Find(&pks).Error
	res := make([]webauthn.Credential, 0, len(pks))
	for _, p := range pks {
		cid, _ := base64.RawStdEncoding.DecodeString(p.CredentialID)
		pub, _ := base64.RawStdEncoding.DecodeString(p.PublicKey)
		res = append(res, webauthn.Credential{ID: cid, PublicKey: pub, Authenticator: webauthn.Authenticator{SignCount: uint32(p.SignCount)}})
	}
	return res
}

// mutationFieldsAuth groups user/profile/authentication related mutations.
func mutationFieldsAuth(t *gqlTypes) graphql.Fields {
	fields := graphql.Fields{}

	fields["agreePolicies"] = &graphql.Field{
		Type: graphql.Boolean,
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				if err := DB.Model(&utils.User{}).Where("id = ?", u.ID).Update("is_agreed_policies", true).Error; err != nil {
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
			displayName, _ := p.Args["display_name"].(string)
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				dn := strings.TrimSpace(displayName)
				if len(dn) > 255 {
					dn = dn[:255]
				}
				if err := DB.Model(&utils.User{}).Where("id = ?", u.ID).Update("display_name", dn).Error; err != nil {
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
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				val, _ := p.Args["email_login_enabled"].(bool)
				var fresh utils.User
				if err := DB.First(&fresh, u.ID).Error; err != nil {
					return false, err
				}
				hasPassword := fresh.HashedPassword != nil && *fresh.HashedPassword != ""
				totpEnabled, _, _ := utils.GetActiveTOTP(fresh.ID, DB)
				var pkCount int64
				_ = DB.Model(&utils.Passkey{}).Where("user_id = ?", fresh.ID).Count(&pkCount).Error
				if !val && !(hasPassword || totpEnabled || pkCount > 0) {
					return false, fmt.Errorf("cannot disable without another auth factor")
				}
				if err := DB.Model(&utils.User{}).Where("id = ?", fresh.ID).Update("email_code_allowed", val).Error; err != nil {
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
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				if tok, ok := c.Locals("token").(string); ok && tok != "" {
					_ = RDB.Del(c.Context(), "session:"+tok).Err()
					_ = RDB.SRem(c.Context(), fmt.Sprintf("user_sessions:%d", c.Locals("user").(*utils.User).ID), tok).Err()
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["deleteMe"] = &graphql.Field{
		Type: graphql.Boolean,
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				if err := DB.Model(&utils.User{}).Where("id = ?", u.ID).Update("is_deleted", true).Error; err != nil {
					return false, err
				}
				if tok, ok := c.Locals("token").(string); ok && tok != "" {
					_ = RDB.Del(c.Context(), "session:"+tok).Err()
					_ = RDB.SRem(c.Context(), fmt.Sprintf("user_sessions:%d", u.ID), tok).Err()
				}
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
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				secret := utils.GenerateBase32Secret()
				key := fmt.Sprintf("totp-setup:%d", u.ID)
				if err := RDB.Set(c.Context(), key, secret, 10*time.Minute).Err(); err != nil {
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
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				code, _ := p.Args["code"].(string)
				key := fmt.Sprintf("totp-setup:%d", u.ID)
				secret, _ := RDB.Get(c.Context(), key).Result()
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
				var ts utils.TOTPSecret
				if err := DB.Where("user_id = ?", u.ID).First(&ts).Error; err == nil {
					ts.Secret = enc
					ts.IsActive = true
					_ = DB.Save(&ts).Error
				} else {
					_ = DB.Create(&utils.TOTPSecret{UserID: u.ID, Secret: enc, IsActive: true}).Error
				}
				_ = RDB.Del(c.Context(), key).Err()
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["totpDisable"] = &graphql.Field{
		Type: graphql.Boolean,
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				var ts utils.TOTPSecret
				if err := DB.Where("user_id = ?", u.ID).First(&ts).Error; err != nil {
					return false, fmt.Errorf("not enabled")
				}
				ts.IsActive = false
				if err := DB.Save(&ts).Error; err != nil {
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
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				_ = DB.Where("user_id = ? AND is_used = 0", u.ID).Delete(&utils.RecoveryCode{}).Error
				codes := make([]string, 0, 10)
				for i := 0; i < 10; i++ {
					plain := utils.GenerateRecoveryCode()
					hashed := utils.Sha256Hex([]byte(plain))
					_ = DB.Create(&utils.RecoveryCode{UserID: u.ID, HashedCode: hashed}).Error
					codes = append(codes, plain)
				}
				return codes, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["webauthnRegisterBegin"] = &graphql.Field{
		Type: graphql.String,
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return "", fmt.Errorf("unauthorized")
				}
				if WAuth == nil {
					return "", fmt.Errorf("webauthn not configured")
				}
				u := c.Locals("user").(*utils.User)
				// wrap the utils.User so we can provide DB access for credential loading
				wuser := &webauthnUserWrapper{User: u, db: DB}
				creation, sd, err := WAuth.BeginRegistration(wuser)
				if err != nil {
					return "", fmt.Errorf("begin failed")
				}
				bsd, _ := json.Marshal(sd)
				_ = RDB.Set(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID), string(bsd), 10*time.Minute).Err()
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
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				if WAuth == nil {
					return false, fmt.Errorf("webauthn not configured")
				}
				u := c.Locals("user").(*utils.User)
				raw, _ := RDB.Get(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID)).Result()
				if raw == "" {
					return false, fmt.Errorf("session expired")
				}
				var sd webauthn.SessionData
				_ = json.Unmarshal([]byte(raw), &sd)
				payload, _ := p.Args["input"].(string)
				req, _ := http.NewRequestWithContext(c.Context(), "POST", "", strings.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
				// wrap so FinishRegistration can read credentials via DB if needed
				wuser := &webauthnUserWrapper{User: u, db: DB}
				cred, err := WAuth.FinishRegistration(wuser, sd, req)

				if err != nil {
					return false, fmt.Errorf("finish failed")
				}
				pub := base64.RawStdEncoding.EncodeToString(cred.PublicKey)
				cid := base64.RawStdEncoding.EncodeToString(cred.ID)
				devName := "Passkey"
				rec := utils.Passkey{UserID: u.ID, CredentialID: cid, PublicKey: pub, SignCount: int64(cred.Authenticator.SignCount), DeviceName: &devName}
				if err := DB.Create(&rec).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				_ = RDB.Del(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID)).Err()
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
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				id, _ := p.Args["id"].(int)
				name, _ := p.Args["name"].(string)
				name = strings.TrimSpace(name)
				if len(name) > 255 {
					name = name[:255]
				}
				var pk utils.Passkey
				if err := DB.Where("id = ? AND user_id = ?", id, u.ID).First(&pk).Error; err != nil {
					return false, fmt.Errorf("not found")
				}
				pk.DeviceName = &name
				if err := DB.Save(&pk).Error; err != nil {
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
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				id, _ := p.Args["id"].(int)
				if err := DB.Where("id = ? AND user_id = ?", id, u.ID).Delete(&utils.Passkey{}).Error; err != nil {
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
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				curr, _ := p.Args["current"].(string)
				next, _ := p.Args["next"].(string)
				if len(strings.TrimSpace(next)) < 8 {
					return false, fmt.Errorf("password too short")
				}
				var fresh utils.User
				if err := DB.First(&fresh, u.ID).Error; err != nil {
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
				if err := DB.Model(&utils.User{}).Where("id = ?", u.ID).Update("hashed_password", &hs).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				curTok, _ := c.Locals("token").(string)
				utils.RevokeOtherSessions(c, u.ID, curTok, DB, RDB)
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	return fields
}
