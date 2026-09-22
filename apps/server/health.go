package server

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const readinessTimeout = 2 * time.Second

func registerHealthRoutes(app fiber.Router, db *gorm.DB, rdb *redis.Client) {
	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/readyz", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), readinessTimeout)
		defer cancel()

		sqlDB, err := db.DB()
		if err != nil {
			return c.Status(fiber.StatusServiceUnavailable).SendString("unavailable")
		}
		// Check both dependencies concurrently so a single outage cannot double
		// the probe's maximum latency.
		dbResult := make(chan error, 1)
		go func() { dbResult <- sqlDB.PingContext(ctx) }()
		redisResult := make(chan error, 1)
		go func() { redisResult <- rdb.Ping(ctx).Err() }()
		if <-dbResult != nil || <-redisResult != nil {
			return c.Status(fiber.StatusServiceUnavailable).SendString("unavailable")
		}
		return c.SendString("ok")
	})
}
