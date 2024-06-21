package crons

import (
	"github.com/rs/zerolog/log"
	cache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/last_prices"
	loader "gitlab.com/paramountdax-exchange/exchange_api_v2/lastprices"
)

// CronUpdateLastPricesCache godoc
func CronUpdateLastPricesCache() {
	lastPrices, err := loader.GetLoader().GetLastPrices()
	if err != nil {
		log.Debug().Err(err).Msg("Unable to update cached last prices list")
		return
	}
	cache.SetAll(lastPrices)
}
