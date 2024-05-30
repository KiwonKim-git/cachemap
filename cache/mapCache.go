package cache

import (
	"log"
	"sync"
	"time"

	"github.com/KiwonKim-git/cachemap/schema"
)

type CacheMap struct {
	cacheMap    *sync.Map
	cacheConfig *schema.CacheConf
	scheduler   *cacheScheduler
}

func NewCacheMap(config *schema.CacheConf) *CacheMap {

	c := &CacheMap{
		cacheMap:    &sync.Map{},
		cacheConfig: &schema.CacheConf{},
	}

	if config != nil {
		c.cacheConfig.Verbose = config.Verbose
		c.cacheConfig.Name = config.Name
		c.cacheConfig.RandomizedDuration = config.RandomizedDuration
		if c.cacheConfig.RandomizedDuration && config.CacheDuration < (60*time.Second) {
			c.cacheConfig.CacheDuration = 60 * time.Second //60s
		} else {
			c.cacheConfig.CacheDuration = config.CacheDuration
		}
		c.cacheConfig.RedisConf = nil // do not use Redis config for sync.Map cache
	} else {
		c.cacheConfig.Verbose = false
		c.cacheConfig.Name = "cacheMap"
		c.cacheConfig.RandomizedDuration = false
		c.cacheConfig.CacheDuration = 1 * time.Hour // Default. 1 hour.
		c.cacheConfig.RedisConf = nil
	}

	c.cacheConfig.SchedulerConf = getCacheSchedulerConfig(config)

	c.scheduler = getCacheScheduler(c.cacheMap, c.cacheConfig)

	log.Printf("CacheMap CREATE - [%s] created CacheMap cacheDuration: [%s], randomizedDuration: [%t], cronExprForScheduler: [%s]",
		c.cacheConfig.Name, c.cacheConfig.CacheDuration.String(), c.cacheConfig.RandomizedDuration, c.cacheConfig.SchedulerConf.CronExprForScheduler)

	return c
}

// Store any element into sync.Map in golang.
// If you want to set specific experation time, you need to set expireAt.
// If not, you can just set expireAt as nil and the duration in cache config will be used.
func (c *CacheMap) Store(key interface{}, value interface{}, expireAt *time.Time) {

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

	c.cacheMap.Store(key, e)

	if c.cacheConfig.Verbose {
		loc := time.FixedZone("KST", 9*60*60)
		log.Printf("CacheMap STORE - [%s] stored the key [%v] at [%s] and it will be expired at [%s] with randomizedTime: [%s]",
			c.cacheConfig.Name, key, e.lastUpdated.In(loc).Format(time.RFC3339), e.expireAt.In(loc).Format(time.RFC3339), time.Duration(randomizedTime).String())
	}
}

func (c *CacheMap) Load(key interface{}) (value interface{}, result schema.RESULT, lastUpdated *time.Time) {

	value = nil
	result = schema.NOT_FOUND
	lastUpdated = nil

	v, ok := c.cacheMap.Load(key)

	if ok && v != nil {

		e, ok := v.(elementForCache)
		value = e.value
		lastUpdated = &e.lastUpdated

		now := time.Now()
		if ok && now.Before(e.expireAt) {
			if c.cacheConfig.Verbose {
				log.Printf("CacheMap LOAD - [%s] loaded the key: [%v]", c.cacheConfig.Name, key)
			}
			result = schema.VALID
		} else {
			if c.cacheConfig.Verbose {
				loc := time.FixedZone("KST", 9*60*60)
				log.Printf("CacheMap EXPIRED - [%s] has the key [%v] but, it was expired and/or the data is invaild, ok : [%t], now() : [%s], expired at : [%s]",
					c.cacheConfig.Name, key, ok, now.In(loc).Format(time.RFC3339), e.expireAt.In(loc).Format(time.RFC3339))
			}
			result = schema.EXPIRED
		}
	}
	return value, result, lastUpdated
}
