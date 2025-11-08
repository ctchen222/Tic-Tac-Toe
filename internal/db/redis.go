package db

import (
	"context"
	"os"

	"github.com/go-redis/redis/v8"
)

// NewRedisClient creates and returns a new Redis client.
// It connects to the Redis server at the address specified by the
// REDIS_ADDR environment variable, or "localhost:6379" if not set.
func NewRedisClient(ctx context.Context) (*redis.Client, error) {
	redisAddr := os.Getenv("REDIS_CONNSTRING")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Ping the server to ensure the connection is established.
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}
