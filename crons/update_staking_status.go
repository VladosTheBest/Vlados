package crons

import (
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

func CronUpdateStakingStatus() {
	repo := queries.GetRepo()

	err := repo.Conn.Table("stakings").
		Where("expired_at <= now()").
		Update("status", model.StakingStatusExpired).
		Error

	if err != nil && err.Error() != "record not found" {
		log.Error().Err(err).Msg("Unable to update sub accounts list")
	}

	return
}
