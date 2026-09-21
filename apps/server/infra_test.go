package server

import (
	"strings"
	"testing"
)

func TestRedisOptionsFromEnv(t *testing.T) {
	t.Setenv("REDIS_ADDR", "redis.internal:6379")
	t.Setenv("REDIS_USERNAME", "taskly")
	t.Setenv("REDIS_PASSWORD", "test-secret")
	t.Setenv("REDIS_DB", "2")

	options, err := redisOptionsFromEnv()
	if err != nil {
		t.Fatalf("redisOptionsFromEnv: %v", err)
	}
	if options.Addr != "redis.internal:6379" || options.Username != "taskly" || options.Password != "test-secret" || options.DB != 2 {
		t.Fatalf("unexpected Redis options: addr=%q username=%q db=%d", options.Addr, options.Username, options.DB)
	}
	if options.Protocol != 2 || !options.DisableIdentity {
		t.Fatalf("unexpected Redis protocol options: protocol=%d disable_identity=%t", options.Protocol, options.DisableIdentity)
	}
}

func TestRedisOptionsFromEnvRejectsMissingCredentials(t *testing.T) {
	t.Setenv("REDIS_USERNAME", "")
	t.Setenv("REDIS_PASSWORD", "")

	_, err := redisOptionsFromEnv()
	if err == nil || !strings.Contains(err.Error(), "REDIS_USERNAME") {
		t.Fatalf("expected missing username error, got %v", err)
	}

	t.Setenv("REDIS_USERNAME", "taskly")
	_, err = redisOptionsFromEnv()
	if err == nil || !strings.Contains(err.Error(), "REDIS_PASSWORD") {
		t.Fatalf("expected missing password error, got %v", err)
	}
}

func TestRedisOptionsFromEnvRejectsInvalidDBWithoutLeakingPassword(t *testing.T) {
	const secret = "must-not-appear"
	t.Setenv("REDIS_USERNAME", "taskly")
	t.Setenv("REDIS_PASSWORD", secret)
	t.Setenv("REDIS_DB", "invalid")

	_, err := redisOptionsFromEnv()
	if err == nil || !strings.Contains(err.Error(), "REDIS_DB") {
		t.Fatalf("expected invalid DB error, got %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("Redis configuration error leaked the password")
	}
}
