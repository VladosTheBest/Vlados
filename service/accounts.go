package service

import (
	"fmt"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"

	"github.com/ericlagergren/decimal"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/fms"
)

func (service *Service) CreateSubAccount(subAccountData *model.SubAccount) (*model.SubAccount, error) {
	tx := service.repo.Conn.Begin()

	toSubAccount, err := service.ops.CreateSubAccount(tx,
		subAccountData.UserId,
		subAccountData.AccountGroup,
		subAccountData.MarketType,
		subAccountData.DepositAllowed,
		subAccountData.WithdrawalAllowed,
		subAccountData.TransferAllowed,
		subAccountData.IsDefault,
		subAccountData.Title,
		subAccountData.Comment,
		model.SubAccountStatusActive,
	)

	if err != nil {
		log.Info().
			Str("section", "service").
			Str("action", "CreateSubAccount").
			Msg("can't create sub account")
		tx.Rollback()
		return nil, err
	}

	_, err = service.ops.FundsEngine.InitAccountBalances(toSubAccount, false)
	if err != nil {
		log.Info().
			Str("section", "service").
			Str("action", "CreateSubAccount").
			Msg("can't init balances for sub account")
		tx.Rollback()
		return nil, err
	}

	// If the new sub-account is set as default, update the default sub-account in the cache
	if subAccountData.IsDefault {
		if err := subAccounts.SetUserSubAccountDefault(subAccountData.MarketType, subAccountData.UserId, subAccountData.AccountGroup, toSubAccount); err != nil {
			log.Info().
				Str("section", "service").
				Str("action", "CreateSubAccount").
				Msg("can't update default sub account in cache")
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		log.Error().Err(err).Msg("Failed to commit transaction")
		return nil, err
	}

	return toSubAccount, nil
}

func (service *Service) CountSubAccounts(userID uint64) (int64, error) {
	var count int64
	err := service.repo.Conn.Model(&model.SubAccount{}).Where("user_id = ?", userID).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (service *Service) TransferBetweenSubAccounts(userID uint64, amount *decimal.Big, coinSymbol string, accountFrom, accountTo *fms.AccountBalances) error {
	tx := service.repo.Conn.Begin()

	err := service.ops.TransferSubAccounts(tx, userID, amount, coinSymbol, accountFrom, accountTo)
	if err != nil {
		log.Info().
			Str("section", "service").
			Str("action", "TransferBetweenSubAccounts").
			Msg("can't transfer between sub accounts")
		return err
	}

	return nil
}

func (service *Service) WithdrawFromSubAccount(userID uint64, amount *decimal.Big, coinSymbol string, accountFrom *fms.AccountBalances) error {
	tx := service.repo.Conn.Begin()
	err := service.ops.WithdrawFromSubAccount(tx, userID, amount, coinSymbol, accountFrom)
	if err != nil {
		log.Info().
			Str("section", "service").
			Str("action", "TransferBetweenSubAccounts").
			Msg("can't transfer between sub accounts")
		return err
	}

	return nil
}

type Answer struct {
	SubAccount model.SubAccount     `json:"sub_account"`
	Balance    *fms.AccountBalances `json:"balance"`
}

func (service *Service) GetBalancesForSubAccounts(userID uint64, accounts []model.SubAccount) ([]Answer, error) {
	var answers []Answer

	for _, account := range accounts {
		balance, err := service.FundsEngine.GetAccountBalances(userID, account.ID)
		if err != nil {
			log.Info().
				Str("section", "service").
				Str("action", "GetBalancesForSubAccounts").
				Msg(err.Error())
			return nil, errors.New("error when getting balance for sub accounts")
		}
		answers = append(answers, Answer{
			SubAccount: account,
			Balance:    balance,
		})
	}

	return answers, nil
}

// GetAccountBalancesForSubAccounts retrieves the AccountBalances for the source and destination subaccounts.
func (service *Service) GetAccountBalancesForSubAccounts(userID, accountFromID, accountToID uint64) (*fms.AccountBalances, *fms.AccountBalances, error) {
	accountFrom, err := service.FundsEngine.GetAccountBalances(userID, accountFromID)
	if err != nil {
		return nil, nil, fmt.Errorf("Error getting accountFrom balances: %v", err)
	}

	accountTo, err := service.FundsEngine.GetAccountBalances(userID, accountToID)
	if err != nil {
		return nil, nil, fmt.Errorf("Error getting accountTo balances: %v", err)
	}

	return accountFrom, accountTo, nil
}

func (service *Service) GetAccountBalanceForSubAccount(userID, accountID uint64) (*fms.AccountBalances, error) {
	accountFrom, err := service.FundsEngine.GetAccountBalances(userID, accountID)
	if err != nil {
		return nil, fmt.Errorf("error getting account balances: %v", err)
	}

	return accountFrom, nil
}
