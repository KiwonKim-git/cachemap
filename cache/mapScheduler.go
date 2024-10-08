package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/KiwonKim-git/cachemap/schema"
	"github.com/KiwonKim-git/cachemap/util"
	"github.com/robfig/cron"
)

type mapScheduler struct {
	cron *cron.Cron
	job  mapJob
}

type mapJob struct {
	name    string
	cache   *sync.Map
	total   int
	expired int
	config  *schema.CacheConf
}

func (j mapJob) Run() {

	j.total = 0
	j.expired = 0

	if j.cache != nil {
		j.cache.Range(j.removeExpiredEntry)

	} else {
		j.config.Logger.PrintLogs(util.ERROR, fmt.Sprintf("CacheJob - [%s] runs but, cache is nil", j.name))
		return
	}

	j.config.Logger.PrintLogs(util.ERROR,
		fmt.Sprintf("CacheJob - [%s] runs at [%s], Total: [%d], Expired: [%d]", j.name, time.Now().In(time.FixedZone("KST", 9*60*60)).Format(time.RFC3339), j.total, j.expired))
}
func (j *mapJob) removeExpiredEntry(key, value interface{}) bool {

	j.increaseTotalEntry()
	// log.Printf("CacheScheduler.go# [%s] existing entries - key: [%v]", j.name, key)

	element, ok := value.(elementForCache)
	now := time.Now()

	if !ok {
		j.config.Logger.PrintLogs(util.ERROR, fmt.Sprint("Failed while converting from interface{} to the cache element. Key: ", key))
	} else if now.After(element.ExpireAt) {

		j.increaseExpiredEntry()

		if j.config != nil && j.config.SchedulerConf != nil && j.config.SchedulerConf.PreProcess != nil {
			err := j.config.SchedulerConf.PreProcess(element.Value)
			if err != nil {
				j.config.Logger.PrintLogs(util.ERROR, fmt.Sprint("Failed while pre-processing before deletion of the element. Error: ", err))
			}
		}

		j.config.Logger.PrintLogs(util.DEBUG,
			fmt.Sprintf("CacheJob REMOVE - [%s] removeExpiredEntry key: [%v] expired at [%s] \n",
				j.name, key, element.ExpireAt.In(time.FixedZone("KST", 9*60*60)).Format(time.RFC3339)))

		j.cache.Delete(key)

		if j.config != nil && j.config.SchedulerConf != nil && j.config.SchedulerConf.PostProcess != nil {
			err := j.config.SchedulerConf.PostProcess(element.Value)
			if err != nil {
				j.config.Logger.PrintLogs(util.ERROR, fmt.Sprint("Failed while post-processing after deletion of the element. Error: ", err))
			}
		}
	}

	return true
}

func (j *mapJob) increaseTotalEntry() (total int) {

	j.total += 1
	return j.total
}

func (j *mapJob) increaseExpiredEntry() (total int) {

	j.expired += 1
	return j.expired
}

func getMapScheduler(cacheMap *sync.Map, config *schema.CacheConf) (scheduler *mapScheduler) {

	scheduler = &mapScheduler{
		cron: cron.New(),
		job: mapJob{
			name:    config.Name,
			cache:   cacheMap,
			total:   0,
			expired: 0,
			config:  config,
		},
	}
	if scheduler.job.config.Logger == nil {
		config.Logger = util.Default()
	}
	scheduler.cron.AddJob(config.SchedulerConf.CronExprForScheduler, scheduler.job)
	scheduler.cron.Start()
	return scheduler
}
