package server

import (
	"net"
	"net/http/httptest"
	"testing"
	"unsafe"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricMethodDoesNotRetainRequestBuffer(t *testing.T) {
	for _, want := range []string{fiber.MethodGet, fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete, fiber.MethodOptions, fiber.MethodHead} {
		buffer := []byte(want)
		borrowed := unsafe.String(unsafe.SliceData(buffer), len(buffer))
		label := metricMethod(borrowed)
		buffer[0] = 'X'
		if label != want {
			t.Fatalf("method label changed after request buffer reuse: got %q, want %q", label, want)
		}
	}
	if got := metricMethod("CUSTOM"); got != "OTHER" {
		t.Fatalf("unknown method label = %q, want OTHER", got)
	}
}

func TestMetricsMiddlewareUsesRoutePattern(t *testing.T) {
	app := fiber.New()
	app.Use(metricsMiddleware())
	app.Get("/metrics-test/:id", func(c *fiber.Ctx) error { return c.SendStatus(500) })
	if _, err := app.Test(httptest.NewRequest("GET", "/metrics-test/private-token", nil)); err != nil {
		t.Fatal(err)
	}
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range metrics {
		if family.GetName() != "taskly_http_requests_total" {
			continue
		}
		for _, sample := range family.GetMetric() {
			labels := map[string]string{}
			for _, label := range sample.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["route"] == "/metrics-test/private-token" {
				t.Fatal("raw URL became a metric label")
			}
			if labels["route"] == "/metrics-test/:id" && labels["method"] == "GET" && labels["status"] == "500" {
				return
			}
		}
	}
	t.Fatal("expected route pattern and status metric")
}

func TestMetricsServerReturnsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := metricsServer(listener.Addr().String()); err == nil {
		t.Fatal("expected occupied port to fail")
	}
}
