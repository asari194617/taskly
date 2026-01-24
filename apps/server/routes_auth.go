package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	grpcdyn "github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/gorm"
)

func registerAuthRoutes(app *fiber.App) {
	// request login code via email
	app.Post("/auth/request-code", func(c *fiber.Ctx) error {
		var body struct {
			Email string `json:"email"`
		}
		if err := parseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		email := strings.ToLower(strings.TrimSpace(body.Email))
		if !isEmail(email) {
			return validationError(c, map[string]string{"email": "invalid email"})
		}
		ip := c.IP()
		// IP rate limit (max 10 req/10min)
		ipKey := "login_code_req_ip:" + ip
		ipCount, _ := RDB.Incr(c.Context(), ipKey).Result()
		if ipCount == 1 {
			_ = RDB.Expire(c.Context(), ipKey, 10*time.Minute)
		}
		if ipCount > 10 {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "ip rate limit"})
		}
		// Email rate limit (max 5 req/10min)
		emailKey := "login_code_req_email:" + email
		emailCount, _ := RDB.Incr(c.Context(), emailKey).Result()
		if emailCount == 1 {
			_ = RDB.Expire(c.Context(), emailKey, 10*time.Minute)
		}
		if emailCount > 5 {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "email rate limit"})
		}
		// cooldown: deny if a code was created within last 60s and not expired
		var recent LoginCode
		if err := DB.Where("email = ?", email).Order("id DESC").First(&recent).Error; err == nil {
			if time.Since(recent.CreatedAt) < 60*time.Second && recent.ConsumedAt == nil {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many requests"})
			}
			// ロック中なら拒否
			if recent.LockedUntil != nil && recent.LockedUntil.After(time.Now()) {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "locked due to failed attempts"})
			}
		}
		// generate code
		code := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
		// In development, log the code for testing when email delivery is unreliable
		if strings.ToLower(env("NODE_ENV", "development")) == "development" {
			log.Printf("[DEV] Login code for %s: %s", email, code)
		}
		// Compose email via email-compose service (gRPC)
		var subject, html, text string
		if s, h, t, err := composeLoginCodeGRPC(c.Context(), env("EMAIL_COMPOSE_GRPC_ADDR", "email-compose:50051"), "ja", email, code, "Taskly"); err == nil {
			subject, html, text = s, h, t
		}
		// Fallback subject/body if composer failed
		if subject == "" {
			subject = "Taskly Login Code"
			html = buildEmailHTML("ログインコード", fmt.Sprintf("<p>以下の6桁コードを入力してください。</p><div style=\"font-family:ui-monospace, Menlo, Consolas, monospace; font-size:20px; letter-spacing:4px; font-weight:700;\">%s</div><p style=\"color:#666;\">5分以内に入力してください。</p>", code))
			text = fmt.Sprintf("Your login code is: %s\nIt expires in 5 minutes.\n", code)
		}
		smtpHost := env("SMTP_HOST", "localhost")
		smtpPort := env("SMTP_PORT", "25")
		from := env("SMTP_FROM", "noreply@taskly.local")
		smtpUser := env("SMTP_USER", from)
		smtpPass := env("SMTP_PASSWORD", "")
		// Build RFC5322 message with required headers (From/To/Subject/Date) - HTML email
		nowRFC := time.Now().Format(time.RFC1123Z)
		boundary := fmt.Sprintf("taskly-%d", time.Now().UnixNano())
		msg := []byte(
			"From: " + from + "\r\n" +
				"To: " + body.Email + "\r\n" +
				"Subject: " + "[" + code + "]" + subject + "\r\n" +
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
		toSec, _ := strconv.Atoi(env("SMTP_TIMEOUT", "60"))
		if toSec <= 0 {
			toSec = 60
		}
		dialer := &net.Dialer{Timeout: time.Duration(toSec) * time.Second}
		mode := strings.ToLower(strings.TrimSpace(env("SMTP_TLS", ""))) // "starttls" | "smtps" | ""
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
		_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
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
				_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
				if err := cl.Auth(auth); err != nil {
					log.Printf("smtp auth error: %v", err)
					return c.Status(502).JSON(fiber.Map{"error": "mail auth failed"})
				}
			}
		}
		_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
		if err := cl.Mail(from); err != nil {
			log.Printf("smtp mail from error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail from failed"})
		}
		_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
		if err := cl.Rcpt(body.Email); err != nil {
			log.Printf("smtp rcpt to error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail rcpt failed"})
		}
		_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
		w, err := cl.Data()
		if err != nil {
			log.Printf("smtp data start error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail data failed"})
		}
		_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
		if _, err := w.Write(msg); err != nil {
			_ = w.Close()
			log.Printf("smtp data write error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail write failed"})
		}
		_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
		if err := w.Close(); err != nil {
			log.Printf("smtp data close error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "mail finalize failed"})
		}
		// store code after successful send
		exp := time.Now().Add(10 * time.Minute)
		lc := LoginCode{Email: email, HashedCode: sha256Hex([]byte(code)), ExpiresAt: exp}
		if err := DB.Create(&lc).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "server"})
		}
		return c.SendStatus(fiber.StatusOK)
	})

	// auth options
	app.Post("/auth/options", func(c *fiber.Ctx) error {
		var body struct {
			Email string `json:"email"`
		}
		if err := parseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		email := strings.ToLower(strings.TrimSpace(body.Email))
		if !isEmail(email) {
			return validationError(c, map[string]string{"email": "invalid email"})
		}
		var u User
		if err := DB.Where("email = ?", email).First(&u).Error; err != nil {
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
			if ok, _, _ := getActiveTOTP(u.ID); ok {
				totpEnabled = true
			}
		}
		// passkey presence
		passkeyEnabled := false
		if hasUser {
			var cnt int64
			_ = DB.Model(&Passkey{}).Where("user_id = ?", u.ID).Count(&cnt).Error
			passkeyEnabled = cnt > 0
		}
		// recovery codes presence (independent of TOTP)
		recoveryAvailable := false
		if hasUser {
			var rc int64
			_ = DB.Model(&RecoveryCode{}).Where("user_id = ? AND is_used = 0", u.ID).Count(&rc).Error
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
	app.Post("/auth/login", func(c *fiber.Ctx) error {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Code     string `json:"code"`
			Totp     string `json:"totp"`
			Recovery string `json:"recovery_code"`
		}
		if err := parseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		email := strings.ToLower(strings.TrimSpace(body.Email))
		if !isEmail(email) {
			return validationError(c, map[string]string{"email": "invalid email"})
		}
		// Lookup user; do not auto-create for non-existing accounts
		var u User
		if err := DB.Where("email = ?", email).First(&u).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user_not_found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		// Treat logically deleted users as non-existent for login
		if u.IsDeleted {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user_not_found"})
		}
		ip := c.IP()
		// IP rate limit (max 20 req/10min)
		ipKey := "login_attempt_ip:" + ip
		ipCount, _ := RDB.Incr(c.Context(), ipKey).Result()
		if ipCount == 1 {
			_ = RDB.Expire(c.Context(), ipKey, 10*time.Minute)
		}
		if ipCount > 20 {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "ip rate limit"})
		}
		// Email rate limit (max 10 req/10min)
		emailKey := "login_attempt_email:" + u.Email
		emailCount, _ := RDB.Incr(c.Context(), emailKey).Result()
		if emailCount == 1 {
			_ = RDB.Expire(c.Context(), emailKey, 10*time.Minute)
		}
		if emailCount > 10 {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "email rate limit"})
		}
		// Blocklist enforcement
		var cnt int64
		if err := DB.Model(&Block{}).
			Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", u.ID, time.Now()).
			Count(&cnt).Error; err == nil && cnt > 0 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "blocked"})
		}
		// Primary factor: if password is set, require password; otherwise allow email code
		reqPassword := (u.HashedPassword != nil && *u.HashedPassword != "")
		primaryOK := false
		if reqPassword {
			if body.Password == "" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "password required"})
			}
			// verify bcrypt password
			if err := bcrypt.CompareHashAndPassword([]byte(*u.HashedPassword), []byte(body.Password)); err == nil {
				primaryOK = true
			} else {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid password"})
			}
		} else {
			// Respect user setting for email code login
			if !u.EmailCodeAllowed {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "email code disabled"})
			}
			// verify login code (email magic code) using DB
			if body.Code == "" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "code required"})
			}
			email := strings.ToLower(strings.TrimSpace(body.Email))
			var lc LoginCode
			if err := DB.Where("email = ? AND consumed_at IS NULL AND expires_at > ?", email, time.Now()).Order("id DESC").First(&lc).Error; err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid code"})
			}
			// ロック中なら拒否
			if lc.LockedUntil != nil && lc.LockedUntil.After(time.Now()) {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "locked due to failed attempts"})
			}
			if lc.HashedCode != sha256Hex([]byte(body.Code)) {
				// 失敗回数インクリメント
				failCount := lc.FailedAttempts + 1
				update := map[string]interface{}{"FailedAttempts": failCount}
				// 5回以上失敗で有効期限までロック
				if failCount >= 5 {
					update["LockedUntil"] = lc.ExpiresAt
				}
				_ = DB.Model(&LoginCode{}).Where("id = ?", lc.ID).Updates(update).Error
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid code"})
			}
			now := time.Now()
			_ = DB.Model(&LoginCode{}).Where("id = ?", lc.ID).Update("consumed_at", now).Error
			primaryOK = true
		}
		if !primaryOK {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		// 2FA enforcement (TOTP or recovery code)
		if active, secret, _ := getActiveTOTP(u.ID); active {
			ok := false
			if body.Totp != "" {
				if validateTOTP(secret, body.Totp) {
					ok = true
				}
			}
			if !ok && body.Recovery != "" {
				if consumeRecoveryCode(u.ID, body.Recovery) {
					ok = true
				}
			}
			if !ok {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "2fa required"})
			}
		}
		// Create session token (opaque, strong randomness) and store in Redis
		plat := strings.ToLower(strings.TrimSpace(c.Get("X-Client-Platform")))
		if plat == "" {
			plat = "web"
		}
		if !isAllowedPlatform(plat) {
			return c.Status(400).JSON(fiber.Map{"error": "invalid platform"})
		}
		token := generateSessionToken(u.ID, plat)
		ttl := time.Hour * 24 * 30
		if err := RDB.Set(c.Context(), "session:"+token, fmt.Sprintf("%d", u.ID), ttl).Err(); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "session"})
		}
		// track user sessions for revocation
		_ = RDB.SAdd(c.Context(), fmt.Sprintf("user_sessions:%d", u.ID), token).Err()
		_ = RDB.Expire(c.Context(), fmt.Sprintf("user_sessions:%d", u.ID), ttl).Err()
		// Mark user as registered upon first successful login
		if !u.IsRegistered {
			_ = DB.Model(&User{}).Where("id = ? AND is_registered = 0", u.ID).Update("is_registered", true).Error
		}
		// Store hash of token and associated user id for DB-fallback validation
		DB.Create(&Session{UserID: u.ID, ClientPlatform: plat, HashedToken: sha256Hex([]byte(token)), ExpiresAt: ptrTime(time.Now().Add(ttl))})

		// Also provide a JWT if needed (optional)
		claims := jwt.MapClaims{"sub": u.ID, "exp": time.Now().Add(ttl).Unix()}
		jwtt := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		jwtStr, _ := jwtt.SignedString([]byte(env("JWT_SECRET", "dev")))

		return c.JSON(fiber.Map{"token": token, "jwt": jwtStr})
	})

	app.Post("/auth/logout", func(c *fiber.Ctx) error {
		token := parseAuthToken(c.Get("Authorization"))
		if token != "" {
			_ = RDB.Del(c.Context(), "session:"+token).Err()
			// Best-effort: remove DB session record if present (hashed or legacy plain)
			_ = DB.Where("hashed_token = ? OR hashed_token = ?", sha256Hex([]byte(token)), token).Delete(&Session{}).Error
			// remove from user_sessions set if we can resolve uid
			if uidStr, err := RDB.Get(c.Context(), "session:"+token).Result(); err == nil && uidStr != "" {
				_ = RDB.SRem(c.Context(), fmt.Sprintf("user_sessions:%s", uidStr), token).Err()
			}
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
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
