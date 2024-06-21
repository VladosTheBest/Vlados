package service

import (
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

func (service *Service) CreateCardPaymentAccount(userId uint64) (*model.SubAccount, error) {

	logger := log.With().
		Str("service", "cardPayment").
		Str("method", "CreateCardPaymentAccount").
		Uint64("userId", userId).
		Logger()

	tx := service.repo.Conn.Begin()

	cardPaymentSubAccount, err := service.ops.CreateSubAccount(tx,
		userId,
		model.AccountGroupCardPayment,
		model.MarketTypeSpot,
		true,
		true,
		true,
		false,
		"Card Payment Account",
		"",
		model.SubAccountStatusActive,
	)

	if err != nil {
		logger.Error().Err(err).Msg("unable to create subAccount for bot")
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		logger.Error().Err(err).
			Msg("unable to commit transaction")
		return nil, err
	}

	subAccounts.Set(cardPaymentSubAccount, false)
	service.FundsEngine.InitAccountBalances(cardPaymentSubAccount, false)

	return cardPaymentSubAccount, nil
}
