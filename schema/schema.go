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
type ElementForCache struct {
	ExpireAt    time.Time
	LastUpdated time.Time
	Value       interface{}
}

type CacheConf struct {
	Verbose              bool
	Name                 string
	CacheDuration        time.Duration
	RandomizedDuration   bool
	CronExprForScheduler string // Seconds Minutes Hours Day_of_month Month Day_of_week  (e.g. "0 0 12 * * *" means 12:00:00 PM every day)
	RedisConf            *RedisConf
}

type RedisConf struct {
	// Namespace for database to store KEY and VALUE in same logical storage. Usually service name if the appliction consist of multiple applications.
	/* Default: Name value in CacheConf */
	Namespace string
	// Group name to prevent key duplication in same namespcae. E.g., if you want to use session ID as a KEY in differnt data cetegories, it may cause key duplication among data categories.
	/* Default: empty */
	Group         string
	ServerAddress string
	ServerPort    string
	Protocol      string
	Password      string
}
