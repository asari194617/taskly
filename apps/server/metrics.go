package server

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
)

var httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "taskly_http_requests_total", Help: "API requests by method, route and response status.",
}, []string{"method", "route", "status"})

var httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name: "taskly_http_request_duration_seconds", Help: "API request duration by method, route and response status.",
	Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
}, []string{"method", "route", "status"})

func init() {
	prometheus.MustRegister(httpRequests, httpDuration)
}

func metricsMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		status := c.Response().StatusCode()
		if err != nil {
			if fiberErr, ok := err.(*fiber.Error); ok {
				status = fiberErr.Code
			} else {
				status = fiber.StatusInternalServerError
			}
		}
		route := "unmatched"
		if matched := c.Route(); matched != nil && matched.Path != "" && status != fiber.StatusNotFound {
			route = matched.Path
		}
		method := c.Method()
		if method != fiber.MethodGet && method != fiber.MethodPost && method != fiber.MethodPut && method != fiber.MethodPatch && method != fiber.MethodDelete && method != fiber.MethodOptions && method != fiber.MethodHead {
			method = "OTHER"
		}
		code := strconv.Itoa(status)
		httpRequests.WithLabelValues(method, route, code).Inc()
		httpDuration.WithLabelValues(method, route, code).Observe(time.Since(start).Seconds())
		return err
	}
}
