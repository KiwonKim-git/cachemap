package cache

import (
	"fmt"
	"sync"

	"github.com/KiwonKim-git/cachemap/schema"
	"github.com/KiwonKim-git/cachemap/util"
)

// Provide a pool which consists of sync.Mutex lock for keys. This pool is implemented based on cacheMap and the lock in this pool is valid within the process (local).
type MutexPool struct {
	keyLocks *CacheMap
}

func NewMutexPool(config *schema.CacheConf) (lockPool *MutexPool) {
	if config == nil {
		util.Default().PrintLogs(util.ERROR, "NewMapKeyLockPool is called with nil config")
		return nil
	}
	return &MutexPool{keyLocks: NewCacheMap(config)}
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
		l.keyLocks.cacheConfig.Logger.PrintLogs(util.ERROR, fmt.Sprintf("Unlock - [%s] does not exist but unlock is requested", key))
	} else if result == schema.EXPIRED {
		l.keyLocks.cacheConfig.Logger.PrintLogs(util.ERROR, fmt.Sprintf("Unlock - [%s] is expired but unlock is requested", key))
	} else {
		l.keyLocks.cacheConfig.Logger.PrintLogs(util.ERROR, fmt.Sprintf("Unlock - [%s] is in error state but unlock is requested", key))
	}
}
