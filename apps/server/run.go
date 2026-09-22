package server

import (
	"context"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/jobs"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/getsentry/sentry-go"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	ddProfiler "gopkg.in/DataDog/dd-trace-go.v1/profiler"
)

func Run() {
	// Fail fast before any expensive startup work.
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("FATAL: JWT_SECRET must be set in non-development environments.")
	}

	// Start Datadog tracer (reads most settings from env; select the transport explicitly below).
	serviceName := utils.Env("DD_SERVICE", "taskly-server")
	envName := utils.Env("DD_ENV", utils.Env("SENTRY_ENVIRONMENT", utils.Env("NODE_ENV", "development")))
	version := utils.Env("DD_VERSION", utils.Env("SENTRY_RELEASE", ""))
	shutdown, err := initTelemetry(context.Background(), serviceName, envName, version)
	if err != nil {
		log.Fatalf("OpenTelemetry initialization: %v", err)
	}
	defer shutdownTelemetry(shutdown)
	// Configure Datadog tracer options (including optional sampling rate)
	tOpts := []tracer.StartOption{
		tracer.WithService(serviceName),
		tracer.WithEnv(envName),
		tracer.WithServiceVersion(version),
		datadogAgentStartOption(),
		tracer.WithRuntimeMetrics(),
	}
	if sr := strings.TrimSpace(utils.Env("DD_TRACE_SAMPLE_RATE", "")); sr != "" {
		if v, err := strconv.ParseFloat(sr, 64); err == nil && v >= 0 && v <= 1 {
			tOpts = append(tOpts, tracer.WithSampler(tracer.NewRateSampler(v)))
		}
	}
	tracer.Start(tOpts...)
	defer tracer.Stop()

	if mustInitSentry() {
		defer sentry.Flush(2 * time.Second)
	}

	// Optional: Start Datadog profiler when enabled
	if strings.ToLower(utils.Env("DD_PROFILING_ENABLED", "false")) == "true" {
		_ = ddProfiler.Start(
			ddProfiler.WithService(serviceName),
			ddProfiler.WithEnv(envName),
			ddProfiler.WithVersion(version),
		)
		defer ddProfiler.Stop()
	}

	db := mustInitDB() // ローカル変数にする
	if err := jobs.EnsureSchema(db); err != nil {
		log.Fatalf("integration jobs schema: %v", err)
	}
	jobs.RegisterQueueMetrics(db)
	registerOperationsMetrics(db)
	rdb := mustInitRedis()
	wAuth := initWebAuthn()
	if err := metricsServer(utils.Env("METRICS_BIND_ADDR", "127.0.0.1:9464")); err != nil {
		log.Fatal(err)
	}
	app := newServerApp(db, rdb, wAuth, serviceName)

	port := utils.Env("API_PORT", "8080")
	log.Printf("Application Server listening on :%s", port)
	stopContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-stopContext.Done()
		if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
			log.Printf("server shutdown: %v", err)
		}
	}()
	if err := app.Listen(":" + port); err != nil {
		panic(err) // Run's deferred tracer shutdowns must execute before exit.
	}
}

func datadogAgentStartOption() tracer.StartOption {
	if rawURL := strings.TrimSpace(os.Getenv("DD_TRACE_AGENT_URL")); rawURL != "" {
		if socketPath, ok := datadogUnixSocketPath(rawURL); ok {
			return tracer.WithUDS(socketPath)
		}
		log.Printf("WARN: DD_TRACE_AGENT_URL must be a valid unix:// URL; falling back to DD_AGENT_HOST")
	}

	agentHost := utils.Env("DD_AGENT_HOST", "127.0.0.1")
	agentPort := utils.Env("DD_TRACE_AGENT_PORT", "8126")
	return tracer.WithAgentAddr(agentHost + ":" + agentPort)
}

func datadogUnixSocketPath(rawURL string) (string, bool) {
	agentURL, err := url.Parse(rawURL)
	if err != nil || agentURL.Scheme != "unix" || agentURL.Host != "" || agentURL.Path == "" {
		return "", false
	}
	return agentURL.Path, true
}
