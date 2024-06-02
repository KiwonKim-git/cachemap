package cache

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func getRedisClient(ctx context.Context, opt *redis.UniversalOptions) (client redis.UniversalClient) {

	client = redis.NewUniversalClient(opt)
	return client
}
