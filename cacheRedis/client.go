package cacheRedis

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

func getRedisClient(ctx context.Context, opt *redis.Options) (client *redis.Client) {

	client = redis.NewClient(opt)

	err := client.Set(ctx, "test1", "value1", 0).Err()
	if err != nil {
		panic(err)
	}

	val, err := client.Get(ctx, "test1").Result()
	if err != nil {
		panic(err)
	}
	log.Println("test1: ", val)

	val, err = client.Get(ctx, "test2").Result()
	if err == redis.Nil {
		log.Println("test2 does not exist")
	} else if err != nil {
		panic(err)
	} else {
		log.Println("test2", val)
	}
	return client
}
