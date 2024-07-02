package cache

import (
	"fmt"
	"log"

	"github.com/KiwonKim-git/cachemap/schema"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"
)

// Provide a pool which consists of redsync.Mutex lock for keys. This pool is implemented based on Redsync, CacheMap and Redis Cluster.
// The lock in this pool is valid within the same redis cluster (distributed).
// Redsync is a Redis-based distributed mutual exclusion lock implementation for Go as a library.
// CacheMap is used for caching redsync.Mutex locks to avoid from creating the same lock multiple times.
// Redis Cluster is used for storing the lock information.
type RedisLockPool struct {
	redsync  *redsync.Redsync
	keyLocks *CacheMap
	config   *schema.CacheConf
}

func NewRedisLockPool(config *schema.CacheConf) (lockPool *RedisLockPool) {

	if config == nil {
		log.Println("NewRedisKeyLockPool is called with nil config")
		return nil
	}
	if config.RedisConf == nil {
		log.Println("NewRedisKeyLockPool is called with nil RedisConf")
		return nil
	}
	// Create a pool with go-redis which is the pool redisync will use while communicating with Redis. This can also be any pool that implements the `redis.Pool` interface.
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		ClientName: config.Name,
		Addrs:      config.RedisConf.ServerAddresses,
		Username:   config.RedisConf.Username, // no username specified
		Password:   config.RedisConf.Password, // no password set
	})

	pool := goredis.NewPool(client)

	// Create an instance of redisync to be used to obtain a mutual exclusion lock.
	rs := redsync.New(pool)

	return &RedisLockPool{
		redsync:  rs,
		keyLocks: NewCacheMap(config),
		config:   config,
	}
}

func (l *RedisLockPool) getLockByKey(actualKey string) (mu *redsync.Mutex, result schema.RESULT) {

	v, result, _ := l.keyLocks.Load(actualKey)
	if result == schema.VALID && v != nil {
		mu, ok := v.(*redsync.Mutex)
		if ok {
			return mu, result
		}
	}
	// Obtain a new mutex by using the same name for all instances wanting the same lock.
	mu = l.redsync.NewMutex(actualKey)
	l.keyLocks.Store(actualKey, mu, nil)
	return mu, result
}

// Obtain a lock for given key. After this is successful, no one else can obtain the same lock (the same mutex name) until it is unlocked.
func (l *RedisLockPool) Lock(key string) (err error) {
	actualKey := getRedisKeyPrefix(l.config.RedisConf) + key
	return l.lock(actualKey)
}

func (l *RedisLockPool) lock(actualKey string) (err error) {
	mu, _ := l.getLockByKey(actualKey)

	// TODO: remove logs
	log.Printf("[RedisLock] Lock: [%s]", actualKey)
	err = mu.Lock()
	if err == nil {
		log.Printf("[RedisLock] Locked: [%s]", actualKey)
	} else {
		log.Printf("[RedisLock] Error while Lock: [%s], error : %v", actualKey, err)
	}
	return err
	///////////////////

	//return mu.Lock()
}

// Obtain a lock for given key. After this is successful, no one else can obtain the same lock (the same mutex name) until it is unlocked.
// And also TryLock only attempts to lock m once and returns immediately regardless of success or failure without retrying.
func (l *RedisLockPool) TryLock(key string) (err error) {
	actualKey := getRedisKeyPrefix(l.config.RedisConf) + key
	return l.tryLock(actualKey)
}

func (l *RedisLockPool) tryLock(actualKey string) (err error) {
	mu, _ := l.getLockByKey(actualKey)

	// TODO: remove logs
	log.Printf("[RedisLock] Try Lock: [%s]", actualKey)
	err = mu.TryLock()
	if err == nil {
		log.Printf("[RedisLock] Locked: [%s]", actualKey)
	} else {
		log.Printf("[RedisLock] Error while Try Lock: [%s], error : %v", actualKey, err)
	}
	return err
	///////////////////

	//return mu.TryLock()
}

// Release the lock and then other processes or threads can obtain a lock. Ok will represent the status of unlocking.
func (l *RedisLockPool) Unlock(key string) (ok bool, err error) {

	actualKey := getRedisKeyPrefix(l.config.RedisConf) + key
	return l.unlock(actualKey)
}

func (l *RedisLockPool) unlock(actualKey string) (ok bool, err error) {

	ok = false
	err = nil

	mu, result := l.getLockByKey(actualKey)

	// TODO: remove logs
	log.Printf("[RedisLock] Unlock: [%s]", actualKey)
	//////////////
	if result == schema.VALID {
		ok, err = mu.Unlock()
		if ok {
			// TODO: remove logs
			log.Printf("[RedisLock] Unlocked: [%s]", actualKey)
			//////////////////
		} else {
			if err != nil {
				log.Printf("[RedisLock] Unlock - Error while Unlock. key: [%s] error: %v", actualKey, err)
			} else {
				log.Println("[RedisLock] Unlock - Unlock failed without error")
			}
		}
	} else if result == schema.NOT_FOUND {
		err = fmt.Errorf("[RedisLock] Unlock - [%s] does not exist but unlock is requested", actualKey)
	} else if result == schema.EXPIRED {
		err = fmt.Errorf("[RedisLock] Unlock - [%s] is expired but unlock is requested", actualKey)
	} else {
		err = fmt.Errorf("[RedisLock] Unlock - [%s] is in error state but unlock is requested", actualKey)
	}
	return ok, err
}
