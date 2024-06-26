package cache

import (
	"log"
	"sync"
	"time"

	"github.com/KiwonKim-git/cachemap/schema"
	"github.com/robfig/cron"
)

type cacheScheduler struct {
	cron *cron.Cron
	job  cacheJob
}

type cacheJob struct {
	name     string
	cacheMap *sync.Map
	total    int
	expired  int
	config   *schema.CacheConf
}

func (j cacheJob) Run() {

	loc := time.FixedZone("KST", 9*60*60)
	now := time.Now()

	j.total = 0
	j.expired = 0

	if j.cacheMap != nil {
		j.cacheMap.Range(j.removeExpiredEntry)

	} else {
		log.Printf("CacheJob - [%s] runs but, cacheMap is nil", j.name)
		return
	}

	log.Printf("CacheJob - [%s] runs at [%s], Total: [%d], Expired: [%d]", j.name, now.In(loc).Format(time.RFC3339), j.total, j.expired)
}
func (j *cacheJob) removeExpiredEntry(key, value interface{}) bool {

	j.increaseTotalEntry()
	// log.Printf("CacheScheduler.go# [%s] existing entries - key: [%v]", j.name, key)

	element, ok := value.(elementForCache)
	now := time.Now()

	if !ok {
		log.Println("Failed while converting from interface{} to the cache element. Key: ", key)
	} else if now.After(element.expireAt) {

		j.increaseExpiredEntry()

		if j.config != nil && j.config.SchedulerConf != nil && j.config.SchedulerConf.PreProcess != nil {
			logs, err := j.config.SchedulerConf.PreProcess(element.value)
			if j.config.Verbose && logs != "" {
				log.Println(logs)
			}
			if err != nil {
				log.Println("Failed while pre-processing before deletion of the element. Error: ", err)
			}
		}

		if j.config.Verbose {
			loc := time.FixedZone("KST", 9*60*60)
			log.Printf("CacheJob REMOVE - [%s] removeExpiredEntry key: [%v] expired at [%s] \n", j.name, key, element.expireAt.In(loc).Format(time.RFC3339))
		}

		j.cacheMap.Delete(key)

		if j.config != nil && j.config.SchedulerConf != nil && j.config.SchedulerConf.PostProcess != nil {
			logs, err := j.config.SchedulerConf.PostProcess(element.value)
			if j.config.Verbose && logs != "" {
				log.Println(logs)
			}
			if err != nil {
				log.Println("Failed while post-processing after deletion of the element. Error: ", err)
			}
		}
	}

	return true
}

func (j *cacheJob) increaseTotalEntry() (total int) {

	j.total += 1
	return j.total
}

func (j *cacheJob) increaseExpiredEntry() (total int) {

	j.expired += 1
	return j.expired
}

func getCacheScheduler(cacheMap *sync.Map, config *schema.CacheConf) (scheduler *cacheScheduler) {
	scheduler = &cacheScheduler{}

	scheduler.job = cacheJob{
		name:     config.Name,
		cacheMap: cacheMap,
		total:    0,
		expired:  0,
		config:   config,
	}

	scheduler.cron = cron.New()
	scheduler.cron.AddJob(config.SchedulerConf.CronExprForScheduler, scheduler.job)
	scheduler.cron.Start()
	return scheduler
}

func getCacheSchedulerConfig(cacheConf *schema.CacheConf) (schedulerConf *schema.SchedulerConf) {

	cronExpr := "0 0 * * * *" // Default. run every hour
	var preProc schema.ProcessFunc = nil
	var postProc schema.ProcessFunc = nil

	if cacheConf != nil && cacheConf.SchedulerConf != nil {

		if cacheConf.SchedulerConf.CronExprForScheduler != "" {
			cronExpr = cacheConf.SchedulerConf.CronExprForScheduler
		}
		preProc = cacheConf.SchedulerConf.PreProcess
		postProc = cacheConf.SchedulerConf.PostProcess
	}
	schedulerConf = &schema.SchedulerConf{
		CronExprForScheduler: cronExpr,
		PreProcess:           preProc,
		PostProcess:          postProc,
	}
	return schedulerConf
}
