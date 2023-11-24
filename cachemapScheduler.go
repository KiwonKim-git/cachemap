package cachemap

import (
	"log"
	"sync"
	"time"

	"github.com/robfig/cron"
)

type CacheScheduler struct {
	cron *cron.Cron
	job  CacheJob
}

type CacheJob struct {
	name     string
	cacheMap *sync.Map
	total    int
	expired  int
}

func (j CacheJob) Run() {

	loc := time.FixedZone("KST", 9*60*60)
	now := time.Now()

	j.total = 0
	j.expired = 0

	if j.cacheMap != nil {
		j.cacheMap.Range(j.removeExpiredEntry)

	} else {
		log.Printf("CacheScheduler.go# [%s] CacheJob runs but, cacheMap is nil", j.name)
		return
	}

	log.Printf("CacheScheduler.go# [%s] CacheJob runs at [%s], Total: [%d], Expired: [%d]", j.name, now.In(loc).Format(time.RFC3339), j.total, j.expired)
}
func (j *CacheJob) removeExpiredEntry(key, value interface{}) bool {

	j.total++
	// log.Printf("CacheScheduler.go# [%s] existing entries - key: [%v]", j.name, key)

	element, ok := value.(ElementForCacheMap)
	now := time.Now()

	if ok && now.After(element.expire) {

		j.expired++
		loc := time.FixedZone("KST", 9*60*60)
		log.Printf("CacheScheduler.go# [%s] removeExpiredEntry key: [%v] expired at [%s]", j.name, key, element.expire.In(loc).Format(time.RFC3339))

		j.cacheMap.Delete(key)

		log.Printf("CacheScheduler.go# [%s] removeExpiredEntry delete key: [%v]", j.name, key)
	}

	return true
}
