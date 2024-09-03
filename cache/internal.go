package cache

import (
	"time"

	"github.com/KiwonKim-git/cachemap/schema"
)

// struct to store any element in map
type elementForCache struct {
	// Entry in cache will be expired at this time
	ExpireAt time.Time `json:"expireAt"`
	// Last updated time of the entry in cache
	LastUpdated time.Time `json:"lastUpdated"`
	// Actual Value in cache
	Value interface{} `json:"value"`
}

func getSchedulerConfig(cacheConf *schema.CacheConf) (schedulerConf *schema.SchedulerConf) {

	cronExpr := "0 0 * * * *" // Default. run every hour
	var preProc schema.ProcessFunc = nil
	var postProc schema.ProcessFunc = nil

	if cacheConf != nil {
		if cacheConf.SchedulerConf != nil {

			if cacheConf.SchedulerConf.CronExprForScheduler != "" {
				cronExpr = cacheConf.SchedulerConf.CronExprForScheduler
			}
			preProc = cacheConf.SchedulerConf.PreProcess
			postProc = cacheConf.SchedulerConf.PostProcess
		} else if cacheConf.RedisConf != nil {
			// if there is no scheduler configuration for Redis cache,
			// return nil to let Redis set expiration time for each key
			return nil
		}
	}
	schedulerConf = &schema.SchedulerConf{
		CronExprForScheduler: cronExpr,
		PreProcess:           preProc,
		PostProcess:          postProc,
	}
	return schedulerConf
}
