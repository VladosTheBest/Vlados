package user_fees

import (
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"sync"
)

// Cache godoc
type Cache struct {
	fees map[uint64]*model.UserFee
	lock *sync.RWMutex
}

// cache instance that keeps the list of api keys in the system
var cache *Cache

func init() {
	cache = &Cache{
		fees: make(map[uint64]*model.UserFee),
		lock: &sync.RWMutex{},
	}
}

// Get a key by prefix from the cache
func Get(userID uint64) (fees *model.UserFee, found bool) {
	cache.lock.RLock()
	fees, found = cache.fees[userID]
	cache.lock.RUnlock()
	return fees, found
}

// Set godoc
func Set(fees *model.UserFee) {
	cache.lock.Lock()
	cache.fees[fees.UserID] = fees
	cache.lock.Unlock()
}

// SetAll godoc
func SetAll(fees []*model.UserFee) {
	cache.lock.Lock()
	for _, feesItem := range fees {
		cache.fees[feesItem.UserID] = feesItem
	}
	cache.lock.Unlock()
}
