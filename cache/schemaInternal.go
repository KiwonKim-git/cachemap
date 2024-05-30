package cache

import "time"

// struct to store any element in map
type elementForCache struct {
	// Entry in cache will be expired at this time
	expireAt time.Time
	// Last updated time of the entry in cache
	lastUpdated time.Time
	// Actual value in cache
	value interface{}
}
