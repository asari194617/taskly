package domain

import "testing"

func TestWebPushVAPIDTableName(t *testing.T) {
	if got, want := (WebPushVAPID{}).TableName(), "web_push_vapid"; got != want {
		t.Fatalf("WebPushVAPID.TableName() = %q, want %q", got, want)
	}
}
