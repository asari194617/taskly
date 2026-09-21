package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTelemetryMiddlewareRecordsSafeRouteAndParent(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
		_ = provider.Shutdown(t.Context())
	})

	app := fiber.New()
	app.Use(telemetryMiddleware())
	app.Get("/tasks/:id", func(c *fiber.Ctx) error { return c.SendStatus(204) })
	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest("GET", "/tasks/secret-id?token=secret-token", nil)
	req.Header.Set("Authorization", "Bearer secret-auth")
	req.Header.Set("traceparent", "00-11111111111111111111111111111111-2222222222222222-01")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	health, err := app.Test(httptest.NewRequest("GET", "/healthz", nil))
	if err != nil {
		t.Fatal(err)
	}
	_ = health.Body.Close()

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	span := spans[0]
	if span.Name() != "GET /tasks/:id" {
		t.Fatalf("span name = %q", span.Name())
	}
	if span.Parent().SpanID().String() != "2222222222222222" {
		t.Fatalf("parent span = %s", span.Parent().SpanID())
	}
	for _, attribute := range span.Attributes() {
		value := attribute.Value.Emit()
		if strings.Contains(value, "secret-id") || strings.Contains(value, "secret-token") || strings.Contains(value, "secret-auth") {
			t.Fatalf("sensitive attribute: %s", attribute)
		}
	}
}

func TestTelemetryRejectsInvalidSampleRate(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	t.Setenv("OTEL_TRACES_SAMPLE_RATE", "1.5")
	if _, err := initTelemetry(t.Context(), "test", "test", ""); err == nil {
		t.Fatal("expected invalid sample rate error")
	}
}
