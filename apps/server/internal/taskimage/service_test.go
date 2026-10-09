package taskimage

import (
	"context"
	"strings"
	"testing"
)

func TestValidKey(t *testing.T) {
	valid := "task-images/42/550e8400-e29b-41d4-a716-446655440000"
	if !ValidKey(valid) || !OwnedKey(42, valid) {
		t.Fatalf("expected valid owned key")
	}
	for _, key := range []string{
		"task-images/42/1",
		"task-images/42/../550e8400-e29b-41d4-a716-446655440000",
		"task-images/43/550e8400-e29b-41d4-a716-446655440000",
	} {
		if OwnedKey(42, key) {
			t.Fatalf("unexpected owned key: %s", key)
		}
	}
}

func TestAllowedContentType(t *testing.T) {
	for _, contentType := range []string{"image/jpeg", "image/png", "image/webp", "image/gif"} {
		if !AllowedContentType(contentType) {
			t.Fatalf("expected %s to be allowed", contentType)
		}
	}
	for _, contentType := range []string{"image/svg+xml", "text/html", "application/octet-stream"} {
		if AllowedContentType(contentType) {
			t.Fatalf("expected %s to be rejected", contentType)
		}
	}
}

func TestPresignUploadUsesUUIDKeyAndRequiredHeaders(t *testing.T) {
	t.Setenv("R2_ENDPOINT", "https://account.r2.cloudflarestorage.com")
	t.Setenv("R2_BUCKET", "taskly-test")
	t.Setenv("R2_ACCESS_KEY_ID", "access")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret")
	service, err := NewFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	upload, err := service.PresignUpload(context.Background(), 42, "image/png", 123)
	if err != nil {
		t.Fatal(err)
	}
	if !OwnedKey(42, upload.Key) || !strings.Contains(upload.URL, "X-Amz-Signature=") {
		t.Fatalf("unexpected signed upload: %+v", upload)
	}
	if upload.Headers["Content-Type"] != "image/png" || upload.Headers["X-Amz-Meta-Owner-Id"] != "42" {
		t.Fatalf("missing signed headers: %+v", upload.Headers)
	}
	if len(upload.Headers) != 2 {
		t.Fatalf("unexpected browser upload headers: %+v", upload.Headers)
	}
}
