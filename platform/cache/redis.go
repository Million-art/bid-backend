package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bid-hub/admin-backend/platform/env"
	"github.com/redis/go-redis/v9"
)

func InitRedis(cfg *env.Config) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       0, // use default DB
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		slog.Warn("redis connection failed; caching disabled with DB fallback", "error", err)
		return nil
	}

	slog.Info("connected to Redis")
	return client
}
