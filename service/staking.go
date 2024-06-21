package service

import (
	"errors"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/userbalance"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

func (service *Service) CreateStaking(userId uint64, data *model.CreateStakingRequest, fromSubAccount *model.SubAccount) error {

	logger := log.With().
		Str("service", "staking").
		Str("method", "CreateStaking").
		Uint64("userId", userId).
		Uint64("fromSubAccoutID", fromSubAccount.ID).
		Interface("data", data).
		Logger()

	tx := service.repo.Conn.Begin()

	period := model.StakingPeriod(data.Period)
	if !period.IsValid() {
		tx.Rollback()
		logger.Error().Msg("wrong period")
		return errors.New("wrong period")
	}

	fromFundsAccount, err := service.FundsEngine.GetAccountBalances(userId, fromSubAccount.ID)
	if err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("unable to get funds account")
		return err
	}

	fromFundsAccount.LockAccount()
	defer fromFundsAccount.UnlockAccount()

	fundsInContractCoinAvailable, err := fromFundsAccount.GetAvailableBalanceForCoin(data.CoinSymbol)
	if err != nil {
		tx.Rollback()
		logger.Error().Err(err).Str("coin", data.CoinSymbol).Msg("unable to get available funds")
		return err
	}

	if fundsInContractCoinAvailable.Cmp(data.Amount) == -1 {
		tx.Rollback()
		return errors.New("insufficient funds")
	}

	toSubAccount, err := service.ops.CreateSubAccount(tx,
		userId,
		model.AccountGroupStaking,
		model.MarketTypeSpot,
		false,
		false,
		false,
		false,
		"Staking Account",
		"",
		model.SubAccountStatusActive,
	)

	if err != nil {
		logger.Error().Err(err).Msg("unable to create subAccount for bot")
		return err
	}

	logger = logger.With().Uint64("toSubAccountID", toSubAccount.ID).Logger()

	toFundsAccount, err := service.FundsEngine.GetAccountBalances(userId, toSubAccount.ID)
	if err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("unable to get funds account")
		return err
	}

	toFundsAccount.LockAccount()
	defer toFundsAccount.UnlockAccount()

	periodsMap := service.cfg.Staking.GetPeriodsMap()
	periodConfig := periodsMap[data.Period]

	_, _, err = service.ops.CreateStaking(tx, userId, data, period, periodConfig, fromFundsAccount, toFundsAccount)
	if err != nil {
		logger.Error().Err(err).Msg("unable to deposit to the bonus account")
		return err
	}

	if err := tx.Commit().Error; err != nil {
		logger.Error().Err(err).
			Msg("unable to commit transaction")
		return err
	}

	subAccounts.Set(toSubAccount, false)

	userbalance.SetWithPublish(userId, fromSubAccount.ID)
	userbalance.SetWithPublish(userId, toSubAccount.ID)

	return nil
}

func (service *Service) ListStakingEarnings(userID uint64, stakingID uint64, status *model.StakingEarningStatus, limit, page int) ([]model.StakingEarning, error) {
	result := make([]model.StakingEarning, 0)
	var err error
	var db = service.repo.ConnReader.Table("staking_earnings")

	db = db.Where("user_id = ?", userID)

	if stakingID != 0 {
		db = db.Where("staking_id = ?", stakingID)
	}

	if status != nil {
		db = db.Where("status = ?", status)
	}

	db = db.Order("created_at DESC")
	if limit == 0 {
		db = db.Find(&result)
	} else {
		db = db.Limit(limit).Offset((page - 1) * limit).Find(&result)
	}
	err = db.Error
	return result, err
}
