package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache is a Cache backed by Redis.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache connects to Redis and returns a ready-to-use RedisCache.
func NewRedisCache(addr, password string, db int) *RedisCache {
	return &RedisCache{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		}),
	}
}

// Ping verifies the connection is alive.
func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return val, err
}

func (r *RedisCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisCache) FlushAll(ctx context.Context) error {
	return r.client.FlushAll(ctx).Err()
}
