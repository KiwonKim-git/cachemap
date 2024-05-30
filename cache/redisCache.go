package cache

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/KiwonKim-git/cachemap/schema"
	"github.com/redis/go-redis/v9"
)

type CacheRedis struct {
	cache       *redis.ClusterClient
	cacheConfig *schema.CacheConf
}

func NewCacheRedis(ctx context.Context, config *schema.CacheConf) *CacheRedis {

	c := &CacheRedis{
		cache:       nil,
		cacheConfig: &schema.CacheConf{},
	}

	if config != nil && config.RedisConf != nil {

		c.cache = getRedisClient(ctx, &redis.ClusterOptions{
			ClientName: config.Name,
			Addrs:      config.RedisConf.ServerAddresses,
			Username:   config.RedisConf.Username, // no username specified
			Password:   config.RedisConf.Password, // no password set
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
		if config.RedisConf.Namespace == "" {
			c.cacheConfig.RedisConf.Namespace = c.cacheConfig.Name
		}
	}

	log.Printf("CacheRedis CREATE - [%s] created CacheRedis cacheDuration: [%s], randomizedDuration: [%t], serverAddress: [%v]",
		c.cacheConfig.Name, c.cacheConfig.CacheDuration.String(), c.cacheConfig.RandomizedDuration, c.cacheConfig.RedisConf.ServerAddresses)

	return c
}

// Store any element into Redis server.
// The KEY should be string and the VALUE should be possible to be marshaled into json format before storing.
// If you want to set specific experation time, you need to set expireAt.
// If not, you can just set expireAt as nil and the duration in cache config will be used.
func (c *CacheRedis) Store(ctx context.Context, key string, value interface{}, expireAt *time.Time) error {

	e := elementForCache{}

	randomizedTime := int64(0)
	now := time.Now()
	if expireAt != nil && expireAt.After(now) {
		e.expireAt = *expireAt
	} else {
		if c.cacheConfig.RandomizedDuration {
			randomizedTime = int64((now.UnixNano() % 25) * int64(c.cacheConfig.CacheDuration) / 100)
			e.expireAt = now.Add(time.Duration(randomizedTime)).Add(c.cacheConfig.CacheDuration)
		} else {
			e.expireAt = now.Add(c.cacheConfig.CacheDuration)
		}
	}
	e.lastUpdated = now
	e.value = value

	bytes, err := json.Marshal(e)
	if err != nil {
		return err
	}

	actualKey := c.cacheConfig.RedisConf.Namespace
	if c.cacheConfig.RedisConf.Group != "" {
		actualKey += ":" + c.cacheConfig.RedisConf.Group
	}
	actualKey += ":" + key

	err = c.cache.Set(ctx, actualKey, bytes, e.expireAt.Sub(now)).Err()
	if err != nil {
		return err
	}

	if c.cacheConfig.Verbose {
		loc := time.FixedZone("KST", 9*60*60)
		log.Printf("CacheRedis STORE - [%s] stored the key [%s] (actual: %s) at [%s] and it will be expired at [%s] with randomizedTime: [%s], element: [%s]",
			c.cacheConfig.Name, key, actualKey, e.lastUpdated.In(loc).Format(time.RFC3339), e.expireAt.In(loc).Format(time.RFC3339), time.Duration(randomizedTime).String(), string(bytes))
	}
	return nil
}

func (c *CacheRedis) Load(ctx context.Context, key string) (value interface{}, result schema.RESULT, lastUpdated *time.Time, err error) {

	value = nil
	result = schema.NOT_FOUND
	lastUpdated = nil
	err = nil

	actualKey := c.cacheConfig.RedisConf.Namespace
	if c.cacheConfig.RedisConf.Group != "" {
		actualKey += ":" + c.cacheConfig.RedisConf.Group
	}
	actualKey += ":" + key

	bytes, err := c.cache.Get(ctx, actualKey).Bytes()

	if err != nil {
		if err == redis.Nil {
			if c.cacheConfig.Verbose {
				log.Printf("CacheRedis NOT FOUND - [%s] does not have the key [%s] (actual: %s)", c.cacheConfig.Name, key, actualKey)
			}
			result = schema.NOT_FOUND
			err = nil // handle this case as not an error
		} else {
			log.Printf("CacheRedis ERROR - [%s] has error while it gets the value of the key [%s] (actual: %s) from Redis server, error: [%v]", c.cacheConfig.Name, key, actualKey, err)
			result = schema.ERROR
		}
	} else {
		e := elementForCache{}
		err = json.Unmarshal(bytes, &e)
		if err != nil {
			log.Printf("CacheRedis ERROR - [%s] has error while it unmarshal the value of the key [%s] (actual: %s) from Redis server, error: [%v]", c.cacheConfig.Name, key, actualKey, err)
		} else {
			value = e.value
			lastUpdated = &e.lastUpdated
			now := time.Now()

			if now.Before(e.expireAt) {
				if c.cacheConfig.Verbose {
					log.Printf("CacheRedis LOAD - [%s] loaded the key: [%s] (actual: %s)", c.cacheConfig.Name, key, actualKey)
				}
				result = schema.VALID
			} else {
				if c.cacheConfig.Verbose {
					loc := time.FixedZone("KST", 9*60*60)
					log.Printf("CacheRedis EXPIRED - [%s] has the key [%v] (actual: %s) but, it was expired now(): [%s], expired at: [%s]",
						c.cacheConfig.Name, key, actualKey, now.In(loc).Format(time.RFC3339), e.expireAt.In(loc).Format(time.RFC3339))
				}
				result = schema.EXPIRED
			}
		}
	}
	return value, result, lastUpdated, err
}
