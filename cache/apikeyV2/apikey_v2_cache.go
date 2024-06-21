package apikey

import (
	"sync"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// Cache godoc
type Cache struct {
	keys map[string]*model.UserAPIKeysV2
	lock *sync.RWMutex
}

var cache *Cache

func init() {
	cache = &Cache{
		keys: make(map[string]*model.UserAPIKeysV2),
		lock: &sync.RWMutex{},
	}
}

func Get(token string) (key *model.UserAPIKeysV2, found bool) {
	cache.lock.RLock()
	key, found = cache.keys[token]
	cache.lock.RUnlock()
	return
}

// SetAll godoc
// Update the internal memory api key cache with the new list of keys
func SetAll(keys map[string]*model.UserAPIKeysV2) {
	cache.lock.Lock()
	cache.keys = keys
	cache.lock.Unlock()
}
