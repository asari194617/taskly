package handler

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/middleware"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type failedSecretReader struct{ successfulReads int }

func (r *failedSecretReader) Read(p []byte) (int, error) {
	if r.successfulReads == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	for i := range p {
		p[i] = byte(i)
	}
	r.successfulReads--
	return len(p), nil
}

func TestSecretIssuanceRoutesFailOnRandomError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.IcalKey{}, &domain.RecoveryCode{}); err != nil {
		t.Fatal(err)
	}
	u := domain.User{ID: 9, Email: "test@example.com"}
	h := NewHandler(db, nil, nil)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", &u); return c.Next() })
	app.Post("/session", func(c *fiber.Ctx) error { return h.issueSession(c, u.ID, "web", time.Now()) })
	h.RegisterIcalTokensRoutes(app.Group("/ical-tokens"))
	h.Register2FARoutes(app.Group("/2fa"), middleware.NewMiddleware(db, nil, nil))

	original := rand.Reader
	rand.Reader = &failedSecretReader{}
	t.Cleanup(func() { rand.Reader = original })
	for _, path := range []string{"/session", "/ical-tokens/", "/2fa/totp/setup", "/2fa/recovery/generate"} {
		resp, err := app.Test(httptest.NewRequest("POST", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != 500 {
			t.Errorf("%s: status = %d, want 500", path, resp.StatusCode)
		}
	}
	var icalCount, recoveryCount int64
	if err := db.Model(&domain.IcalKey{}).Count(&icalCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&domain.RecoveryCode{}).Count(&recoveryCount).Error; err != nil {
		t.Fatal(err)
	}
	if icalCount != 0 || recoveryCount != 0 {
		t.Fatalf("secret rows persisted: ical=%d recovery=%d", icalCount, recoveryCount)
	}
}

func TestRecoveryRandomFailureRollsBackExistingCodes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.RecoveryCode{}); err != nil {
		t.Fatal(err)
	}
	u := domain.User{ID: 9}
	if err := db.Create(&domain.RecoveryCode{UserID: u.ID, HashedCode: "old"}).Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", &u); return c.Next() })
	NewHandler(db, nil, nil).Register2FARoutes(app.Group("/2fa"), middleware.NewMiddleware(db, nil, nil))
	original := rand.Reader
	rand.Reader = &failedSecretReader{successfulReads: 1}
	t.Cleanup(func() { rand.Reader = original })
	resp, err := app.Test(httptest.NewRequest("POST", "/2fa/recovery/generate", nil))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	var codes []domain.RecoveryCode
	if err := db.Where("user_id = ?", u.ID).Find(&codes).Error; err != nil {
		t.Fatal(err)
	}
	if len(codes) != 1 || codes[0].HashedCode != "old" {
		t.Fatalf("codes changed: %+v", codes)
	}
}
