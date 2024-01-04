package cacheMap

import (
	"log"
	"sync"
	"time"

	"github.com/KiwonKim-git/cachemap/schema"
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
		log.Printf("CacheJob - [%s] runs but, cacheMap is nil", j.name)
		return
	}

	log.Printf("CacheJob - [%s] runs at [%s], Total: [%d], Expired: [%d]", j.name, now.In(loc).Format(time.RFC3339), j.total, j.expired)
}
func (j *CacheJob) removeExpiredEntry(key, value interface{}) bool {

	j.total++
	// log.Printf("CacheScheduler.go# [%s] existing entries - key: [%v]", j.name, key)

	element, ok := value.(schema.ElementForCacheMap)
	now := time.Now()

	if ok && now.After(element.ExpireAt) {

		j.expired++
		loc := time.FixedZone("KST", 9*60*60)
		log.Printf("CacheJob REMOVE - [%s] removeExpiredEntry key: [%v] expired at [%s]", j.name, key, element.ExpireAt.In(loc).Format(time.RFC3339))

		j.cacheMap.Delete(key)
	}

	return true
}
