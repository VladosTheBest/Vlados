package markets

import (
	"errors"
	coins "gitlab.com/paramountdax-exchange/exchange_api_v2/apps/coinsLocal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"strings"
	"sync"
)

// Cache godoc
type Cache struct {
	// the local cache of all markets
	markets map[string]*model.Market
	// read write lock on data
	// References:
	// - https://golang.org/pkg/sync/#RWMutex
	// - https://medium.com/golangspec/sync-rwmutex-ca6c6c3208a0
	lock *sync.RWMutex
}

// cache instance that keeps the list of markets in the system
var cache *Cache

// ErrNotFound godoc
var ErrNotFound = errors.New("Not Found")

func init() {
	cache = &Cache{
		markets: map[string]*model.Market{},
		lock:    &sync.RWMutex{},
	}
}

// Get a market by symbol from the cache
func Get(symbol string) (market *model.Market, err error) {
	var ok bool
	cache.lock.RLock()
	if market, ok = cache.markets[symbol]; !ok {
		err = ErrNotFound
	}
	cache.lock.RUnlock()
	return market, err
}

// GetAll markets from the cache
func GetAll() (markets []*model.Market) {
	cache.lock.RLock()
	markets = []*model.Market{}
	for i := range cache.markets {
		markets = append(markets, cache.markets[i])
	}
	cache.lock.RUnlock()
	return markets
}

// GetAllActive markets from the cache
func GetAllActive() (markets []*model.Market) {
	cache.lock.RLock()
	markets = make([]*model.Market, 0)
	for i := range cache.markets {
		if cache.markets[i].Status == model.MarketStatusActive {
			markets = append(markets, cache.markets[i])
		}
	}
	cache.lock.RUnlock()
	return markets
}

// Set godoc
// Update a single market in the cache
func Set(market *model.Market, coin *coins.App) {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	cache.markets[market.ID] = market
	if err := calculateMarket(market, coin); err != nil {
		//log.Debug().Err(err).
		//	Str("from", market.MarketCoinSymbol).
		//	Str("to", "USDT").
		//	Msg("Unable to set markets cross")
	}
}

// SetAll godoc
// Update the internal memory market cache with the new list of markets
func SetAll(markets map[string]*model.Market, coin *coins.App) {
	for _, market := range markets {
		Set(market, coin)
	}
}

func calculateMarket(market *model.Market, coin *coins.App) error {
	crossRates, err := coin.GetAll()

	if err != nil {
		return err
	}

	minMarketCrossRate, ok := crossRates[strings.ToUpper(market.MarketCoinSymbol)]["USDT"]
	if !ok {
		return errors.New("unable to get market cross rate")
	}

	minMarket := conv.NewDecimalWithPrecision().Quo(market.MinMarketVolume.V, minMarketCrossRate)
	minQuote := conv.NewDecimalWithPrecision().Quo(market.MinQuoteVolume.V, minMarketCrossRate)

	market.SetCross(minMarket, minQuote)

	return nil
}

func SetHighlight(marketID string, highlight bool) bool {
	cache.lock.RLock()
	defer cache.lock.RUnlock()

	if _, ok := cache.markets[marketID]; !ok {
		return false
	}
	cache.markets[marketID].Highlight = highlight

	return true
}
