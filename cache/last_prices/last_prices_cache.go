package last_prices

import (
	"errors"
	"github.com/ericlagergren/decimal"
	"sync"
)

type LastPricesData map[string]decimal.Big

// Cache godoc
type Cache struct {
	// the local cache of all coins
	coins LastPricesData
	// read write lock on data
	// References:
	// - https://golang.org/pkg/sync/#RWMutex
	// - https://medium.com/golangspec/sync-rwmutex-ca6c6c3208a0
	lock *sync.RWMutex
}

// cache instance that keeps the list of coins in the system
var cache *Cache

// ErrNotFound godoc
var ErrNotFound = errors.New("Not Found")

func init() {
	cache = &Cache{
		coins: LastPricesData{},
		lock:  &sync.RWMutex{},
	}
}

// Get a last price for market by symbol from the cache
func Get(symbol string) (price decimal.Big, err error) {
	var ok bool
	cache.lock.RLock()
	if price, ok = cache.coins[symbol]; !ok {
		err = ErrNotFound
	}
	cache.lock.RUnlock()
	return price, err
}

// SetAll godoc
// Update the internal memory last prices cache with the new list of last prices
func SetAll(prices map[string]decimal.Big) {
	cache.lock.Lock()
	cache.coins = prices
	cache.lock.Unlock()
}
