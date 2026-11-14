package server

import (
	"context"
	"errors"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redisServer "github.com/alicebob/miniredis/v2/server"
	"github.com/redis/go-redis/v9"
)

func TestDatadogAgentStartOptionAcceptsUnixSocket(t *testing.T) {
	path, ok := datadogUnixSocketPath("unix:///var/run/datadog/apm.socket")
	if !ok || path != "/var/run/datadog/apm.socket" {
		t.Fatalf("unexpected Datadog socket path: path=%q ok=%t", path, ok)
	}
}

func TestDatadogAgentStartOptionFallsBackToTCP(t *testing.T) {
	t.Setenv("DD_TRACE_AGENT_URL", "")
	t.Setenv("DD_AGENT_HOST", "agent.internal")
	t.Setenv("DD_TRACE_AGENT_PORT", "18126")

	if option := datadogAgentStartOption(); option == nil {
		t.Fatal("expected a Datadog TCP start option")
	}
}

func TestDatadogAgentStartOptionDoesNotLeakInvalidURL(t *testing.T) {
	const secret = "must-not-appear"
	t.Setenv("DD_TRACE_AGENT_URL", "://"+secret)

	previous := log.Writer()
	var output strings.Builder
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })

	if option := datadogAgentStartOption(); option == nil {
		t.Fatal("expected fallback Datadog start option")
	}
	if strings.Contains(output.String(), secret) {
		t.Fatal("Datadog configuration warning leaked the invalid URL")
	}
}

func TestDatadogUnixSocketPathRejectsNonUnixURL(t *testing.T) {
	if path, ok := datadogUnixSocketPath("http://agent.internal:8126"); ok || path != "" {
		t.Fatalf("expected non-Unix URL to be rejected: path=%q ok=%t", path, ok)
	}
}

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
	if !options.ContextTimeoutEnabled {
		t.Fatal("Redis socket operations must retain request deadlines")
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

func TestRedisSocketReadKeepsRequestDeadline(t *testing.T) {
	server := miniredis.RunT(t)
	server.RequireUserAuth("taskly", "test-secret")
	t.Setenv("REDIS_ADDR", server.Addr())
	t.Setenv("REDIS_USERNAME", "taskly")
	t.Setenv("REDIS_PASSWORD", "test-secret")
	t.Setenv("REDIS_DB", "0")
	options, err := redisOptionsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	options.MaxRetries = -1
	rdb := redis.NewClient(options)
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 1)
	server.Server().SetPreHook(func(_ *redisServer.Peer, command string, _ ...string) bool {
		if strings.EqualFold(command, "get") {
			started <- struct{}{}
			return true // Leave the socket read pending without a Redis reply.
		}
		return false
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	before := time.Now()
	err = rdb.Get(ctx, "blocked").Err()
	const budget = 500 * time.Millisecond
	elapsed := time.Since(before)
	var socketError net.Error
	if err == nil || (!errors.Is(err, context.DeadlineExceeded) && !(errors.As(err, &socketError) && socketError.Timeout())) || elapsed >= budget {
		t.Fatalf("Redis read did not obey request deadline: err=%v elapsed=%s", err, elapsed)
	}
	// The socket deadline and context cancellation share an absolute deadline,
	// but their timers can wake different goroutines in either order. Observe
	// cancellation within the original total budget rather than assuming order.
	remaining := time.NewTimer(time.Until(before.Add(budget)))
	defer remaining.Stop()
	select {
	case <-ctx.Done():
	case <-remaining.C:
		t.Fatal("request context did not expire within the original deadline budget")
	}
	if ctx.Err() != context.DeadlineExceeded || time.Since(before) > budget {
		t.Fatalf("request deadline was not observed: context=%v elapsed=%s", ctx.Err(), time.Since(before))
	}
	select {
	case <-started:
	default:
		t.Fatal("Redis command did not reach the blocked socket")
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
