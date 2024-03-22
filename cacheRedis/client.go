package cacheRedis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func getRedisClient(ctx context.Context, opt *redis.ClusterOptions) (client *redis.ClusterClient) {

	client = redis.NewClusterClient(opt)
	return client
}
