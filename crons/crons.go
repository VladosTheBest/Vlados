package crons

import (
	"github.com/robfig/cron"
	coins "gitlab.com/paramountdax-exchange/exchange_api_v2/apps/coinsLocal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/config"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

var cronService *cron.Cron

// Start Initiate the crons based on the given configuration file
func Start(crons config.Crons, repo *queries.Repo, coin *coins.App) {
	cronService = cron.New()
	for id, schedule := range crons {
		callback := GetCronByID(id, repo, coin)
		// @todo CH: eventually handle the error if the cron can't be created
		_ = cronService.AddFunc(schedule, callback)
		// call the caching functions at least once at startup to init caching
		if id != "system_revert_frozen_orders" && id != "update_balance24h_stats" {
			callback()
		}
	}
	cronService.Start()
}

// GetCronByID get a function to execute basef on the id
func GetCronByID(id string, repo *queries.Repo, coinsApp *coins.App) func() {
	switch id {
	case "system_revert_frozen_orders":
		return CronSystemRevertFrozenOrder
	case "update_auth_cache":
		return CronUpdateAuthCache
	case "update_markets_cache":
		return func() {
			CronUpdateMarketsCache(coinsApp)
		}
	case "update_coins_cache":
		return CronUpdateCoinsCache
	case "update_apikeys_cache":
		return CronUpdateAPIKeysCache
	case "update_user_fees_cache":
		return CronUpdateUserFeesCache
	case "update_last_prices_cache":
		return CronUpdateLastPricesCache
	case "update_sub_accounts_cache":
		return CronUpdateSubAccountsCache
	case "update_balance24h_stats":
		return (func() {
			CronUpdateBalance24h(repo, coinsApp)
		})
	case "update_stakings_status":
		return CronUpdateStakingStatus
	}
	return (func() {})
}

// Close godoc
func Close() {
	cronService.Stop()
	close(SystemRevertOrderChan)
}
