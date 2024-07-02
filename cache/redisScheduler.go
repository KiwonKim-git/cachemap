package cache

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/KiwonKim-git/cachemap/schema"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron"
)

type redisScheduler struct {
	cron              *cron.Cron
	job               redisJob
	expiredKeyChannel chan string
}

type redisJob struct {
	name         string
	cache        redis.UniversalClient
	totalExpired int
	handled      int
	config       *schema.CacheConf
	keyLock      *RedisLockPool
}

func (j redisJob) Run() {

	loc := time.FixedZone("KST", 9*60*60)
	now := time.Now()

	j.totalExpired = 0
	j.handled = 0

	if j.cache != nil {
		expiredPrefix := KEY_PREFIX_EXPIRED + "{" + getRedisKeyPrefix(j.config.RedisConf) + "*"

		// TODO: remove logs
		log.Printf("CacheJob - expiredPrefix: [%s]", expiredPrefix)

		clusterClient, ok := j.cache.(*redis.ClusterClient)
		if !ok {
			log.Println("CacheJob - Failed to convert j.cache to *redis.ClusterClient")
			return
		}
		err := clusterClient.ForEachMaster(context.Background(), func(ctx context.Context, client *redis.Client) error {
			iter := client.Scan(ctx, 0, expiredPrefix, 0).Iterator()

			for iter.Next(ctx) {
				j.iterate(iter.Val())
			}
			return iter.Err()
		})
		if err != nil {
			log.Println("CacheJob - Failed while scanning keys in Redis. Error: ", err)
			return
		}
	} else {
		log.Printf("CacheJob - [%s] runs but, cache is nil", j.name)
		return
	}

	log.Printf("CacheJob - [%s] runs at [%s], TotalExpired: [%d], Handled: [%d]", j.name, now.In(loc).Format(time.RFC3339), j.totalExpired, j.handled)
}

func (j *redisJob) iterate(expiredKey string) {

	j.increaseTotalExpiredEntry()

	key := strings.Replace(expiredKey, KEY_PREFIX_EXPIRED+"{"+getRedisKeyPrefix(j.config.RedisConf), "", 1)
	key = strings.Replace(key, "}", "", 1)
	// TODO: remove logs
	log.Printf("CacheJob - [%s] existing entries - expiredKey: [%v], key: [%v]", j.name, expiredKey, key)

	// TODO: remove logs
	log.Printf("[RedisLock][Iterate] Try Lock: [%s]", key)
	lockError := j.keyLock.TryLock(key)
	defer func() {
		if lockError == nil {
			log.Printf("[RedisLock][Iterate] Try Unlock: [%s]", key)
			ok, err := j.keyLock.Unlock(key)
			if ok {
				log.Printf("[RedisLock][Iterate] Unlocked: [%s]", key)
			} else {
				if err != nil {
					log.Println("[RedisLock][Iterate] Error while Unlock. error: ", err)
				} else {
					log.Println("[RedisLock][Iterate] Unlock failed without error")
				}
			}
		}
	}()
	//////////////////////////////////

	if lockError == nil {
		// TODO: remove logs
		log.Printf("[RedisLock][Iterate] Locked: [%s]", key)

		clusterClient, ok := j.cache.(*redis.ClusterClient)
		if !ok {
			err := fmt.Errorf("CacheRedis ERROR - [%s] failed to convert the client to *redis.ClusterClient", j.config.Name)
			log.Println(err)
			return
		}

		value, result, _, err := getValueFromRedis(context.Background(), clusterClient, expiredKey, j.config)

		if err != nil {
			log.Println("CacheJob - Failed while getting value from Redis. Error: ", err)
		} else if result == schema.EXPIRED && value != nil {
			j.removeExpiredEntry(key, value)
		} else if value != nil {
			log.Printf("CacheJob - the [%s] key is not expired yet (status : %v) but, why is this key moved to EXPIRED namespace? anyway remove it.", key, result.String())
			j.removeExpiredEntry(key, value)
		} else {
			log.Printf("CacheJob - Why is the [%s] key moved to EXPIRED namespace? expired key : [%s]", key, expiredKey)
		}
	} else {
		log.Println("[RedisLock][Iterate] Failed while locking the key. Skip to next expired key. Error: ", lockError)
	}
}

func (j *redisJob) removeExpiredEntry(key string, value interface{}) bool {

	j.increaseHandledEntry()

	if j.config != nil && j.config.SchedulerConf != nil && j.config.SchedulerConf.PreProcess != nil {
		err := j.config.SchedulerConf.PreProcess(value)
		if err != nil {
			log.Println("Failed while pre-processing before deletion of the element. Error: ", err)
		}
	}

	if j.config.Verbose {
		log.Printf("CacheJob REMOVE - [%s] removeExpiredEntry key: [%v] \n", j.name, key)
	}

	clusterClient, ok := j.cache.(*redis.ClusterClient)
	if !ok {
		log.Println("CacheJob - Failed to convert j.cache to *redis.ClusterClient")
		return false
	}
	clusterClient.Del(context.Background(), key)

	if j.config != nil && j.config.SchedulerConf != nil && j.config.SchedulerConf.PostProcess != nil {
		err := j.config.SchedulerConf.PostProcess(value)
		if err != nil {
			log.Println("Failed while post-processing after deletion of the element. Error: ", err)
		}
	}

	return true
}

func (j *redisJob) increaseTotalExpiredEntry() (total int) {

	j.totalExpired += 1
	return j.totalExpired
}

func (j *redisJob) increaseHandledEntry() (total int) {

	j.handled += 1
	return j.handled
}

func (j *redisJob) handleExpiredEntry(actualKey string) {

	key := strings.Replace(actualKey, getRedisKeyPrefix(j.config.RedisConf), "", 1)
	// To make the expired key have same hash tag, need to add curly braces before and after the actual key
	expiredKey := KEY_PREFIX_EXPIRED + "{" + actualKey + "}"
	// TODO: remove logs
	log.Printf("key: %v, expired key : %v", key, expiredKey)

	lockError := j.keyLock.TryLock(key)
	defer func() {
		if lockError == nil {
			log.Printf("[RedisLock][handleExpiredEntry] Try Unlock: [%s]", key)
			ok, err := j.keyLock.Unlock(key)
			if ok {
				log.Printf("[RedisLock][handleExpiredEntry] Unlocked: [%s]", key)
			} else {
				if err != nil {
					log.Println("[RedisLock][handleExpiredEntry] Error while Unlock. error: ", err)
				} else {
					log.Println("[RedisLock][handleExpiredEntry] Unlock failed without error")
				}
			}
		}
	}()
	//////////////////////////////////

	if lockError == nil {
		// TODO: remove logs
		log.Printf("[RedisLock][handleExpiredEntry] Locked:[%s]", key)

		result, err := j.cache.Rename(context.Background(), actualKey, expiredKey).Result()
		if err != nil {
			log.Printf("Failed to rename key [%v] to [%v]. Error: %v \n", actualKey, expiredKey, err)
		} else if j.config.Verbose {
			log.Printf("Renamed key [%v] to [%v]. Result: %v \n", actualKey, expiredKey, result)
		}
	} else {
		log.Println("[RedisLock][handleExpiredEntry] Failed while locking the key. Skip to next expired key. Error: ", lockError)
	}
}

func getRedisScheduler(cache redis.UniversalClient, config *schema.CacheConf) (scheduler *redisScheduler) {
	if config == nil || config.SchedulerConf == nil {
		return nil
	}
	scheduler = &redisScheduler{
		cron: cron.New(),
		job: redisJob{
			name:         config.Name,
			cache:        cache,
			totalExpired: 0,
			handled:      0,
			config:       config,
			keyLock: NewRedisLockPool(&schema.CacheConf{
				Verbose:            config.Verbose,
				Name:               config.RedisConf.Namespace + "-SchedulerKeyLock",
				CacheDuration:      time.Hour * 168, // 1 week
				RandomizedDuration: false,
				RedisConf: &schema.RedisConf{
					Namespace:       config.RedisConf.Namespace + "-SchedulerKeyLock",
					Group:           "",
					ServerAddresses: config.RedisConf.ServerAddresses,
					Username:        config.RedisConf.Username,
					Password:        config.RedisConf.Password,
				},
				SchedulerConf: &schema.SchedulerConf{
					CronExprForScheduler: config.SchedulerConf.CronExprForScheduler,
					PreProcess:           nil,
					PostProcess:          nil,
				},
			}),
		},
	}

	scheduler.cron.AddJob(config.SchedulerConf.CronExprForScheduler, scheduler.job)
	scheduler.cron.Start()

	scheduler.expiredKeyChannel = make(chan string)
	go func() {
		for actualKey := range scheduler.expiredKeyChannel {
			scheduler.job.handleExpiredEntry(actualKey)
		}
	}()

	// this is telling redis to subscribe to events published in the keyevent channel, specifically for expired events
	// pubSub := cache.PSubscribe(context.Background(), "__keyevent*__:expired")
	clusterClient := cache.(*redis.ClusterClient)
	clusterClient.ForEachMaster(context.Background(), func(ctx context.Context, client *redis.Client) error {

		pubSub := client.PSubscribe(ctx, "__keyevent*__:expired")
		id, err := client.ClientID(ctx).Result()
		if err != nil {
			log.Printf("Failed to get client id. Error: %v", err)
			return err
		}

		name := fmt.Sprintf("pubSub-%d", id)
		ch := pubSub.Channel()

		go func() {
			// infinite loop
			// this listens in the background for messages.
			for msg := range ch {
				// TODO: remove logs
				log.Printf("[%s] received Keyspace event %v, %v, %v \n", name, msg.String(), msg.Channel, msg.Payload)
				if strings.HasPrefix(msg.Payload, getRedisKeyPrefix(config.RedisConf)+KEY_PREFIX_SHADOW) {
					// get the actual key from the shadow key
					actualKey := strings.Replace(msg.Payload, KEY_PREFIX_SHADOW, "", 1)
					log.Printf("[%s] found shadow key : %v, actual key : %v", name, msg.Payload, actualKey)
					scheduler.expiredKeyChannel <- actualKey
				}
			}
		}()

		return nil
	})
	return scheduler
}
