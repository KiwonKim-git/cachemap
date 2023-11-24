package cachemap

import (
	"log"
	"sync"
	"time"

	"github.com/robfig/cron"
)

type RESULT int

const (
	VALID RESULT = iota
	EXPIRED
	NOT_FOUND
)

// struct to store any element in map
type ElementForCacheMap struct {
	expire  time.Time
	element interface{}
}

type CacheMap struct {
	cacheMap    *sync.Map
	cacheConfig *CacheMapConf
	scheduler   CacheScheduler
}

type CacheMapConf struct {
	Name                 string
	CacheDuration        time.Duration
	RandomizedDuration   bool
	CronExprForScheduler string // Seconds Minutes Hours Day_of_month Month Day_of_week  (e.g. "0 0 12 * * *" means 12:00:00 PM every day)
}

func CreateCacheMap(config *CacheMapConf) *CacheMap {

	c := &CacheMap{
		cacheMap:    &sync.Map{},
		cacheConfig: &CacheMapConf{},
		scheduler:   CacheScheduler{},
	}

	if config != nil {
		c.cacheConfig.Name = config.Name
		c.cacheConfig.RandomizedDuration = config.RandomizedDuration
		if c.cacheConfig.RandomizedDuration && config.CacheDuration < (60*time.Second) {
			c.cacheConfig.CacheDuration = 60 * time.Second //60s
		} else {
			c.cacheConfig.CacheDuration = config.CacheDuration
		}
		c.cacheConfig.CronExprForScheduler = config.CronExprForScheduler
	} else {
		c.cacheConfig.Name = "cacheMap"
		c.cacheConfig.RandomizedDuration = false
		c.cacheConfig.CacheDuration = 1 * time.Hour        //1hour
		c.cacheConfig.CronExprForScheduler = "0 0 * * * *" //every hour
	}

	c.scheduler.job = CacheJob{
		name:     c.cacheConfig.Name,
		cacheMap: c.cacheMap,
	}

	c.scheduler.cron = cron.New()
	c.scheduler.cron.AddJob(c.cacheConfig.CronExprForScheduler, c.scheduler.job)
	c.scheduler.cron.Start()

	log.Printf("cachemap.go# [%s] CreateCacheMap cacheDuration: [%s], randomizedDuration: [%t], cronExprForScheduler: [%s]",
		c.cacheConfig.Name, c.cacheConfig.CacheDuration.String(), c.cacheConfig.RandomizedDuration, c.cacheConfig.CronExprForScheduler)

	return c
}

// Store any element into cachemap.
// If you want to set specific experation time, you need to set expireAt.
// If not, but want to use the duration of cachemap, you can just set expireAt as nil
func (c *CacheMap) Store(key interface{}, value interface{}, expireAt *time.Time) {

	e := ElementForCacheMap{}

	randomizedTime := int64(0)
	now := time.Now()
	if expireAt != nil && expireAt.After(now) {
		e.expire = *expireAt
	} else {
		if c.cacheConfig.RandomizedDuration {
			randomizedTime = int64((now.UnixNano() % 25) * int64(c.cacheConfig.CacheDuration) / 100)
			e.expire = now.Add(time.Duration(randomizedTime)).Add(c.cacheConfig.CacheDuration)
		} else {
			e.expire = now.Add(c.cacheConfig.CacheDuration)
		}
	}
	e.element = value

	c.cacheMap.Store(key, e)

	loc := time.FixedZone("KST", 9*60*60)
	log.Printf("cachemap.go# [%s] Store key: [%v], randomizedTime: [%s], will be expired at: [%s]",
		c.cacheConfig.Name, key, time.Duration(randomizedTime).String(), e.expire.In(loc).Format(time.RFC3339))
}

func (c *CacheMap) Load(key interface{}) (element interface{}, result RESULT) {

	e, ok := c.cacheMap.Load(key)

	if ok && e != nil {

		value, ok := e.(ElementForCacheMap)

		now := time.Now()

		if ok && now.Before(value.expire) {
			log.Printf("cachemap.go# [%s] Load key: [%v]", c.cacheConfig.Name, key)
			return value.element, VALID
		} else {
			loc := time.FixedZone("KST", 9*60*60)
			log.Printf("cachemap.go# [%s] cache was expired and/or data is invaild, key : [%v], ok : [%t], now() : [%s], expired at : [%s]",
				c.cacheConfig.Name, key, ok, now.In(loc).Format(time.RFC3339), value.expire.In(loc).Format(time.RFC3339))
			return value.element, EXPIRED
		}
	}
	return nil, NOT_FOUND
}
