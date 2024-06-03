package lock

import (
	"log"
	"sync"

	"github.com/KiwonKim-git/cachemap/cache"
	"github.com/KiwonKim-git/cachemap/schema"
)

// Provide a pool which consists of sync.Mutex lock for keys. This pool is implemented based on cacheMap and the lock in this pool is valid within the process (local).
type MutexPool struct {
	keyLocks *cache.CacheMap
}

func NewMutexPool(config *schema.CacheConf) (lockPool *MutexPool) {
	if config == nil {
		log.Println("NewMapKeyLockPool is called with nil config")
		return nil
	}
	return &MutexPool{keyLocks: cache.NewCacheMap(config)}
}

func (l *MutexPool) getLockByKey(key string) (mu *sync.Mutex, result schema.RESULT) {

	v, result, _ := l.keyLocks.Load(key)
	if result == schema.VALID && v != nil {
		mu, ok := v.(*sync.Mutex)
		if ok {
			return mu, result
		}
	}
	mu = &sync.Mutex{}
	l.keyLocks.Store(key, mu, nil)
	return mu, result
}

func (l *MutexPool) Lock(key string) {
	mu, _ := l.getLockByKey(key)
	mu.Lock()
}

func (l *MutexPool) Unlock(key string) {
	mu, result := l.getLockByKey(key)
	if result == schema.VALID {
		mu.Unlock()
	} else if result == schema.NOT_FOUND {
		log.Printf("Unlock - [%s] does not exist but unlock is requested", key)
	} else if result == schema.EXPIRED {
		log.Printf("Unlock - [%s] is expired but unlock is requested", key)
	} else {
		log.Printf("Unlock - [%s] is in error state but unlock is requested", key)
	}
}
