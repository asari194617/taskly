package handler

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	grpcdyn "github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/gorm"
)

func (h *Handler) RegisterAuthRoutes(router fiber.Router) {
	// request login code via email
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Fatal(err)
	}

	router.Post("/request-code", func(c *fiber.Ctx) error {
		now := time.Now().In(jst)
		var body struct {
			Email string `json:"email"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		email := strings.ToLower(strings.TrimSpace(body.Email))
		if !utils.IsEmail(email) {
			return utils.ValidationError(c, map[string]string{"email": "invalid email"})
		}
		ip := c.IP()
		// IP rate limit (max 10 req/10min)
		ipKey := "login_code_req_ip:" + ip
		ipCount, _ := h.rdb.Incr(c.Context(), ipKey).Result()
		if ipCount == 1 {
			_ = h.rdb.Expire(c.Context(), ipKey, 10*time.Minute)
		}
		if ipCount > 10 {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "ip rate limit"})
		}
		// Email rate limit (max 5 req/10min)
		emailKey := "login_code_req_email:" + email
		emailCount, _ := h.rdb.Incr(c.Context(), emailKey).Result()
		if emailCount == 1 {
			_ = h.rdb.Expire(c.Context(), emailKey, 10*time.Minute)
		}
		if emailCount > 5 {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "email rate limit"})
		}
		// cooldown: deny if a code was created within last 60s and not expired
		var recent domain.LoginCode
		if err := h.db.Where("email = ?", email).Order("id DESC").First(&recent).Error; err == nil {
			if time.Since(recent.CreatedAt) < 60*time.Second && recent.ConsumedAt == nil {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many requests"})
			}
			// ロック中なら拒否
			if recent.LockedUntil != nil && recent.LockedUntil.After(now) {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "locked due to failed attempts"})
			}
		}
		// generate code
		code := fmt.Sprintf("%06d", now.UnixNano()%1000000)
		// In development, log the code for testing when email delivery is unreliable
		if strings.ToLower(utils.Env("NODE_ENV", "development")) == "development" {
			log.Printf("[DEV] Login code for %s: %s", email, code)
		}
		// Compose email via email-compose service (gRPC)
		var subject, html, text string
		if s, h, t, err := composeLoginCodeGRPC(c.Context(), utils.Env("EMAIL_COMPOSE_GRPC_ADDR", "email-compose:50051"), "ja", email, code, "Taskly"); err == nil {
			subject, html, text = s, h, t
		}
		// Fallback subject/body if composer failed
		if subject == "" {
			subject = "Taskly Login Code"
			text = fmt.Sprintf("ログインコード: %s\n5分以内に入力してください。\n", code)
			html = text
		}
		smtpHost := utils.Env("SMTP_HOST", "localhost")
		smtpPort := utils.Env("SMTP_PORT", "25")
		from := utils.Env("SMTP_FROM", "noreply@taskly.local")
		smtpUser := utils.Env("SMTP_USER", from)
		smtpPass := utils.Env("SMTP_PASSWORD", "")
		// Build RFC5322 message with required headers (From/To/Subject/Date) - HTML email
		nowRFC := now.Format(time.RFC1123Z)
		boundary := fmt.Sprintf("taskly-%d", now.UnixNano())
		msg := []byte(
			"From: " + from + "\r\n" +
				"To: " + body.Email + "\r\n" +
				"Subject: " + formatLoginCodeSubject(subject, code) + "\r\n" +
				"Date: " + nowRFC + "\r\n" +
				"MIME-Version: 1.0\r\n" +
				"Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n" +
				"--" + boundary + "\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + text + "\r\n" +
				"--" + boundary + "\r\nContent-Type: text/html; charset=utf-8\r\n\r\n" + html + "\r\n" +
				"--" + boundary + "--\r\n",
		)
		var auth smtp.Auth
		if smtpPass != "" {
			auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
		}
		// Use manual client with timeouts to prevent hanging
		toSec, _ := strconv.Atoi(utils.Env("SMTP_TIMEOUT", "60"))
		if toSec <= 0 {
			toSec = 60
		}
		dialer := &net.Dialer{Timeout: time.Duration(toSec) * time.Second}
		mode := strings.ToLower(strings.TrimSpace(utils.Env("SMTP_TLS", ""))) // "starttls" | "smtps" | ""
		var conn net.Conn
		var err error
		if mode == "smtps" || smtpPort == "465" {
			// Implicit TLS
			conn, err = tls.DialWithDialer(dialer, "tcp", smtpHost+":"+smtpPort, &tls.Config{ServerName: smtpHost})
		} else {
			conn, err = dialer.DialContext(c.Context(), "tcp", smtpHost+":"+smtpPort)
		}
		if err != nil {
			log.Printf("smtp dial error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail connect failed"})
		}
		defer conn.Close()
		_ = conn.SetDeadline(now.Add(time.Duration(toSec) * time.Second))
		cl, err := smtp.NewClient(conn, smtpHost)
		if err != nil {
			log.Printf("smtp client error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail client failed"})
		}
		defer func() { _ = cl.Quit() }()
		// STARTTLS if requested
		if mode == "starttls" {
			if ok, _ := cl.Extension("STARTTLS"); ok {
				if err := cl.StartTLS(&tls.Config{ServerName: smtpHost}); err != nil {
					log.Printf("smtp starttls error: %v", err)
					return c.Status(502).JSON(fiber.Map{"error": "mail tls failed"})
				}
			}
		}
		if auth != nil {
			if ok, _ := cl.Extension("AUTH"); ok {
				_ = conn.SetDeadline(now.Add(time.Duration(toSec) * time.Second))
				if err := cl.Auth(auth); err != nil {
					log.Printf("smtp auth error: %v", err)
					return c.Status(502).JSON(fiber.Map{"error": "mail auth failed"})
				}
			}
		}
		_ = conn.SetDeadline(now.Add(time.Duration(toSec) * time.Second))
		if err := cl.Mail(from); err != nil {
			log.Printf("smtp mail from error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail from failed"})
		}
		_ = conn.SetDeadline(now.Add(time.Duration(toSec) * time.Second))
		if err := cl.Rcpt(body.Email); err != nil {
			log.Printf("smtp rcpt to error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail rcpt failed"})
		}
		_ = conn.SetDeadline(now.Add(time.Duration(toSec) * time.Second))
		w, err := cl.Data()
		if err != nil {
			log.Printf("smtp data start error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail data failed"})
		}
		_ = conn.SetDeadline(now.Add(time.Duration(toSec) * time.Second))
		if _, err := w.Write(msg); err != nil {
			_ = w.Close()
			log.Printf("smtp data write error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail write failed"})
		}
		_ = conn.SetDeadline(now.Add(time.Duration(toSec) * time.Second))
		if err := w.Close(); err != nil {
			log.Printf("smtp data close error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail finalize failed"})
		}
		// store code after successful send
		exp := now.Add(10 * time.Minute)
		lc := domain.LoginCode{Email: email, HashedCode: utils.Sha256Hex([]byte(code)), ExpiresAt: exp, CreatedAt: now}
		if err := h.db.Create(&lc).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "server"})
		}
		return c.SendStatus(fiber.StatusOK)
	})

	// auth options
	router.Post("/options", func(c *fiber.Ctx) error {
		var body struct {
			Email string `json:"email"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		email := strings.ToLower(strings.TrimSpace(body.Email))
		if !utils.IsEmail(email) {
			return utils.ValidationError(c, map[string]string{"email": "invalid email"})
		}
		var u domain.User
		if err := h.db.Where("email = ?", email).First(&u).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// User must already exist for login; do not auto-create
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user_not_found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		// Treat logically deleted users as non-existent for login
		if u.IsDeleted {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user_not_found"})
		}
		hasUser := true
		hasPassword := false
		if hasUser && u.HashedPassword != nil && *u.HashedPassword != "" {
			hasPassword = true
		}
		totpEnabled := false
		if hasUser {
			if ok, _, _ := utils.GetActiveTOTP(u.ID, h.db); ok {
				totpEnabled = true
			}
		}
		// passkey presence
		passkeyEnabled := false
		if hasUser {
			var cnt int64
			_ = h.db.Model(&domain.Passkey{}).Where("user_id = ?", u.ID).Count(&cnt).Error
			passkeyEnabled = cnt > 0
		}
		// recovery codes presence (independent of TOTP)
		recoveryAvailable := false
		if hasUser {
			var rc int64
			_ = h.db.Model(&domain.RecoveryCode{}).Where("user_id = ? AND is_used = 0", u.ID).Count(&rc).Error
			recoveryAvailable = rc > 0
		}
		methods := []string{}
		if hasPassword {
			methods = append(methods, "password")
		}
		if u.EmailCodeAllowed {
			methods = append(methods, "email_code")
		}
		if totpEnabled {
			methods = append(methods, "totp")
		}
		if recoveryAvailable {
			methods = append(methods, "recovery")
		}
		if passkeyEnabled {
			methods = append(methods, "passkey")
		}
		return c.JSON(fiber.Map{
			"has_password":        hasPassword,
			"totp_enabled":        totpEnabled,
			"passkey_enabled":     passkeyEnabled,
			"email_login_enabled": u.EmailCodeAllowed,
			"methods":             methods,
		})
	})

	// login
	router.Post("/login", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		now := time.Now().In(jst)
		var body struct {
			Email             string `json:"email"`
			Password          string `json:"password"`
			AuthTransactionID string `json:"auth_transaction"`
			Code              string `json:"code"`
			Totp              string `json:"totp"`
			Recovery          string `json:"recovery_code"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		platform := strings.ToLower(strings.TrimSpace(c.Get("X-Client-Platform")))
		if platform == "" {
			platform = "web"
		}
		if !utils.IsAllowedPlatform(platform) {
			return c.Status(400).JSON(fiber.Map{"error": "invalid platform"})
		}
		if err := validateLoginClient(c, platform); err != nil {
			return err
		}

		email := strings.ToLower(strings.TrimSpace(body.Email))
		var u domain.User
		usingTransaction := strings.TrimSpace(body.AuthTransactionID) != ""
		var transaction *authTransaction
		var releaseTransaction func()

		if usingTransaction {
			if body.Password != "" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password is not accepted with an auth transaction"})
			}
			var err error
			transaction, releaseTransaction, err = h.lockAuthTransaction(c.Context(), body.AuthTransactionID)
			if errors.Is(err, errAuthTransactionNotFound) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "auth transaction invalid or expired"})
			}
			if errors.Is(err, errAuthTransactionBusy) {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "auth transaction is already in use"})
			}
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "auth transaction"})
			}
			defer releaseTransaction()

			failTransaction := func(message string) error {
				exhausted, err := h.recordAuthTransactionFailure(c.Context(), body.AuthTransactionID, transaction)
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "auth transaction"})
				}
				if exhausted {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "auth transaction attempts exceeded"})
				}
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": message})
			}

			if transaction.ClientContext != authTransactionContext(c, platform) {
				return failTransaction("auth transaction client mismatch")
			}
			if email != "" && email != transaction.Email {
				return failTransaction("auth transaction user mismatch")
			}
			if err := h.db.First(&u, transaction.UserID).Error; err != nil || u.IsDeleted {
				_ = h.consumeAuthTransaction(c.Context(), body.AuthTransactionID)
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user_not_found"})
			}
			email = u.Email
			if allowed, err := h.enforceLoginGuards(c, u, now); !allowed {
				return err
			}

			method := ""
			providedMethods := 0
			if body.Code != "" {
				method = "email_code"
				providedMethods++
			}
			if body.Totp != "" {
				method = "totp"
				providedMethods++
			}
			if body.Recovery != "" {
				method = "recovery"
				providedMethods++
			}
			if providedMethods != 1 || !containsAuthMethod(transaction.AllowedMethods, method) {
				return failTransaction("mfa method not allowed")
			}

			valid := false
			switch method {
			case "email_code":
				if u.EmailCodeAllowed {
					var lc domain.LoginCode
					if err := h.db.Where("email = ? AND consumed_at IS NULL AND expires_at > ?", email, now).Order("id DESC").First(&lc).Error; err == nil {
						if lc.LockedUntil != nil && lc.LockedUntil.After(now) {
							return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "locked due to failed attempts"})
						}
						if lc.HashedCode == utils.Sha256Hex([]byte(body.Code)) {
							_ = h.db.Model(&domain.LoginCode{}).Where("id = ?", lc.ID).Update("consumed_at", now).Error
							valid = true
						} else {
							failCount := lc.FailedAttempts + 1
							updates := map[string]interface{}{"FailedAttempts": failCount}
							if failCount >= 5 {
								updates["LockedUntil"] = lc.ExpiresAt
							}
							_ = h.db.Model(&domain.LoginCode{}).Where("id = ?", lc.ID).Updates(updates).Error
						}
					}
				}
			case "totp":
				if active, secret, _ := utils.GetActiveTOTP(u.ID, h.db); active {
					valid = utils.ValidateTOTP(secret, body.Totp)
				}
			case "recovery":
				valid = utils.ConsumeRecoveryCode(u.ID, body.Recovery, h.db)
			}
			if !valid {
				return failTransaction("invalid mfa")
			}
			if err := h.consumeAuthTransaction(c.Context(), body.AuthTransactionID); err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "auth transaction invalid or expired"})
			}
		} else {
			if !utils.IsEmail(email) {
				return utils.ValidationError(c, map[string]string{"email": "invalid email"})
			}
			// Lookup user; do not auto-create for non-existing accounts.
			if err := h.db.Where("email = ?", email).First(&u).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user_not_found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
			if u.IsDeleted {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user_not_found"})
			}
			if allowed, err := h.enforceLoginGuards(c, u, now); !allowed {
				return err
			}
		}
		if !usingTransaction {
			hasPassword := u.HashedPassword != nil && *u.HashedPassword != ""
			totpActive, totpSecret, _ := utils.GetActiveTOTP(u.ID, h.db)
			var recoveryCount int64
			_ = h.db.Model(&domain.RecoveryCode{}).Where("user_id = ? AND is_used = 0", u.ID).Count(&recoveryCount).Error
			allowedMFA := make([]string, 0, 3)
			if u.EmailCodeAllowed {
				allowedMFA = append(allowedMFA, "email_code")
			}
			if totpActive {
				allowedMFA = append(allowedMFA, "totp")
			}
			if recoveryCount > 0 {
				allowedMFA = append(allowedMFA, "recovery")
			}

			if hasPassword {
				if body.Password == "" {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "password required"})
				}
				if body.Code != "" || body.Totp != "" || body.Recovery != "" {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "submit password before mfa"})
				}
				if err := bcrypt.CompareHashAndPassword([]byte(*u.HashedPassword), []byte(body.Password)); err != nil {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid password"})
				}
				if len(allowedMFA) > 0 {
					rawID, err := h.createAuthTransaction(c.Context(), authTransaction{
						UserID:         u.ID,
						Email:          u.Email,
						AllowedMethods: allowedMFA,
						ClientContext:  authTransactionContext(c, platform),
					})
					if err != nil {
						return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "auth transaction"})
					}
					return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
						"mfa_required":     true,
						"auth_transaction": rawID,
						"expires_in":       int(authTransactionTTL.Seconds()),
						"methods":          allowedMFA,
					})
				}
			} else {
				if len(allowedMFA) == 0 {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "no auth method"})
				}
				ok := false
				// Passwordless accounts continue to authenticate directly with one factor.
				if u.EmailCodeAllowed && body.Code != "" {
					var lc domain.LoginCode
					if err := h.db.Where("email = ? AND consumed_at IS NULL AND expires_at > ?", email, now).Order("id DESC").First(&lc).Error; err == nil {
						if lc.LockedUntil != nil && lc.LockedUntil.After(now) {
							return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "locked due to failed attempts"})
						}
						if lc.HashedCode == utils.Sha256Hex([]byte(body.Code)) {
							_ = h.db.Model(&domain.LoginCode{}).Where("id = ?", lc.ID).Update("consumed_at", now).Error
							ok = true
						} else {
							failCount := lc.FailedAttempts + 1
							update := map[string]interface{}{"FailedAttempts": failCount}
							if failCount >= 5 {
								update["LockedUntil"] = lc.ExpiresAt
							}
							_ = h.db.Model(&domain.LoginCode{}).Where("id = ?", lc.ID).Updates(update).Error
						}
					}
				}
				// TOTP
				if !ok && totpActive && body.Totp != "" {
					if utils.ValidateTOTP(totpSecret, body.Totp) {
						ok = true
					}
				}
				if !ok && recoveryCount > 0 && body.Recovery != "" {
					if utils.ConsumeRecoveryCode(u.ID, body.Recovery, h.db) {
						ok = true
					}
				}
				if !ok {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "mfa required"})
				}
			}
		}
		// Mark user as registered upon first successful login
		if !u.IsRegistered {
			_ = h.db.Model(&domain.User{}).Where("id = ? AND is_registered = 0", u.ID).Update("is_registered", true).Error
		}
		return h.issueSession(c, u.ID, platform, now)
	})

	router.Post("/logout", func(c *fiber.Ctx) error {
		token := utils.SessionTokenFromRequest(c)
		if token != "" {
			var session domain.Session
			_ = h.db.Where("hashed_token = ? OR hashed_token = ?", utils.Sha256Hex([]byte(token)), token).Order("id DESC").First(&session).Error
			utils.RevokeSession(c, token, session.UserID, h.db, h.rdb)
		}
		utils.ClearSessionCookie(c)
		return c.SendStatus(fiber.StatusNoContent)
	})
}

func formatLoginCodeSubject(subject, code string) string {
	return strings.TrimSpace(subject) + ": " + code
}

// composeLoginCodeGRPC calls emailcompose.EmailComposer.ComposeLoginCode via gRPC reflection dynamically.
func composeLoginCodeGRPC(ctx context.Context, addr, locale, email, code, appName string) (subject, html, text string, err error) {
	cc, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", "", "", err
	}
	defer cc.Close()

	// Load proto descriptor from embedded email.proto (no server reflection required)
	parser := protoparse.Parser{
		Accessor: func(filename string) (io.ReadCloser, error) {
			if filename != "email.proto" {
				return nil, fmt.Errorf("proto not found: %s", filename)
			}
			return io.NopCloser(strings.NewReader(emailComposeProto)), nil
		},
	}
	fds, perr := parser.ParseFiles("email.proto")
	if perr != nil || len(fds) == 0 {
		return "", "", "", fmt.Errorf("proto parse error: %v", perr)
	}
	fd := fds[0]
	svc := fd.FindService("emailcompose.EmailComposer")
	if svc == nil {
		return "", "", "", fmt.Errorf("service emailcompose.EmailComposer not found in proto")
	}
	m := svc.FindMethodByName("ComposeLoginCode")
	if m == nil {
		return "", "", "", fmt.Errorf("method not found")
	}
	inDesc := m.GetInputType()
	outDesc := m.GetOutputType()
	in := dynamic.NewMessage(inDesc)
	// set fields
	if f := inDesc.FindFieldByName("locale"); f != nil {
		_ = in.TrySetField(f, locale)
	}
	if f := inDesc.FindFieldByName("email"); f != nil {
		_ = in.TrySetField(f, email)
	}
	if f := inDesc.FindFieldByName("code"); f != nil {
		_ = in.TrySetField(f, code)
	}
	if f := inDesc.FindFieldByName("app_name"); f != nil {
		_ = in.TrySetField(f, appName)
	}
	stub := grpcdyn.NewStub(cc)
	// InvokeRpc returns (proto.Message, error); capture the returned message and cast to *dynamic.Message
	outMsg, err := stub.InvokeRpc(ctx, m, in)
	if err != nil {
		return "", "", "", err
	}
	dm, ok := outMsg.(*dynamic.Message)
	if !ok {
		return "", "", "", fmt.Errorf("unexpected response type")
	}
	// extract fields
	if f := outDesc.FindFieldByName("subject"); f != nil {
		if v, _ := dm.TryGetField(f); v != nil {
			subject, _ = v.(string)
		}
	}
	if f := outDesc.FindFieldByName("html"); f != nil {
		if v, _ := dm.TryGetField(f); v != nil {
			html, _ = v.(string)
		}
	}
	if f := outDesc.FindFieldByName("text"); f != nil {
		if v, _ := dm.TryGetField(f); v != nil {
			text, _ = v.(string)
		}
	}
	if strings.TrimSpace(subject) == "" {
		return "", "", "", fmt.Errorf("empty subject from composer")
	}
	return subject, html, text, nil
}

const emailComposeProto = `syntax = "proto3";
package emailcompose;

service EmailComposer {
  rpc ComposeLoginCode (ComposeLoginCodeRequest) returns (ComposeResponse) {}
  rpc ComposeTaskDigest (ComposeTaskDigestRequest) returns (ComposeResponse) {}
  rpc ComposeModerationAlert (ComposeModerationAlertRequest) returns (ComposeResponse) {}
  rpc ComposeDayShift (ComposeDayShiftRequest) returns (ComposeResponse) {}
  rpc ComposeDayShiftSkip (ComposeDayShiftSkipRequest) returns (ComposeResponse) {}
}

message ComposeLoginCodeRequest {
  string locale = 1; // 'en' or 'ja'
  string email = 2;  // recipient for personalization
  string code = 3;   // login code
  string app_name = 4;
}

message ComposeResponse {
  string subject = 1;
  string html = 2;
  string text = 3;
}

message TaskDigestBadge {
  string label = 1;
  string variant = 2; // "gray" | "red" | "blue"
}

message TaskDigestItem {
  string when_label = 1;
  string subject = 2;
  string title = 3;
  string body = 4;
  string meta = 5;
  repeated TaskDigestBadge badges = 6;
}

message ComposeTaskDigestRequest {
  string locale = 1;  // 'en' or 'ja'
  string app_name = 2; // e.g. Taskly
  repeated TaskDigestItem items = 3;
}

message ModerationHighlight {
  string id = 1;
  string title = 2;
  string category = 3;
  string severity = 4;
}

message ComposeModerationAlertRequest {
  string locale = 1; // 'en' or 'ja'
  string app_name = 2;
  int32 total_flagged = 3;
  int32 auto_hidden = 4;
  string review_url = 5;
  repeated ModerationHighlight highlights = 6;
}

message ComposeDayShiftRequest {
  string locale = 1; // 'en' or 'ja'
  string app_name = 2; // e.g. Taskly
  repeated string members = 3;
  repeated string events = 4;
}

message ComposeDayShiftSkipRequest {
  string locale = 1; // 'en' or 'ja'
  string app_name = 2;
  string today_date = 3; // YYYY-MM-DD
  string today_names = 4; // comma separated display names
  string next_date = 5; // YYYY-MM-DD
  string next_names = 6; // comma separated display names
}
`
