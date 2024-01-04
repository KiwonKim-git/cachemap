package schema

import (
	"time"
)

type RESULT int

const (
	VALID RESULT = iota
	EXPIRED
	NOT_FOUND
	ERROR
)

// struct to store any element in map
type ElementForCacheMap struct {
	ExpireAt    time.Time
	LastUpdated time.Time
	Value       interface{}
}

type CacheMapConf struct {
	Verbose              bool
	Name                 string
	CacheDuration        time.Duration
	RandomizedDuration   bool
	CronExprForScheduler string // Seconds Minutes Hours Day_of_month Month Day_of_week  (e.g. "0 0 12 * * *" means 12:00:00 PM every day)
	RedisConf            *CacheRedisConf
}

type CacheRedisConf struct {
	RedisServerAddress string
	RedisServerPort    string
	RedisProtocol      string
	RedisPassword      string
}
