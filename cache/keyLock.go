package cache

import "sync"

type StringKeyLock struct {
	keyLocks map[string]*sync.Mutex // map of mutexes for each key
	mapLock  sync.Mutex             // to make the map safe concurrently
}

func NewStringKeyLock() *StringKeyLock {
	return &StringKeyLock{keyLocks: make(map[string]*sync.Mutex)}
}

func (l *StringKeyLock) getLockBy(key string) *sync.Mutex {
	l.mapLock.Lock()
	defer l.mapLock.Unlock()

	ret, found := l.keyLocks[key]
	if found {
		return ret
	}

	ret = &sync.Mutex{}
	l.keyLocks[key] = ret
	return ret
}

func (l *StringKeyLock) Lock(key string) {
	l.getLockBy(key).Lock()
}

func (l *StringKeyLock) Unlock(key string) {
	l.getLockBy(key).Unlock()
}
