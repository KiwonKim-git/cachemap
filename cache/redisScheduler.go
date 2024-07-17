package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/KiwonKim-git/cachemap/schema"
	"github.com/KiwonKim-git/cachemap/util"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron"
)

type redisScheduler struct {
	cron              *cron.Cron
	job               redisJob
	expiredKeyChannel chan string
}

type redisJob struct {
	name              string
	cache             redis.UniversalClient
	expiredKeyPattern string
	totalExpired      int
	handled           int
	config            *schema.CacheConf
	keyLock           *RedisLockPool
}

func (j redisJob) Run() {

	loc := time.FixedZone("KST", 9*60*60)
	now := time.Now()

	j.totalExpired = 0
	j.handled = 0

	if j.cache != nil {

		clusterClient, ok := j.cache.(*redis.ClusterClient)
		if !ok {
			j.config.Logger.PrintLogs(util.ERROR, "CacheJob - Failed to convert j.cache to *redis.ClusterClient")
			return
		}
		err := clusterClient.ForEachMaster(context.Background(), func(ctx context.Context, client *redis.Client) error {
			iter := client.Scan(ctx, 0, j.expiredKeyPattern, 0).Iterator()

			for iter.Next(ctx) {
				j.iterate(iter.Val())
			}
			return iter.Err()
		})
		if err != nil {
			j.config.Logger.PrintLogs(util.ERROR, fmt.Sprint("CacheJob - Failed while scanning keys in Redis. Error: ", err))
			return
		}
	} else {
		j.config.Logger.PrintLogs(util.ERROR, fmt.Sprintf("CacheJob - [%s] runs but, cache is nil", j.name))
		return
	}

	j.config.Logger.PrintLogs(util.ERROR, fmt.Sprintf("CacheJob - [%s] runs at [%s], TotalExpired: [%d], Handled: [%d]", j.name, now.In(loc).Format(time.RFC3339), j.totalExpired, j.handled))
}

func (j *redisJob) iterate(expiredKey string) {

	j.increaseTotalExpiredEntry()

	actualKey := strings.Replace(expiredKey, KEY_PREFIX_EXPIRED+"{", "", 1)
	actualKey = strings.Replace(actualKey, "}", "", 1)

	lockError := j.keyLock.tryLock(actualKey)
	defer j.keyLock.unlock(actualKey, lockError)

	if lockError == nil {
		clusterClient, ok := j.cache.(*redis.ClusterClient)
		if !ok {
			j.config.Logger.PrintLogs(util.ERROR, fmt.Sprint(fmt.Errorf("CacheRedis ERROR - [%s] failed to convert the client to *redis.ClusterClient", j.config.Name)))
			return
		}

		value, result, _, err := getValueFromRedis(context.Background(), clusterClient, expiredKey, j.config)

		if err != nil {
			j.config.Logger.PrintLogs(util.ERROR, fmt.Sprint("CacheJob - Failed while getting value from Redis. Error: ", err))
		} else if result == schema.EXPIRED && value != nil {
			j.removeExpiredEntry(expiredKey, value)
		} else if value != nil {
			j.config.Logger.PrintLogs(util.ERROR,
				fmt.Sprintf("CacheJob - the [%s] actual key is not expired yet (status : %v) but, why is this key moved to EXPIRED namespace? anyway remove it.", actualKey, result.String()))
			j.removeExpiredEntry(expiredKey, value)
		} else {
			j.config.Logger.PrintLogs(util.ERROR,
				fmt.Sprintf("CacheJob - Why is the [%s] actual key moved to EXPIRED namespace? expired key : [%s]", actualKey, expiredKey))
		}
	} else {
		j.config.Logger.PrintLogs(util.ERROR, fmt.Sprint("[RedisLock][Iterate] Failed while locking the key. Skip to next expired key. Error: ", lockError))
	}
}

func (j *redisJob) removeExpiredEntry(key string, value interface{}) bool {

	j.increaseHandledEntry()

	if j.config != nil && j.config.SchedulerConf != nil && j.config.SchedulerConf.PreProcess != nil {
		err := j.config.SchedulerConf.PreProcess(value)
		if err != nil {
			j.config.Logger.PrintLogs(util.ERROR, fmt.Sprint("Failed while pre-processing before deletion of the element. Error: ", err))
		}
	}

	j.config.Logger.PrintLogs(util.DEBUG, fmt.Sprintf("CacheJob REMOVE - [%s] removeExpiredEntry key: [%v] \n", j.name, key))

	clusterClient, ok := j.cache.(*redis.ClusterClient)
	if !ok {
		j.config.Logger.PrintLogs(util.ERROR, "CacheJob - Failed to convert j.cache to *redis.ClusterClient")
		return false
	}
	clusterClient.Del(context.Background(), key)

	if j.config != nil && j.config.SchedulerConf != nil && j.config.SchedulerConf.PostProcess != nil {
		err := j.config.SchedulerConf.PostProcess(value)
		if err != nil {
			j.config.Logger.PrintLogs(util.ERROR, fmt.Sprint("Failed while post-processing after deletion of the element. Error: ", err))
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

	// To make the expired key have same hash tag, need to add curly braces before and after the actual key
	expiredKey := KEY_PREFIX_EXPIRED + "{" + actualKey + "}"

	lockError := j.keyLock.tryLock(actualKey)
	defer j.keyLock.unlock(actualKey, lockError)

	if lockError == nil {
		result, err := j.cache.Rename(context.Background(), actualKey, expiredKey).Result()
		if err != nil {
			j.config.Logger.PrintLogs(util.ERROR, fmt.Sprintf("Failed to rename key [%v] to [%v]. Error: %v \n", actualKey, expiredKey, err))
		} else {
			j.config.Logger.PrintLogs(util.DEBUG, fmt.Sprintf("Renamed key [%v] to [%v]. Result: %v \n", actualKey, expiredKey, result))
		}

	} else {
		j.config.Logger.PrintLogs(util.ERROR, fmt.Sprint("[RedisLock][handleExpiredEntry] Failed while locking the key. Skip to next expired key. Error: ", lockError))
	}
}

func getRedisScheduler(cache redis.UniversalClient, config *schema.CacheConf) (scheduler *redisScheduler) {
	if config == nil || config.SchedulerConf == nil {
		return nil
	}
	scheduler = &redisScheduler{
		cron: cron.New(),
		job: redisJob{
			name:              config.Name,
			cache:             cache,
			expiredKeyPattern: KEY_PREFIX_EXPIRED + "{" + getRedisKeyPrefix(config.RedisConf) + "*",
			totalExpired:      0,
			handled:           0,
			config:            config,
			keyLock: NewRedisLockPool(&schema.CacheConf{
				Name:               config.RedisConf.Namespace,
				CacheDuration:      time.Hour * 168, // 1 week
				RandomizedDuration: false,
				RedisConf: &schema.RedisConf{
					Namespace:       config.RedisConf.Namespace,
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
				Logger: config.Logger,
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
			scheduler.job.config.Logger.PrintLogs(util.ERROR, fmt.Sprintf("Failed to get client id. Error: %v", err))
			return err
		}

		name := fmt.Sprintf("pubSub-%d", id)
		ch := pubSub.Channel()

		go func() {
			// infinite loop
			// this listens in the background for messages.
			for msg := range ch {
				// // For debug
				// log.Printf("[%s] received Keyspace event. Channel: [%v], Payload: [%v] \n", name, msg.Channel, msg.Payload)
				// ////////////////////
				if strings.HasPrefix(msg.Payload, getRedisKeyPrefix(config.RedisConf)+KEY_PREFIX_SHADOW) {
					// get the actual key from the shadow key
					actualKey := strings.Replace(msg.Payload, KEY_PREFIX_SHADOW, "", 1)
					scheduler.job.config.Logger.PrintLogs(util.ERROR, fmt.Sprintf("[%s] found shadow key: [%v], actual key: [%v]", name, msg.Payload, actualKey))
					scheduler.expiredKeyChannel <- actualKey
				}
			}
		}()

		return nil
	})
	if scheduler != nil {
		scheduler.job.config.Logger.PrintLogs(util.ERROR,
			fmt.Sprintf("CacheRedisScheduler CREATE - [%s] has been created. expired key pattern: [%s]", scheduler.job.name, scheduler.job.expiredKeyPattern))
	}
	return scheduler
}
