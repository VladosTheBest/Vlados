package crons

import (
	"github.com/rs/zerolog/log"
	coins "gitlab.com/paramountdax-exchange/exchange_api_v2/apps/coinsLocal"
	cache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/markets"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

// CronUpdateMarketsCache godoc
func CronUpdateMarketsCache(coin *coins.App) {
	repo := queries.GetRepo()

	markets, err := repo.GetAllMarketMap()
	if err != nil {
		log.Error().Err(err).Msg("Unable to update cached market list")
		return
	}

	newMarkets := make(map[string]*model.Market)
	for symbol, market := range markets {
		newMarkets[symbol] = market
	}

	cache.SetAll(newMarkets, coin)
}
