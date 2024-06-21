package crons

import (
	"github.com/rs/zerolog/log"
	subAccounts "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

// CronUpdateSubAccountsCache godoc
func CronUpdateSubAccountsCache() {
	repo := queries.GetRepo()

	var accounts []*model.SubAccount
	if err := repo.ConnReader.Find(&accounts).Error; err != nil {
		if err.Error() != "record not found" {
			log.Error().Err(err).Msg("Unable to update sub accounts list")
		}
		return
	}

	if len(accounts) == 0 {
		return
	}
	subAccounts.SetAll(accounts)
}
