package lock

import (
	"log"

	"github.com/KiwonKim-git/cachemap/cache"
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
	keyLocks *cache.CacheMap
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
		keyLocks: cache.NewCacheMap(config),
	}
}

func (l *RedisLockPool) getLockByKey(key string) (mu *redsync.Mutex, result schema.RESULT) {

	v, result, _ := l.keyLocks.Load(key)
	if result == schema.VALID && v != nil {
		mu, ok := v.(*redsync.Mutex)
		if ok {
			return mu, result
		}
	}
	// Obtain a new mutex by using the same name for all instances wanting the same lock.
	mu = l.redsync.NewMutex(key)
	l.keyLocks.Store(key, mu, nil)
	return mu, result
}

// Obtain a lock for given key. After this is successful, no one else can obtain the same lock (the same mutex name) until it is unlocked.
func (l *RedisLockPool) Lock(key string) (err error) {
	mu, _ := l.getLockByKey(key)
	return mu.Lock()
}

// Release the lock and then other processes or threads can obtain a lock. Ok will represent the status of unlocking.
func (l *RedisLockPool) Unlock(key string) (ok bool, err error) {

	mu, result := l.getLockByKey(key)
	if result == schema.VALID {
		return mu.Unlock()
	} else if result == schema.NOT_FOUND {
		log.Printf("Unlock - [%s] does not exist but unlock is requested", key)
	} else if result == schema.EXPIRED {
		log.Printf("Unlock - [%s] is expired but unlock is requested", key)
	} else {
		log.Printf("Unlock - [%s] is in error state but unlock is requested", key)
	}
	return false, nil
}
