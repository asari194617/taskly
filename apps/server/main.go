package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/getsentry/sentry-go"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	ddProfiler "gopkg.in/DataDog/dd-trace-go.v1/profiler"
)

func main() {
	// Fail fast before any expensive startup work.
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("FATAL: JWT_SECRET must be set in non-development environments.")
	}

	// Start Datadog tracer (reads most settings from env; override agent addr explicitly if provided)
	serviceName := utils.Env("DD_SERVICE", "taskly-server")
	envName := utils.Env("DD_ENV", utils.Env("SENTRY_ENVIRONMENT", utils.Env("NODE_ENV", "development")))
	version := utils.Env("DD_VERSION", utils.Env("SENTRY_RELEASE", ""))
	agentHost := utils.Env("DD_AGENT_HOST", "127.0.0.1")
	agentPort := utils.Env("DD_TRACE_AGENT_PORT", "8126")
	// Configure Datadog tracer options (including optional sampling rate)
	tOpts := []tracer.StartOption{
		tracer.WithService(serviceName),
		tracer.WithEnv(envName),
		tracer.WithServiceVersion(version),
		tracer.WithAgentAddr(agentHost + ":" + agentPort),
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
	rdb := mustInitRedis()
	wAuth := initWebAuthn()
	metricsServer(utils.Env("METRICS_BIND_ADDR", "127.0.0.1:9464"))
	app := newServerApp(db, rdb, wAuth, serviceName)

	port := utils.Env("API_PORT", "8080")
	log.Printf("Application Server listening on :%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}
