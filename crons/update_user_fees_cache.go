package crons

import (
	"github.com/rs/zerolog/log"
	cache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/user_fees"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

// CronUpdateUserFeesCache godoc
func CronUpdateUserFeesCache() {
	repo := queries.GetRepo()

	var fees []*model.UserFee
	if err := repo.ConnReader.Find(&fees).Error; err != nil {
		if err.Error() != "record not found" {
			log.Error().Err(err).Msg("Unable to update sub accounts list")
		}
		return
	}

	if len(fees) == 0 {
		return
	}
	cache.SetAll(fees)
}
