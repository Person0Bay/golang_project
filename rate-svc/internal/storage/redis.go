package storage

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	Client *redis.Client
	TTL    time.Duration
}

func NewRedisCache(client *redis.Client, ttl time.Duration) *RedisCache {
	return &RedisCache{Client: client, TTL: ttl}
}

func (c *RedisCache) ReviewMarkerKey(dishID, orderID int) string {
	return "review:" + strconv.Itoa(dishID) + ":" + strconv.Itoa(orderID)
}

func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	res, err := c.Client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return res > 0, nil
}

func (c *RedisCache) SetMarker(ctx context.Context, key string) error {
	return c.Client.Set(ctx, key, "1", c.TTL).Err()
}
