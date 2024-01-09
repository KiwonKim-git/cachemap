package cacheRedis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func getRedisClient(ctx context.Context, opt *redis.Options) (client *redis.Client) {

	client = redis.NewClient(opt)
	return client
}
