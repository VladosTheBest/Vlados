package crons

import (
	"github.com/rs/zerolog/log"
	cache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/coins"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

// CronUpdateCoinsCache godoc
func CronUpdateCoinsCache() {
	repo := queries.GetRepo()

	coins, err := repo.GetAllCoinsMap()
	if err != nil {
		log.Error().Err(err).Msg("Unable to update cached coin list")
		return
	}
	newCoins := make(map[string]*model.Coin)
	for symbol, coin := range coins {
		newCoins[symbol] = coin
	}
	cache.SetAll(newCoins)
}
