package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/KiwonKim-git/cachemap/schema"
	"github.com/KiwonKim-git/cachemap/util"
	"github.com/redis/go-redis/v9"
)

type CacheRedis struct {
	cache       redis.UniversalClient
	cacheConfig *schema.CacheConf
	scheduler   *redisScheduler
}

const KEY_PREFIX_SHADOW = "shadow:"
const KEY_PREFIX_EXPIRED = "expired:"
const KEY_PREFIX_LOCK = "locks:"

func NewCacheRedis(config *schema.CacheConf) *CacheRedis {

	c := &CacheRedis{
		cache:       nil,
		cacheConfig: &schema.CacheConf{},
		scheduler:   nil,
	}

	if config != nil && config.Logger != nil {
		c.cacheConfig.Logger = config.Logger
	} else {
		c.cacheConfig.Logger = util.Default()
	}

	if config != nil && config.RedisConf != nil {

		c.cache = redis.NewUniversalClient(&redis.UniversalOptions{
			ClientName: config.Name,
			Addrs:      config.RedisConf.ServerAddresses,
			Username:   config.RedisConf.Username, // no username specified
			Password:   config.RedisConf.Password, // no password set
		})

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

		c.cacheConfig.SchedulerConf = getSchedulerConfig(config)

		c.scheduler = getRedisScheduler(c.cache, c.cacheConfig)

		c.cacheConfig.Logger.PrintLogs(util.ERROR,
			fmt.Sprintf("CacheRedis CREATE - [%s] has been created. cacheDuration: [%s], randomizedDuration: [%t], serverAddress: [%v]",
				c.cacheConfig.Name, c.cacheConfig.CacheDuration.String(), c.cacheConfig.RandomizedDuration, c.cacheConfig.RedisConf.ServerAddresses))
	} else {
		c.cacheConfig.Logger.PrintLogs(util.ERROR, "CacheRedis CREATE - CacheRedis is created with nil config or RedisConf")
	}

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

	keyPrefix := getRedisKeyPrefix(c.cacheConfig.RedisConf)
	actualKey := keyPrefix + key

	// if the scheduler in CacheRedis is nil, set the expiration time to make Redis handle the expired keys and values.
	// Otherwise, set the expiration time to 0 to make Cache Scheduler handle the expired keys and values.
	// And then, set the shadow key with empty value to get expiration event.
	// This is for the case that the client wants to use pre-process and/or post-process function before and after deletion of expired entries.
	clusterClient, ok := c.cache.(*redis.ClusterClient)
	if !ok {
		err = fmt.Errorf("CacheRedis ERROR - [%s] failed to convert the client to *redis.ClusterClient", c.cacheConfig.Name)
		return err
	}
	if c.scheduler == nil {
		err = clusterClient.Set(ctx, actualKey, bytes, e.ExpireAt.Sub(now)).Err()
	} else {
		err = clusterClient.Set(ctx, actualKey, bytes, 0).Err()
		if err == nil {
			shadowKey := keyPrefix + KEY_PREFIX_SHADOW + key
			err = clusterClient.Set(ctx, shadowKey, "", e.ExpireAt.Sub(now)).Err()
			if err != nil {
				c.cacheConfig.Logger.PrintLogs(util.ERROR, fmt.Sprint("CacheRedis STORE - Failed to store shadow key and it should be deleted manually. Key: ", actualKey))
				// ignore error while storing shadow key
				err = nil
			}
		}
	}
	if err == nil {
		c.cacheConfig.Logger.PrintLogs(util.DEBUG,
			fmt.Sprintf("CacheRedis STORE - [%s] stored the key [%s] (actual: %s) at [%s] and it will be expired at [%s] with randomizedTime: [%s], element: [%s]",
				c.cacheConfig.Name, key, actualKey, e.LastUpdated.In(time.FixedZone("KST", 9*60*60)).Format(time.RFC3339), e.ExpireAt.In(time.FixedZone("KST", 9*60*60)).Format(time.RFC3339), time.Duration(randomizedTime).String(), string(bytes)))
	}
	return err
}

func (c *CacheRedis) Load(ctx context.Context, key string) (value interface{}, result schema.RESULT, lastUpdated *time.Time, err error) {

	actualKey := getRedisKeyPrefix(c.cacheConfig.RedisConf) + key

	clusterClient, ok := c.cache.(*redis.ClusterClient)
	if !ok {
		err = fmt.Errorf("CacheRedis ERROR - [%s] failed to convert the client to *redis.ClusterClient", c.cacheConfig.Name)
		return nil, schema.ERROR, nil, err
	}
	return getValueFromRedis(ctx, clusterClient, actualKey, c.cacheConfig)
}

func getValueFromRedis(ctx context.Context, client *redis.ClusterClient, key string, config *schema.CacheConf) (value interface{}, result schema.RESULT, lastUpdated *time.Time, err error) {

	value = nil
	result = schema.NOT_FOUND
	lastUpdated = nil
	err = nil

	bytes, err := client.Get(ctx, key).Bytes()

	if err != nil {
		if err == redis.Nil {
			config.Logger.PrintLogs(util.DEBUG, fmt.Sprintf("CacheRedis NOT FOUND - [%s] does not have the key [%s]", config.Name, key))
			result = schema.NOT_FOUND
			err = nil // handle this case as not an error
		} else {
			config.Logger.PrintLogs(util.ERROR, fmt.Sprintf("CacheRedis ERROR - [%s] has error while it gets the value of the key [%s] from Redis server, error: [%v]", config.Name, key, err))
			result = schema.ERROR
		}
	} else {
		e := elementForCache{}
		err = json.Unmarshal(bytes, &e)
		if err != nil {
			config.Logger.PrintLogs(util.ERROR, fmt.Sprintf("CacheRedis ERROR - [%s] has error while it unmarshal the value of the key [%s] from Redis server, error: [%v]", config.Name, key, err))
		} else {
			value = e.Value
			lastUpdated = &e.LastUpdated
			now := time.Now()

			if now.Before(e.ExpireAt) {
				config.Logger.PrintLogs(util.DEBUG, fmt.Sprintf("CacheRedis LOAD - [%s] loaded the key: [%s]", config.Name, key))
				result = schema.VALID
			} else {
				config.Logger.PrintLogs(util.DEBUG, fmt.Sprintf("CacheRedis EXPIRED - [%s] has the key [%v] but, it was expired now [%s], expired at: [%s]",
					config.Name, key, now.In(time.FixedZone("KST", 9*60*60)).Format(time.RFC3339), e.ExpireAt.In(time.FixedZone("KST", 9*60*60)).Format(time.RFC3339)))
				result = schema.EXPIRED
			}
		}
	}
	return value, result, lastUpdated, err
}

func getRedisKeyPrefix(config *schema.RedisConf) (prifix string) {
	if config == nil {
		return ""
	}
	prifix = config.Namespace
	if config.Group != "" {
		prifix += ":" + config.Group
	}
	return prifix + ":"
}
