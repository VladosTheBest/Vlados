package coins

import (
	"errors"
	"sync"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// Cache godoc
type Cache struct {
	// the local cache of all coins
	coins map[string]*model.Coin
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
		coins: make(map[string]*model.Coin),
		lock:  &sync.RWMutex{},
	}
}

// Get a coin by symbol from the cache
func Get(symbol string) (coin *model.Coin, err error) {
	var ok bool
	cache.lock.RLock()
	if coin, ok = cache.coins[symbol]; !ok {
		err = ErrNotFound
	}
	cache.lock.RUnlock()
	return coin, err
}

// GetAll markets from the cache
func GetAll() (coins []*model.Coin) {
	cache.lock.RLock()
	coins = make([]*model.Coin, 0)
	for i := range cache.coins {
		coins = append(coins, cache.coins[i])
	}
	cache.lock.RUnlock()
	return coins
}

// GetAll Active markets from the cache
func GetAllActive(status string) (coins []*model.Coin, err error) {
	cache.lock.RLock()
	coins = make([]*model.Coin, 0)
	if len(status) == 0 {
		for i := range cache.coins {
			if cache.coins[i].Status == model.CoinStatusActive {
				coins = append(coins, cache.coins[i])
			}
		}
	} else {
		coinStatus, err := model.GetCoinStatusFromString(status)
		if err != nil {
			cache.lock.RUnlock()
			return nil, err
		}
		for i := range cache.coins {
			if cache.coins[i].Status == coinStatus {
				coins = append(coins, cache.coins[i])
			}
		}
	}

	cache.lock.RUnlock()
	return coins, nil
}

// Set godoc
// Update a single coin in the cache
func Set(symbol string, coin *model.Coin) {
	cache.lock.Lock()
	cache.coins[symbol] = coin
	cache.lock.Unlock()
}

// SetAll godoc
// Update the internal memory coin cache with the new list of coins
func SetAll(coins map[string]*model.Coin) {
	cache.lock.Lock()
	cache.coins = coins
	cache.lock.Unlock()
}
