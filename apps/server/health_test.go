package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

func TestReadinessTracksDependenciesAfterStartup(t *testing.T) {
	db := newTestIcalDB(t)
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr(), MaxRetries: 0})
	defer rdb.Close()
	app := fiber.New()
	registerHealthRoutes(app, db, rdb)

	check := func(path string, want int) {
		t.Helper()
		response, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil), -1)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("%s: status %d, want %d", path, response.StatusCode, want)
		}
	}

	check("/readyz", http.StatusOK)
	redisServer.Close()
	check("/healthz", http.StatusOK)
	check("/readyz", http.StatusServiceUnavailable)
	if err := redisServer.Restart(); err != nil {
		t.Fatal(err)
	}
	check("/readyz", http.StatusOK)

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	check("/healthz", http.StatusOK)
	check("/readyz", http.StatusServiceUnavailable)
}
