package lock

import (
	"sync"

	"github.com/KiwonKim-git/cachemap/cache"
	"github.com/KiwonKim-git/cachemap/schema"
)

// Provide a pool which consists of sync.Mutex lock for keys. The lock in this pool is valid within the process.
type LocalKeyLockPool struct {
	keyLocks *cache.CacheMap
}

func NewLocalKeyLockPool(config *schema.CacheConf) (lockPool *LocalKeyLockPool) {
	return &LocalKeyLockPool{keyLocks: cache.NewCacheMap(config)}
}

func (l *LocalKeyLockPool) getLockByKey(key string) (mu *sync.Mutex) {

	v, result, _ := l.keyLocks.Load(key)
	if result == schema.VALID && v != nil {
		mu, ok := v.(*sync.Mutex)
		if ok {
			return mu
		}
	}
	mu = &sync.Mutex{}
	l.keyLocks.Store(key, mu, nil)
	return mu
}

func (l *LocalKeyLockPool) Lock(key string) {
	l.getLockByKey(key).Lock()
}

func (l *LocalKeyLockPool) Unlock(key string) {
	l.getLockByKey(key).Unlock()
}
