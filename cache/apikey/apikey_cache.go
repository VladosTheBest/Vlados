package apikey

/**
 * API Key Cache
 * =============
 * Stores a list of the most used API keys in memory for easy access
 * to ensure faster response rate for key verification and a lower db load.
 */

import (
	"sync"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// Cache godoc
type Cache struct {
	// the local cache of most used api keys
	keys    map[string]*model.UserAPIKeys
	decoded map[string]string
	// read write lock on data
	// References:
	// - https://golang.org/pkg/sync/#RWMutex
	// - https://medium.com/golangspec/sync-rwmutex-ca6c6c3208a0
	lock *sync.RWMutex
}

// cache instance that keeps the list of api keys in the system
var cache *Cache

func init() {
	cache = &Cache{
		keys:    make(map[string]*model.UserAPIKeys),
		decoded: make(map[string]string),
		lock:    &sync.RWMutex{},
	}
}

// Get a key by prefix from the cache
func Get(prefix string) (key *model.UserAPIKeys, decoded string, found, isDecoded bool) {
	cache.lock.RLock()
	key, found = cache.keys[prefix]
	decoded, isDecoded = cache.decoded[prefix]
	cache.lock.RUnlock()
	return
}

// Set godoc
// Update a single key in the cache
func Set(prefix string, key *model.UserAPIKeys) {
	cache.lock.Lock()
	cache.keys[prefix] = key
	delete(cache.decoded, prefix)
	cache.lock.Unlock()
}

// SetDecoded godoc
// Mark the api key with the given prefix as verified with the given decoded api key
// This allows the system to not need to rehash every api key received against the one received in the system
func SetDecoded(prefix string, decoded string) (ok bool) {
	cache.lock.Lock()
	// check that the associated key exists in the current generation
	if _, ok = cache.keys[prefix]; ok {
		// set it's decoded value in the hash table
		cache.decoded[prefix] = decoded
	}
	cache.lock.Unlock()
	return
}

// SetAll godoc
// Update the internal memory api key cache with the new list of keys
func SetAll(keys map[string]*model.UserAPIKeys) {
	decoded := make(map[string]string)
	cache.lock.Lock()
	cache.keys = keys
	cache.decoded = decoded
	cache.lock.Unlock()
}
