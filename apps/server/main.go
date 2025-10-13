package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Globals moved to globals.go

// env moved to support.go

// ptrTime moved to support.go

func main() {
	DB = mustInitDB()
	RDB = mustInitRedis()
	metricsServer(env("METRICS_BIND_ADDR", "127.0.0.1:9464"))

	// Init WebAuthn (Passkey)
	initWebAuthn()

	// Auto-migrate core models
	_ = DB.AutoMigrate(&User{}, &Session{}, &RoleUser{}, &Permission{}, &Block{}, &APIKey{}, &Subject{}, &Task{}, &ApiActionLog{}, &TOTPSecret{}, &RecoveryCode{}, &Passkey{}, &LoginCode{}, &DayShiftCycle{}, &DayShiftAssignment{}, &DayShiftAssignmentUser{}, &BusinessCalendar{}, &IcalKey{}, &DayShiftCyclePair{}, &MCP{}, &Log{}, &WebPushSubscription{})

	app := fiber.New()
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     env("API_CORS_ORIGINS", "http://localhost:4321"),
		AllowCredentials: strings.ToLower(env("CORS_ALLOW_CREDENTIALS", "true")) == "true",
		AllowHeaders:     "Authorization, Content-Type",
	}))

	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendString("ok") })
	// GraphQL routes
	registerGraphQL(app)

	// Day shift scheduler (08:30 local) – minimal implementation
	if strings.ToLower(env("DAYSHIFT_ENABLED", "true")) == "true" {
		notifyTime := env("DAYSHIFT_NOTIFY_TIME", "08:30")
		go dayShiftScheduler(notifyTime)
	}

	// Daily tasks email digest (07:30 local by default)
	if strings.ToLower(env("TASK_DIGEST_ENABLED", "true")) == "true" {
		go tasksNotifyScheduler(env("TASK_DIGEST_TIME", "07:30"))
	}

	// Auth endpoints (with email code delivery)
	app.Post("/auth/request-code", func(c *fiber.Ctx) error {
		var body struct {
			Email string `json:"email"`
		}
		if err := c.BodyParser(&body); err != nil || body.Email == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		email := strings.ToLower(strings.TrimSpace(body.Email))
		// cooldown: deny if a code was created within last 60s and not expired
		var recent LoginCode
		if err := DB.Where("email = ?", email).Order("id DESC").First(&recent).Error; err == nil {
			if time.Since(recent.CreatedAt) < 60*time.Second && recent.ConsumedAt == nil {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many requests"})
			}
		}
		// generate code
		code := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
		// send email via SMTP synchronously; only store code if send succeeds
		smtpHost := env("SMTP_HOST", "localhost")
		smtpPort := env("SMTP_PORT", "25")
		from := env("SMTP_FROM", "noreply@taskly.local")
		smtpUser := env("SMTP_USER", from)
		smtpPass := env("SMTP_PASSWORD", "")
		// Build RFC5322 message with required headers (From/To/Subject/Date) - HTML email
		nowRFC := time.Now().Format(time.RFC1123Z)
		html := buildEmailHTML("ログインコード", fmt.Sprintf("<p>以下の6桁コードを入力してください。</p><div style=\"font-family:ui-monospace, Menlo, Consolas, monospace; font-size:20px; letter-spacing:4px; font-weight:700;\">%s</div><p style=\"color:#666;\">10分以内に入力してください。</p>", code))
		text := fmt.Sprintf("Your login code is: %s\nIt expires in 10 minutes.\n", code)
		boundary := fmt.Sprintf("taskly-%d", time.Now().UnixNano())
		msg := []byte(
			"From: " + from + "\r\n" +
				"To: " + body.Email + "\r\n" +
				"Subject: Taskly Login Code\r\n" +
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

	// Discover auth options for an email (password/TOTP presence)
	app.Post("/auth/options", func(c *fiber.Ctx) error {
		var body struct {
			Email string `json:"email"`
		}
		if err := c.BodyParser(&body); err != nil || body.Email == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		var u User
		hasUser := DB.Where("email = ?", body.Email).First(&u).Error == nil
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
		methods := []string{}
		if hasPassword {
			methods = append(methods, "password")
		}
		if !hasUser || (hasUser && u.EmailCodeAllowed) {
			methods = append(methods, "email_code")
		}
		if totpEnabled {
			methods = append(methods, "totp", "recovery")
		}
		if passkeyEnabled {
			methods = append(methods, "passkey")
		}
		return c.JSON(fiber.Map{"has_password": hasPassword, "totp_enabled": totpEnabled, "passkey_enabled": passkeyEnabled, "email_login_enabled": func() bool {
			if hasUser {
				return u.EmailCodeAllowed
			}
			return true
		}(), "methods": methods})
	})

	app.Post("/auth/login", func(c *fiber.Ctx) error {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Code     string `json:"code"`
			Totp     string `json:"totp"`
			Recovery string `json:"recovery_code"`
		}
		if err := c.BodyParser(&body); err != nil || body.Email == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		// Lookup user or create placeholder
		var u User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				u = User{Email: body.Email, DisplayName: body.Email}
				if err := DB.Create(&u).Error; err != nil {
					return c.Status(500).JSON(fiber.Map{"error": "db"})
				}
			} else {
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
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
			if lc.HashedCode != sha256Hex([]byte(body.Code)) {
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
		// TODO: verify password if set

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
		// Optionally record DB session (hashed)
		DB.AutoMigrate(&Session{})
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

	// Web Push subscription endpoints (no push send yet)
	// push endpoints will be added after middlewares are defined

	// Auth and audit middlewares
	authRequired := func(c *fiber.Ctx) error {
		if userFromSession(c) || userFromAPIKey(c) {
			return c.Next()
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	audit := func(c *fiber.Ctx) error {
		err := c.Next()
		// Per policy: record ONLY REST requests authenticated with API Key
		if apiKey, ok := c.Locals("apiKey").(string); ok && apiKey != "" {
			hashed := sha256Hex([]byte(apiKey))
			var uid *uint
			if u, ok := c.Locals("user").(*User); ok {
				uid = &u.ID
			}
			_ = DB.Create(&ApiActionLog{UserID: uid, HashedToken: hashed, Action: c.Method() + " " + c.Path(), IP: c.IP()}).Error
		}
		return err
	}

	// Web Push subscription endpoints (no push send yet)
	app.Post("/push/subscribe", authRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct{ Endpoint, P256dh, Auth string }
		if err := c.BodyParser(&body); err != nil || body.Endpoint == "" || body.P256dh == "" || body.Auth == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var rec WebPushSubscription
		if err := DB.Where("user_id = ? AND endpoint = ?", u.ID, body.Endpoint).First(&rec).Error; err == nil {
			rec.P256dh, rec.Auth = body.P256dh, body.Auth
			_ = DB.Save(&rec).Error
		} else {
			_ = DB.Create(&WebPushSubscription{UserID: u.ID, Endpoint: body.Endpoint, P256dh: body.P256dh, Auth: body.Auth}).Error
		}
		return c.SendStatus(204)
	})
	app.Post("/push/unsubscribe", authRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct{ Endpoint string }
		_ = c.BodyParser(&body)
		if strings.TrimSpace(body.Endpoint) != "" {
			_ = DB.Where("user_id = ? AND endpoint = ?", u.ID, body.Endpoint).Delete(&WebPushSubscription{}).Error
		}
		return c.SendStatus(204)
	})

	// Auth settings endpoints
	app.Get("/settings/auth", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		// refresh user
		var fresh User
		_ = DB.First(&fresh, u.ID).Error
		hasPassword := fresh.HashedPassword != nil && *fresh.HashedPassword != ""
		totpEnabled, _, _ := getActiveTOTP(fresh.ID)
		return c.JSON(fiber.Map{
			"email_login_enabled": fresh.EmailCodeAllowed,
			"has_password":        hasPassword,
			"totp_enabled":        totpEnabled,
		})
	})
	app.Post("/settings/auth", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct {
			EmailLoginEnabled *bool `json:"email_login_enabled"`
		}
		if err := c.BodyParser(&body); err != nil || body.EmailLoginEnabled == nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var fresh User
		if err := DB.First(&fresh, u.ID).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		// Allow disabling email login if user has password or TOTP enabled
		hasPassword := fresh.HashedPassword != nil && *fresh.HashedPassword != ""
		totpEnabled, _, _ := getActiveTOTP(fresh.ID)
		var pkCount int64
		_ = DB.Model(&Passkey{}).Where("user_id = ?", fresh.ID).Count(&pkCount).Error
		if !*body.EmailLoginEnabled && !(hasPassword || totpEnabled || pkCount > 0) {
			return c.Status(400).JSON(fiber.Map{"error": "cannot disable without another auth factor"})
		}
		fresh.EmailCodeAllowed = *body.EmailLoginEnabled
		if err := DB.Model(&User{}).Where("id = ?", fresh.ID).Update("email_code_allowed", fresh.EmailCodeAllowed).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// WebAuthn: helpers
	fiberToHTTPRequest := func(c *fiber.Ctx) *http.Request {
		req, _ := http.NewRequestWithContext(c.Context(), "POST", "", io.NopCloser(strings.NewReader(string(c.Body()))))
		for k, v := range c.GetReqHeaders() {
			if len(v) > 0 {
				req.Header.Set(k, v[0])
			}
		}
		return req
	}

	// WebAuthn registration (requires auth)
	app.Post("/webauthn/register/begin", authRequired, audit, func(c *fiber.Ctx) error {
		if WAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		u := c.Locals("user").(*User)
		// Load credentials for user into interface methods
		creation, sd, err := WAuth.BeginRegistration(u)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "begin failed"})
		}
		b, _ := json.Marshal(sd)
		_ = RDB.Set(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID), string(b), 10*time.Minute).Err()
		return c.JSON(creation)
	})
	app.Post("/webauthn/register/finish", authRequired, audit, func(c *fiber.Ctx) error {
		if WAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		u := c.Locals("user").(*User)
		raw, _ := RDB.Get(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID)).Result()
		if raw == "" {
			return c.Status(400).JSON(fiber.Map{"error": "session expired"})
		}
		var sd webauthn.SessionData
		_ = json.Unmarshal([]byte(raw), &sd)
		req := fiberToHTTPRequest(c)
		cred, err := WAuth.FinishRegistration(u, sd, req)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "finish failed"})
		}
		// store credential
		pub := base64.RawStdEncoding.EncodeToString(cred.PublicKey)
		cid := base64.RawStdEncoding.EncodeToString(cred.ID)
		devName := "Passkey"
		rec := Passkey{
			UserID:       u.ID,
			CredentialID: cid,
			PublicKey:    pub,
			SignCount:    int64(cred.Authenticator.SignCount),
			DeviceName:   &devName,
		}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		_ = RDB.Del(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID)).Err()
		return c.SendStatus(204)
	})

	// WebAuthn login (no auth)
	app.Post("/webauthn/login/begin", func(c *fiber.Ctx) error {
		if WAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		var body struct {
			Email string `json:"email"`
		}
		if err := c.BodyParser(&body); err != nil || strings.TrimSpace(body.Email) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var u User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		assertion, sd, err := WAuth.BeginLogin(&u)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "begin failed"})
		}
		b, _ := json.Marshal(sd)
		_ = RDB.Set(c.Context(), fmt.Sprintf("webauthn:login:%d", u.ID), string(b), 10*time.Minute).Err()
		return c.JSON(assertion)
	})
	app.Post("/webauthn/login/finish", func(c *fiber.Ctx) error {
		if WAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		var body struct {
			Email string `json:"email"`
		}
		_ = c.BodyParser(&body)
		if strings.TrimSpace(body.Email) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var u User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		raw, _ := RDB.Get(c.Context(), fmt.Sprintf("webauthn:login:%d", u.ID)).Result()
		if raw == "" {
			return c.Status(400).JSON(fiber.Map{"error": "session expired"})
		}
		var sd webauthn.SessionData
		_ = json.Unmarshal([]byte(raw), &sd)
		req := fiberToHTTPRequest(c)
		cred, err := WAuth.FinishLogin(&u, sd, req)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "invalid passkey"})
		}
		// Blocklist enforcement before issuing session
		var blkCnt int64
		_ = DB.Model(&Block{}).
			Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", u.ID, time.Now()).
			Count(&blkCnt).Error
		if blkCnt > 0 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "blocked"})
		}
		// update sign count
		_ = DB.Model(&Passkey{}).Where("user_id = ? AND credential_id = ?", u.ID, base64.RawStdEncoding.EncodeToString(cred.ID)).Update("sign_count", int64(cred.Authenticator.SignCount)).Error
		// issue session token
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
		DB.Create(&Session{UserID: u.ID, ClientPlatform: plat, HashedToken: sha256Hex([]byte(token)), ExpiresAt: ptrTime(time.Now().Add(ttl))})
		_ = RDB.SAdd(c.Context(), fmt.Sprintf("user_sessions:%d", u.ID), token).Err()
		_ = RDB.Expire(c.Context(), fmt.Sprintf("user_sessions:%d", u.ID), ttl).Err()
		claims := jwt.MapClaims{"sub": u.ID, "exp": time.Now().Add(ttl).Unix()}
		jwtt := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		jwtStr, _ := jwtt.SignedString([]byte(env("JWT_SECRET", "dev")))
		_ = RDB.Del(c.Context(), fmt.Sprintf("webauthn:login:%d", u.ID)).Err()
		return c.JSON(fiber.Map{"token": token, "jwt": jwtStr})
	})

	// User profile: update display name
	app.Patch("/users/me", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct {
			DisplayName *string `json:"display_name"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		updates := map[string]any{}
		if body.DisplayName != nil {
			dn := strings.TrimSpace(*body.DisplayName)
			if len(dn) > 255 {
				dn = dn[:255]
			}
			updates["display_name"] = dn
		}
		if len(updates) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "no changes"})
		}
		if err := DB.Model(&User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// User account delete (soft delete)
	app.Delete("/users/me", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		if err := DB.Model(&User{}).Where("id = ?", u.ID).Update("is_deleted", true).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// API keys endpoints
	app.Get("/apikeys", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var keys []APIKey
		if err := DB.Where("user_id = ?", u.ID).Order("id DESC").Find(&keys).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		// Do not expose hashed tokens
		type resp struct {
			ID          uint      `json:"id"`
			Description *string   `json:"description"`
			IsValid     bool      `json:"is_valid"`
			CreatedAt   time.Time `json:"created_at"`
		}
		out := make([]resp, 0, len(keys))
		for _, k := range keys {
			out = append(out, resp{ID: k.ID, Description: k.Description, IsValid: k.IsValid, CreatedAt: k.CreatedAt})
		}
		return c.JSON(out)
	})

	app.Post("/apikeys", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		limit, err := apiKeyLimitForUser(u.ID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		if limit >= 0 { // -1 means unlimited
			var cur int64
			if err := DB.Model(&APIKey{}).Where("user_id = ? AND is_valid = 1", u.ID).Count(&cur).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
			if int(cur) >= limit {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "api key limit reached"})
			}
		}
		var body struct {
			Description *string `json:"description"`
		}
		_ = c.BodyParser(&body)
		plain, hashed, genErr := generateAPIKey()
		if genErr != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gen"})
		}
		rec := APIKey{UserID: u.ID, HashedToken: hashed, Description: body.Description}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": rec.ID, "token": plain})
	})

	app.Patch("/apikeys/:id", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var rec APIKey
		if err := DB.Where("id = ? AND user_id = ?", c.Params("id"), u.ID).First(&rec).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		var body struct {
			IsValid     *bool   `json:"is_valid"`
			Description *string `json:"description"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		if body.IsValid != nil {
			rec.IsValid = *body.IsValid
		}
		if body.Description != nil {
			rec.Description = body.Description
		}
		if err := DB.Save(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(fiber.Map{"id": rec.ID, "is_valid": rec.IsValid})
	})

	// Users/me (auth required)
	app.Get("/users/me", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		// roles
		var rls []RoleUser
		_ = DB.Where("user_id = ?", u.ID).Find(&rls).Error
		roles := make([]string, 0, len(rls))
		isAdmin := false
		isManager := false
		for _, r := range rls {
			roles = append(roles, r.Role)
			if r.Role == "admin" {
				isAdmin = true
			}
			if r.Role == "manager" {
				isManager = true
			}
		}
		return c.JSON(fiber.Map{"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "roles": roles, "is_admin": isAdmin, "is_manager": isManager})
	})

	// iCal token management
	app.Get("/ical/tokens", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var items []IcalKey
		if err := DB.Where("user_id = ?", u.ID).Order("id DESC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		base := strings.TrimRight(env("ICAL_BASE_URL", env("API_PUBLIC_URL", "http://localhost:8080")+"/ical"), "/")
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, map[string]any{
				"id":          it.ID,
				"description": it.Description,
				"url":         fmt.Sprintf("%s/%s.ics", base, it.Token),
				"created_at":  it.CreatedAt,
			})
		}
		return c.JSON(out)
	})
	app.Post("/ical/tokens", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct {
			Description *string `json:"description"`
		}
		_ = c.BodyParser(&body)
		tok := generateIcalToken()
		rec := IcalKey{UserID: u.ID, Token: tok, Description: body.Description}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		base := strings.TrimRight(env("ICAL_BASE_URL", env("API_PUBLIC_URL", "http://localhost:8080")+"/ical"), "/")
		return c.Status(201).JSON(fiber.Map{"id": rec.ID, "url": fmt.Sprintf("%s/%s.ics", base, rec.Token)})
	})
	app.Delete("/ical/tokens/:id", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		if err := DB.Where("id = ? AND user_id = ?", c.Params("id"), u.ID).Delete(&IcalKey{}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// Day shift admin endpoints (admin only)
	adminOnly := func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		isAdmin, err1 := userHasRole(u.ID, "admin")
		isManager, err2 := userHasRole(u.ID, "manager")
		if err1 != nil || err2 != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		}
		if !(isAdmin || isManager) {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		return c.Next()
	}
	app.Get("/admin/dayshift/assignments", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var items []DayShiftAssignment
		if err := DB.Order("date DESC").Limit(200).Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(items)
	})
	app.Post("/admin/dayshift/rotate", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		// rotate to next business day and assign next pair
		if err := performDayShiftRotation(time.Now()); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "rotate"})
		}
		return c.SendStatus(204)
	})
	// mark complete/unfulfilled
	app.Post("/admin/dayshift/:date/status", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		dateStr := c.Params("date") // YYYY-MM-DD
		var body struct {
			Status string  `json:"status"`
			Notes  *string `json:"notes"`
		}
		if err := c.BodyParser(&body); err != nil || (body.Status != "completed" && body.Status != "unfulfilled" && body.Status != "assigned") {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var asg DayShiftAssignment
		if err := DB.Where("date = ?", dateStr).First(&asg).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		asg.Status = body.Status
		asg.Notes = body.Notes
		if err := DB.Save(&asg).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
	// shift assignment to next business day (e.g., school closed)
	app.Post("/admin/dayshift/:date/shift", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		dateStr := c.Params("date")
		var asg DayShiftAssignment
		if err := DB.Where("date = ?", dateStr).First(&asg).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		next := nextBusinessDay(asg.Date)
		if next.IsZero() {
			return c.Status(400).JSON(fiber.Map{"error": "no next business day"})
		}
		var target DayShiftAssignment
		if err := DB.Where("date = ?", next.Format("2006-01-02")).First(&target).Error; err != nil {
			target = DayShiftAssignment{Date: next, CycleID: asg.CycleID, Status: "assigned"}
			if err := DB.Create(&target).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
		}
		var users []DayShiftAssignmentUser
		_ = DB.Where("day_shift_assignments_id = ?", asg.ID).Find(&users).Error
		_ = DB.Where("day_shift_assignments_id = ?", target.ID).Delete(&DayShiftAssignmentUser{}).Error
		for _, u := range users {
			_ = DB.Create(&DayShiftAssignmentUser{DayShiftAssignmentsID: target.ID, UserID: u.UserID}).Error
		}
		return c.SendStatus(204)
	})

	// cycle pair management
	app.Get("/admin/dayshift/cycles/:id/pairs", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var pairs []DayShiftCyclePair
		if err := DB.Where("cycle_id = ?", c.Params("id")).Order("position ASC, id ASC").Find(&pairs).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(pairs)
	})
	app.Post("/admin/dayshift/cycles/:id/pairs", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		cycleID, _ := strconv.Atoi(c.Params("id"))
		var body struct {
			User1ID  uint `json:"user1_id"`
			User2ID  uint `json:"user2_id"`
			Position *int `json:"position"`
		}
		if err := c.BodyParser(&body); err != nil || body.User1ID == 0 || body.User2ID == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		pos := 0
		if body.Position != nil {
			pos = *body.Position
		} else {
			var max int64
			_ = DB.Model(&DayShiftCyclePair{}).Where("cycle_id = ?", cycleID).Select("COALESCE(MAX(position),0)").Scan(&max).Error
			pos = int(max) + 1
		}
		rec := DayShiftCyclePair{CycleID: uint(cycleID), User1ID: body.User1ID, User2ID: body.User2ID, Position: pos}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(201).JSON(rec)
	})
	app.Delete("/admin/dayshift/cycles/:id/pairs/:pairId", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		if err := DB.Delete(&DayShiftCyclePair{}, "id = ? AND cycle_id = ?", c.Params("pairId"), c.Params("id")).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
	app.Post("/admin/dayshift/cycles/:id/pairs/:pairId/move", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var body struct {
			Position int `json:"position"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var rec DayShiftCyclePair
		if err := DB.Where("id = ? AND cycle_id = ?", c.Params("pairId"), c.Params("id")).First(&rec).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		rec.Position = body.Position
		if err := DB.Save(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(rec)
	})

	// cycles management
	app.Get("/admin/dayshift/cycles", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var items []DayShiftCycle
		if err := DB.Order("begin_date DESC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(items)
	})
	app.Post("/admin/dayshift/cycles", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var body struct {
			BeginDate   string `json:"begin_date"`
			FinishDate  string `json:"finish_date"`
			Description string `json:"description"`
		}
		if err := c.BodyParser(&body); err != nil || body.BeginDate == "" || body.FinishDate == "" || body.Description == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		bd, err1 := time.Parse("2006-01-02", body.BeginDate)
		fd, err2 := time.Parse("2006-01-02", body.FinishDate)
		if err1 != nil || err2 != nil || !bd.Before(fd) {
			return c.Status(400).JSON(fiber.Map{"error": "invalid dates"})
		}
		rec := DayShiftCycle{BeginDate: bd, FinishDate: fd, Description: body.Description}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(201).JSON(rec)
	})
	app.Delete("/admin/dayshift/cycles/:id", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		if err := DB.Delete(&DayShiftCycle{}, c.Params("id")).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// business calendar management (range updates)
	app.Post("/admin/calendar/upsert-range", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var body struct {
			Start      string  `json:"start"`
			End        string  `json:"end"`
			IsBusiness bool    `json:"is_business_day"`
			Notes      *string `json:"notes"`
		}
		if err := c.BodyParser(&body); err != nil || body.Start == "" || body.End == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		s, e := body.Start, body.End
		sd, err1 := time.Parse("2006-01-02", s)
		ed, err2 := time.Parse("2006-01-02", e)
		if err1 != nil || err2 != nil || sd.After(ed) {
			return c.Status(400).JSON(fiber.Map{"error": "invalid dates"})
		}
		for d := sd; !d.After(ed); d = d.Add(24 * time.Hour) {
			var bc BusinessCalendar
			if err := DB.Where("date = ?", d.Format("2006-01-02")).First(&bc).Error; err != nil {
				bc = BusinessCalendar{Date: dateOnly(d), IsBusinessDay: body.IsBusiness, Notes: body.Notes}
				_ = DB.Create(&bc).Error
			} else {
				bc.IsBusinessDay = body.IsBusiness
				bc.Notes = body.Notes
				_ = DB.Save(&bc).Error
			}
		}
		return c.SendStatus(204)
	})

	// Admin: manage permissions
	app.Get("/admin/permissions", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var items []Permission
		if err := DB.Order("id DESC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(items)
	})
	app.Post("/admin/permissions", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var body struct {
			UserID uint    `json:"user_id"`
			Target string  `json:"target"`
			Method string  `json:"method"`
			Value  *string `json:"value"`
		}
		if err := c.BodyParser(&body); err != nil || body.UserID == 0 || body.Target == "" || body.Method == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		rec := Permission{UserID: body.UserID, Target: body.Target, Method: strings.ToLower(body.Method), Value: body.Value}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(201).JSON(rec)
	})
	app.Delete("/admin/permissions/:id", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		if err := DB.Delete(&Permission{}, c.Params("id")).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// Admin: user management (list/search, roles, block/unblock)
	app.Get("/admin/users", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		q := strings.TrimSpace(c.Query("q"))
		limit, _ := strconv.Atoi(c.Query("limit", "100"))
		if limit <= 0 || limit > 500 {
			limit = 100
		}
		dbq := DB.Model(&User{})
		if q != "" {
			like := "%" + q + "%"
			dbq = dbq.Where("email LIKE ? OR display_name LIKE ?", like, like)
		}
		var users []User
		if err := dbq.Order("id DESC").Limit(limit).Find(&users).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		// fetch roles and block status
		ids := make([]uint, 0, len(users))
		for _, u := range users {
			ids = append(ids, u.ID)
		}
		var roles []RoleUser
		if len(ids) > 0 {
			_ = DB.Where("user_id IN ?", ids).Find(&roles).Error
		}
		roleMap := map[uint][]string{}
		for _, r := range roles {
			roleMap[r.UserID] = append(roleMap[r.UserID], r.Role)
		}
		// block status
		blocked := map[uint]bool{}
		if len(ids) > 0 {
			var blks []Block
			_ = DB.Where("user_id IN ? AND (expires_at IS NULL OR expires_at > ?)", ids, time.Now()).Find(&blks).Error
			for _, b := range blks {
				blocked[b.UserID] = true
			}
		}
		out := make([]map[string]any, 0, len(users))
		for _, u := range users {
			out = append(out, map[string]any{"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "is_deleted": u.IsDeleted, "roles": roleMap[u.ID], "is_blocked": blocked[u.ID]})
		}
		return c.JSON(out)
	})
	app.Post("/admin/users/:id/roles", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		uid := c.Params("id")
		var body struct {
			Role string `json:"role"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		role := strings.ToLower(strings.TrimSpace(body.Role))
		if role != "admin" && role != "manager" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid role"})
		}
		// check exists
		var cnt int64
		_ = DB.Model(&RoleUser{}).Where("user_id = ? AND role = ?", uid, role).Count(&cnt).Error
		if cnt == 0 {
			_ = DB.Create(&RoleUser{UserID: toUint(uid), Role: role}).Error
		}
		return c.SendStatus(201)
	})
	app.Delete("/admin/users/:id/roles/:role", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		if err := DB.Where("user_id = ? AND role = ?", c.Params("id"), c.Params("role")).Delete(&RoleUser{}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
	app.Post("/admin/users/:id/block", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var body struct {
			Reason    *string `json:"reason"`
			ExpiresAt *string `json:"expires_at"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var expPtr *time.Time
		if body.ExpiresAt != nil && *body.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, *body.ExpiresAt); err == nil {
				expPtr = &t
			} else {
				return c.Status(400).JSON(fiber.Map{"error": "invalid expires_at"})
			}
		}
		rec := Block{UserID: toUint(c.Params("id")), Reason: body.Reason, ExpiresAt: expPtr}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(201)
	})
	app.Post("/admin/users/:id/unblock", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		now := time.Now()
		if err := DB.Model(&Block{}).Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", c.Params("id"), now).Updates(map[string]any{"expires_at": now}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// 2FA: TOTP setup, activate, disable
	app.Post("/2fa/totp/setup", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		// generate base32 secret
		secret := generateBase32Secret()
		// store in Redis temporary
		key := fmt.Sprintf("totp-setup:%d", u.ID)
		if err := RDB.Set(c.Context(), key, secret, 10*time.Minute).Err(); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "server"})
		}
		issuer := "Taskly"
		label := fmt.Sprintf("%s:%s", issuer, u.Email)
		otpauth := fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s", urlEncode(label), secret, urlEncode(issuer))
		return c.JSON(fiber.Map{"secret": secret, "otpauth": otpauth})
	})
	app.Post("/2fa/totp/activate", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct {
			Code string `json:"code"`
		}
		if err := c.BodyParser(&body); err != nil || body.Code == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		key := fmt.Sprintf("totp-setup:%d", u.ID)
		secret, _ := RDB.Get(c.Context(), key).Result()
		if secret == "" {
			return c.Status(400).JSON(fiber.Map{"error": "setup expired"})
		}
		if !validateTOTP(secret, body.Code) {
			return c.Status(401).JSON(fiber.Map{"error": "invalid code"})
		}
		enc, err := encryptString(secret)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "server"})
		}
		// upsert
		var ts TOTPSecret
		if err := DB.Where("user_id = ?", u.ID).First(&ts).Error; err == nil {
			ts.Secret = enc
			ts.IsActive = true
			_ = DB.Save(&ts).Error
		} else {
			_ = DB.Create(&TOTPSecret{UserID: u.ID, Secret: enc, IsActive: true}).Error
		}
		_ = RDB.Del(c.Context(), key).Err()
		return c.SendStatus(204)
	})
	app.Post("/2fa/totp/disable", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var ts TOTPSecret
		if err := DB.Where("user_id = ?", u.ID).First(&ts).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not enabled"})
		}
		ts.IsActive = false
		if err := DB.Save(&ts).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// Recovery codes
	app.Post("/2fa/recovery/generate", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		// delete existing unused codes
		_ = DB.Where("user_id = ? AND is_used = 0", u.ID).Delete(&RecoveryCode{}).Error
		// generate 10 codes
		codes := make([]string, 0, 10)
		for i := 0; i < 10; i++ {
			plain := generateRecoveryCode()
			hashed := sha256Hex([]byte(plain))
			_ = DB.Create(&RecoveryCode{UserID: u.ID, HashedCode: hashed}).Error
			codes = append(codes, plain)
		}
		return c.JSON(fiber.Map{"codes": codes})
	})
	app.Post("/2fa/recovery/consume", func(c *fiber.Ctx) error {
		var body struct {
			Email string `json:"email"`
			Code  string `json:"code"`
		}
		if err := c.BodyParser(&body); err != nil || body.Email == "" || body.Code == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var u User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		if consumeRecoveryCode(u.ID, body.Code) {
			return c.SendStatus(204)
		}
		return c.Status(401).JSON(fiber.Map{"error": "invalid code"})
	})

	// Tasks CRUD with permissions
	app.Get("/tasks", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		// Permission: read tasks (default allow)
		if ok, err := checkPermission(u.ID, "tasks", "read", nil); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		} else if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}

		overdue := c.Query("overdue") == "true"
		qstr := strings.TrimSpace(c.Query("q"))
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		if limit <= 0 || limit > 100 {
			limit = 20
		}
		// Cursor based on deadline and id to ensure stable ordering
		cursorDeadlineStr := c.Query("cursor_deadline")
		cursorIDStr := c.Query("cursor_id")
		var cursorDeadline *time.Time
		var cursorID uint
		if cursorDeadlineStr != "" {
			if t, err := time.Parse(time.RFC3339, cursorDeadlineStr); err == nil {
				cursorDeadline = &t
			}
		}
		if cursorIDStr != "" {
			if v, err := strconv.Atoi(cursorIDStr); err == nil && v > 0 {
				cursorID = uint(v)
			}
		}

		// Build query
		dbq := DB.Model(&Task{})
		now := time.Now()
		if overdue {
			dbq = dbq.Where("deadline IS NOT NULL AND deadline < ?", now)
		} else {
			dbq = dbq.Where("deadline IS NULL OR deadline >= ?", now)
		}
		if qstr != "" {
			like := "%" + qstr + "%"
			dbq = dbq.Where("title LIKE ? OR description LIKE ?", like, like)
		}
		// Apply cursor for pagination
		// For non-overdue list (ascending by deadline NULLS LAST then id), we page after (deadline, id)
		// For overdue (descending), we page before (deadline, id)
		if cursorDeadline != nil {
			if overdue {
				dbq = dbq.Where("(deadline < ?) OR (deadline = ? AND id < ?)", cursorDeadline, cursorDeadline, cursorID)
			} else {
				dbq = dbq.Where("(deadline > ?) OR (deadline = ? AND id > ?)", cursorDeadline, cursorDeadline, cursorID)
			}
		} else if cursorID != 0 {
			// Fallback if only id is present
			if overdue {
				dbq = dbq.Where("id < ?", cursorID)
			} else {
				dbq = dbq.Where("id > ?", cursorID)
			}
		}
		// Ordering
		if overdue {
			dbq = dbq.Order("deadline DESC").Order("id DESC")
		} else {
			// deadline ASC with NULLS LAST: emulate by ordering by (deadline IS NULL) then deadline ASC
			dbq = dbq.Order("CASE WHEN deadline IS NULL THEN 1 ELSE 0 END ASC").Order("deadline ASC").Order("id ASC")
		}
		var items []Task
		if err := dbq.Limit(limit + 1).Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		var nextCursor map[string]string
		if len(items) > limit {
			last := items[limit-1]
			items = items[:limit]
			if last.Deadline != nil {
				nextCursor = map[string]string{
					"cursor_deadline": last.Deadline.Format(time.RFC3339),
					"cursor_id":       fmt.Sprintf("%d", last.ID),
				}
			} else {
				nextCursor = map[string]string{"cursor_id": fmt.Sprintf("%d", last.ID)}
			}
		}
		return c.JSON(fiber.Map{"items": items, "next_cursor": nextCursor})
	})
	app.Post("/tasks", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		if ok, err := checkPermission(u.ID, "tasks", "create", nil); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		} else if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		var body struct {
			Title           string  `json:"title"`
			SubjectID       uint    `json:"subject_id"`
			Description     *string `json:"description"`
			IsRequireHandIn *bool   `json:"is_require_hand_in"`
			Deadline        *string `json:"deadline"`
		}
		if err := c.BodyParser(&body); err != nil || body.Title == "" || body.SubjectID == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var deadlinePtr *time.Time
		if body.Deadline != nil && *body.Deadline != "" {
			if t, err := time.Parse(time.RFC3339, *body.Deadline); err == nil {
				deadlinePtr = &t
			} else {
				return c.Status(400).JSON(fiber.Map{"error": "invalid deadline"})
			}
		}
		rec := Task{AddedUserID: u.ID, Title: body.Title, SubjectID: body.SubjectID, Description: body.Description, Deadline: deadlinePtr}
		if body.IsRequireHandIn != nil {
			rec.IsRequireHandIn = *body.IsRequireHandIn
		}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(201).JSON(rec)
	})
	app.Patch("/tasks/:id", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var rec Task
		if err := DB.First(&rec, c.Params("id")).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		var body struct {
			Title           *string `json:"title"`
			SubjectID       *uint   `json:"subject_id"`
			Description     *string `json:"description"`
			IsRequireHandIn *bool   `json:"is_require_hand_in"`
			Deadline        *string `json:"deadline"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if body.Title != nil {
			rec.Title = *body.Title
		}
		if body.SubjectID != nil {
			rec.SubjectID = *body.SubjectID
		}
		if body.Description != nil {
			rec.Description = body.Description
		}
		if body.IsRequireHandIn != nil {
			rec.IsRequireHandIn = *body.IsRequireHandIn
		}
		if body.Deadline != nil {
			if *body.Deadline == "" {
				rec.Deadline = nil
			} else if t, err := time.Parse(time.RFC3339, *body.Deadline); err == nil {
				rec.Deadline = &t
			} else {
				return c.Status(400).JSON(fiber.Map{"error": "invalid deadline"})
			}
		}
		// enforce permission: update own or explicit
		if ok, err := checkPermission(u.ID, "tasks", "update", &rec.AddedUserID); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		} else if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		if err := DB.Save(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(rec)
	})
	app.Delete("/tasks/:id", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		if _, err := strconv.Atoi(c.Params("id")); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
		}
		var rec Task
		if err := DB.First(&rec, c.Params("id")).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		// Admin/manager fallback, otherwise require delete permission
		isAdmin, _ := userHasRole(u.ID, "admin")
		isManager, _ := userHasRole(u.ID, "manager")
		allowed := isAdmin || isManager
		if !allowed {
			if ok, err := checkPermission(u.ID, "tasks", "delete", &rec.AddedUserID); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "perm"})
			} else {
				allowed = ok
			}
		}
		if !allowed {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		if err := DB.Delete(&Task{}, rec.ID).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	app.Post("/integrations/teams/status", func(c *fiber.Ctx) error {
		// verify HMAC signature
		body := c.Body()
		sig := c.Get("X-Signature")
		secret := env("TEAMS_STATUS_SIGNING_SECRET", "")
		if secret != "" {
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(body)
			expected := fmt.Sprintf("sha256=%x", mac.Sum(nil))
			if sig == "" || !hmac.Equal([]byte(sig), []byte(expected)) {
				return c.Status(401).JSON(fiber.Map{"error": "invalid signature"})
			}
		}
		// store minimal log
		type payload struct {
			IsSuccess      bool   `json:"is_success"`
			TaskID         uint   `json:"task_id"`
			MessageID      string `json:"message_id"`
			ConversationID string `json:"conversation_id"`
		}
		var p payload
		_ = c.BodyParser(&p)
		if p.TaskID != 0 {
			_ = DB.Table("tasks_teams_logs").Create(map[string]any{
				"task_id":    p.TaskID,
				"is_success": p.IsSuccess,
				"teams_message_id": func() *string {
					if p.MessageID == "" {
						return nil
					}
					v := p.MessageID
					return &v
				}(),
				"teams_conversation_id": func() *string {
					if p.ConversationID == "" {
						return nil
					}
					v := p.ConversationID
					return &v
				}(),
				"last_status": "callback_received",
			}).Error
		}
		return c.SendStatus(fiber.StatusOK)
	})

	// Teams inbound + async processing
	// Simple buffered channel acts as a queue for demo purposes
	var teamsQueue = make(chan map[string]any, 100)
	// Worker goroutine
	go func() {
		for job := range teamsQueue {
			// Extract data
			title, _ := job["title"].(string)
			desc, _ := job["description"].(*string)
			addedUserID, _ := job["added_user_id"].(uint)
			subjectID, _ := job["subject_id"].(uint)
			callback, _ := job["status_callback"].(*string)
			// Create task
			rec := Task{AddedUserID: addedUserID, Title: title, SubjectID: subjectID, Description: desc}
			var isSuccess bool
			if err := DB.Create(&rec).Error; err != nil {
				log.Printf("teams worker: create task error: %v", err)
				isSuccess = false
			} else {
				isSuccess = true
			}
			// Log into tasks_teams_logs (initial)
			_ = DB.Table("tasks_teams_logs").Create(map[string]any{
				"task_id":    rec.ID,
				"is_success": isSuccess,
				"callback_url": func() *string {
					if callback == nil {
						return nil
					}
					return callback
				}(),
				"attempts": 0,
				"last_status": func() string {
					if isSuccess {
						return "created"
					} else {
						return "create_failed"
					}
				}(),
			}).Error
			// Callback with retries if provided
			if callback != nil && *callback != "" {
				cbURL := *callback
				payload := fmt.Sprintf(`{"is_success":%t,"task_id":%d}`, isSuccess, rec.ID)
				secret := env("TEAMS_STATUS_SIGNING_SECRET", "")
				// HMAC signature
				var sig string
				if secret != "" {
					mac := hmac.New(sha256.New, []byte(secret))
					mac.Write([]byte(payload))
					sig = fmt.Sprintf("sha256=%x", mac.Sum(nil))
				}
				client := &http.Client{Timeout: 10 * time.Second}
				attempts := 0
				for attempts < 3 {
					attempts++
					req, _ := http.NewRequest("POST", cbURL, strings.NewReader(payload))
					req.Header.Set("Content-Type", "application/json")
					if sig != "" {
						req.Header.Set("X-Signature", sig)
					}
					resp, err := client.Do(req)
					if err != nil {
						_ = DB.Table("tasks_teams_logs").Where("task_id = ?", rec.ID).Updates(map[string]any{
							"attempts":    attempts,
							"last_status": "callback_error",
							"last_error":  err.Error(),
						}).Error
						time.Sleep(time.Duration(attempts) * time.Second)
						continue
					}
					bodyBytes := make([]byte, 0)
					if resp.Body != nil {
						bodyBytes, _ = io.ReadAll(resp.Body)
						_ = resp.Body.Close()
					}
					_ = DB.Table("tasks_teams_logs").Where("task_id = ?", rec.ID).Updates(map[string]any{
						"attempts":             attempts,
						"last_status":          "callback_sent",
						"last_response_status": resp.StatusCode,
						"last_response_body":   string(bodyBytes),
					}).Error
					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						break
					}
					time.Sleep(time.Duration(attempts) * time.Second)
				}
			}
		}
	}()

	app.Post("/integrations/teams/inbound", func(c *fiber.Ctx) error {
		// Verify shared secret header
		secret := c.Get("X-Teams-Secret")
		if secret == "" || secret != env("TEAMS_CALLBACK_ENDPOINT_SECRET", "change_me") {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		var body struct {
			Email          string  `json:"email"`
			Title          string  `json:"title"`
			Description    *string `json:"description"`
			SubjectID      uint    `json:"subject_id"`
			StatusCallback *string `json:"status_callback"`
		}
		if err := c.BodyParser(&body); err != nil || body.Email == "" || body.Title == "" || body.SubjectID == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		// Resolve user by email
		var u User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "user not found"})
		}
		// Enqueue job
		job := map[string]any{
			"title":         body.Title,
			"description":   body.Description,
			"added_user_id": u.ID,
			"subject_id":    body.SubjectID,
		}
		if body.StatusCallback != nil {
			job["status_callback"] = body.StatusCallback
		}
		select {
		case teamsQueue <- job:
			return c.SendStatus(202)
		default:
			return c.Status(503).JSON(fiber.Map{"error": "queue full"})
		}
	})

	// Inbound endpoint to mark day shift unfulfilled when an email about 日直 is received via Power Automate
	app.Post("/integrations/dayshift/inbound", func(c *fiber.Ctx) error {
		secret := c.Get("X-Taskly-Secret")
		if secret == "" || secret != env("DAYSHIFT_INBOUND_SECRET", "change_me") {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		var body struct {
			Title   string  `json:"title"`
			Content string  `json:"content"`
			Date    *string `json:"date"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if !strings.Contains(body.Title+" "+body.Content, "日直") {
			return c.Status(400).JSON(fiber.Map{"error": "not a dayshift notice"})
		}
		var target time.Time
		if body.Date != nil && *body.Date != "" {
			if t, err := time.Parse("2006-01-02", *body.Date); err == nil {
				target = t
			} else {
				target = dateOnly(time.Now())
			}
		} else {
			target = dateOnly(time.Now())
		}
		var asg DayShiftAssignment
		if err := DB.Where("date = ?", target.Format("2006-01-02")).First(&asg).Error; err != nil {
			// create if missing within current cycle if any
			cyc, _ := getCurrentCycle(target)
			asg = DayShiftAssignment{Date: dateOnly(target)}
			if cyc != nil {
				asg.CycleID = cyc.ID
			}
			asg.Status = "unfulfilled"
			if err := DB.Create(&asg).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
		} else {
			asg.Status = "unfulfilled"
			_ = DB.Save(&asg).Error
		}
		return c.SendStatus(202)
	})

	app.Get("/ical/:token.ics", func(c *fiber.Ctx) error {
		token := c.Params("token")
		var key IcalKey
		if err := DB.Where("token = ?", token).First(&key).Error; err != nil {
			return c.Status(404).SendString("not found")
		}
		ics := buildICSForUser(key.UserID)
		c.Set("Content-Type", "text/calendar; charset=utf-8")
		return c.SendString(ics)
	})

	port := env("API_PORT", "8080")
	log.Printf("Application Server listening on :%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}
