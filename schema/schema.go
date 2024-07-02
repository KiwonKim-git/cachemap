package schema

import (
	"fmt"
	"time"
)

type RESULT int

const (
	VALID RESULT = iota
	EXPIRED
	NOT_FOUND
	ERROR
)

func (r RESULT) String() string {
	switch r {
	case VALID:
		return "VALID"
	case EXPIRED:
		return "EXPIRED"
	case NOT_FOUND:
		return "NOT_FOUND"
	case ERROR:
		return "ERROR"
	}
	return fmt.Sprintf("%d", int(r))
}

type CacheConf struct {
	// True if the cache should print more logs for debugging purpose
	Verbose bool
	// Name of the cache
	Name string
	// Duration for cache to be expired
	CacheDuration time.Duration
	// True if the cache duration should be randomized
	RandomizedDuration bool
	// Redis configuration if this cache is used with Redis Cluster
	RedisConf *RedisConf
	// Configuration for scheduler to remove expired entry
	SchedulerConf *SchedulerConf
}

type SchedulerConf struct {
	// Seconds Minutes Hours Day_of_month Month Day_of_week  (e.g. "0 0 12 * * *" means 12:00:00 PM every day)
	CronExprForScheduler string
	// A function that will be called before removing the expired entry
	PreProcess ProcessFunc
	// A function that will be called after removing the expired entry
	PostProcess ProcessFunc
}

// A function that process data entry in cache. Returns error if any.
type ProcessFunc func(value interface{}) (err error)

type RedisConf struct {
	// Namespace for database to store KEY and VALUE in same logical storage. Usually service name if the appliction consist of multiple applications.
	/* Default: Name value in CacheConf */
	Namespace string
	// Group name to prevent key duplication in same namespcae. E.g., if you want to use session ID as a KEY in differnt data cetegories, it may cause key duplication among data categories.
	/* Default: empty */
	Group string
	// A list of server address for Redis Cluster
	/* [host1:port1, host2:port2, ... hostN:portN] */
	ServerAddresses []string
	Username        string
	Password        string
}
