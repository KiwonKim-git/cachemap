package cacheRedis

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/KiwonKim-git/cachemap/schema"
	"github.com/redis/go-redis/v9"
)

type CacheRedis struct {
	cache       *redis.Client
	cacheConfig *schema.CacheMapConf
}

func CreateCacheRedis(ctx context.Context, config *schema.CacheMapConf) *CacheRedis {

	c := &CacheRedis{
		cache:       nil,
		cacheConfig: &schema.CacheMapConf{},
	}

	if config != nil && config.RedisConf != nil {

		c.cache = getRedisClient(ctx, &redis.Options{
			ClientName: config.Name,
			Network:    config.RedisConf.RedisProtocol,
			Addr:       config.RedisConf.RedisServerAddress + ":" + config.RedisConf.RedisServerPort,
			Password:   config.RedisConf.RedisPassword, // no password set
			DB:         0,                              // use default DB
		})

		c.cacheConfig.Verbose = config.Verbose
		c.cacheConfig.Name = config.Name
		c.cacheConfig.RandomizedDuration = config.RandomizedDuration
		if c.cacheConfig.RandomizedDuration && config.CacheDuration < (60*time.Second) {
			c.cacheConfig.CacheDuration = 60 * time.Second //60s
		} else {
			c.cacheConfig.CacheDuration = config.CacheDuration
		}
		c.cacheConfig.RedisConf = config.RedisConf
	}

	log.Printf("CacheRedis CREATE - [%s] created CacheRedis cacheDuration: [%s], randomizedDuration: [%t], serverAddress: [%s:%s]",
		c.cacheConfig.Name, c.cacheConfig.CacheDuration.String(), c.cacheConfig.RandomizedDuration, c.cacheConfig.RedisConf.RedisServerAddress, c.cacheConfig.RedisConf.RedisServerPort)

	return c
}

// Store any element into Redis server.
// The KEY should be string and the VALUE should be possible to be marshaled into json format before storing.
// If you want to set specific experation time, you need to set expireAt.
// If not, you can just set expireAt as nil and the duration in cache config will be used.
func (c *CacheRedis) Store(ctx context.Context, key string, value interface{}, expireAt *time.Time) error {

	e := schema.ElementForCacheMap{}

	randomizedTime := int64(0)
	now := time.Now()
	if expireAt != nil && expireAt.After(now) {
		e.ExpireAt = *expireAt
	} else {
		if c.cacheConfig.RandomizedDuration {
			randomizedTime = int64((now.UnixNano() % 25) * int64(c.cacheConfig.CacheDuration) / 100)
			e.ExpireAt = now.Add(time.Duration(randomizedTime)).Add(c.cacheConfig.CacheDuration)
		} else {
			e.ExpireAt = now.Add(c.cacheConfig.CacheDuration)
		}
	}
	e.LastUpdated = now
	e.Value = value

	bytes, err := json.Marshal(e)
	if err != nil {
		return err
	}

	err = c.cache.Set(ctx, key, bytes, e.ExpireAt.Sub(now)).Err()
	if err != nil {
		return err
	}

	if c.cacheConfig.Verbose {
		loc := time.FixedZone("KST", 9*60*60)
		log.Printf("CacheRedis STORE - [%s] stored the key [%v] at [%s] and it will be expired at [%s] with randomizedTime: [%s], element: [%s]",
			c.cacheConfig.Name, key, e.LastUpdated.In(loc).Format(time.RFC3339), e.ExpireAt.In(loc).Format(time.RFC3339), time.Duration(randomizedTime).String(), string(bytes))
	}
	return nil
}

func (c *CacheRedis) Load(ctx context.Context, key string) (value interface{}, result schema.RESULT, lastUpdated *time.Time, err error) {

	value = nil
	result = schema.NOT_FOUND
	lastUpdated = nil
	err = nil

	bytes, err := c.cache.Get(ctx, key).Bytes()

	if err != nil {
		log.Printf("CacheRedis ERROR - [%s] has error while it gets the value of the key [%s] from Redis server, error: [%v]", c.cacheConfig.Name, key, err)
		result = schema.ERROR
	} else if err == redis.Nil {
		if c.cacheConfig.Verbose {
			log.Printf("CacheRedis NOT FOUND - [%s] does not have the key [%s]", c.cacheConfig.Name, key)
		}
		result = schema.NOT_FOUND
	} else {
		e := schema.ElementForCacheMap{}
		err = json.Unmarshal(bytes, &e)
		if err != nil {
			log.Printf("CacheRedis ERROR - [%s] has error while it unmarshal the value of the key [%s] from Redis server, error: [%v]", c.cacheConfig.Name, key, err)
		} else {
			value = e.Value
			lastUpdated = &e.LastUpdated
			now := time.Now()

			if now.Before(e.ExpireAt) {
				if c.cacheConfig.Verbose {
					log.Printf("CacheRedis LOAD - [%s] loaded the key: [%s]", c.cacheConfig.Name, key)
				}
				result = schema.VALID
			} else {
				if c.cacheConfig.Verbose {
					loc := time.FixedZone("KST", 9*60*60)
					log.Printf("CacheRedis EXPIRED - [%s] has the key [%v] but, it was expired now(): [%s], expired at: [%s]",
						c.cacheConfig.Name, key, now.In(loc).Format(time.RFC3339), e.ExpireAt.In(loc).Format(time.RFC3339))
				}
				result = schema.EXPIRED
			}
		}
	}
	return value, result, lastUpdated, err
}
