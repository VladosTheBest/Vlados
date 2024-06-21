package service

import (
	"errors"
	"github.com/ericlagergren/decimal"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/solaris"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"strconv"
)

// GetAvailableCardsList returns cards available for user according to available balance on card payment sub account
func (service *Service) GetAvailableCardsList(userID uint64) ([]model.AllowedCard, error) {
	logger := log.With().
		Str("section", "card").
		Str("action", "GetAvailableCardsList").
		Uint64("user_id", userID).
		Logger()

	// Get user total balance
	totalBalance, err := service.GetUserAvailableCardPaymentBalance(userID)
	if err != nil {
		logger.Error().Err(err).Msg("error returned from GetUserAvailableCardPaymentBalance method")
		return nil, err
	}

	// Get list of card types
	cards, err := service.repo.GetAllCardTypes(map[string]interface{}{})
	if err != nil {
		logger.Error().Err(err).Msg("error returned from GetCards method")
		return nil, err
	}

	// Create list of cards that available to current user
	allowedCards := []model.AllowedCard{}
	for _, card := range cards {
		isAllowed := false
		if card.ShipmentPrice.V.Cmp(totalBalance) <= 0 { //todo: ask about this price
			isAllowed = true
		}
		tmp := model.AllowedCard{
			Card:      card,
			IsAllowed: isAllowed,
		}

		allowedCards = append(allowedCards, tmp)
	}

	return allowedCards, nil
}

// GetCurrentUserCardPaymentBalance returns users card payment sub account available balance
func (service *Service) GetCurrentUserCardPaymentBalance(userID uint64) (*decimal.Big, error) {
	logger := log.With().
		Str("section", "card").
		Str("action", "GetCurrentUserCardPaymentBalance").
		Uint64("user_id", userID).
		Logger()

	totalBalance, err := service.GetUserAvailableCardPaymentBalance(userID)
	if err != nil {
		logger.Error().Err(err).Msg("error returned from GetUserAvailableCardPaymentBalance method")
		return nil, err
	}

	return totalBalance, nil
}

// GetUserAvailableCardPaymentBalance returns user total balance in EUR
func (service *Service) GetUserAvailableCardPaymentBalance(userID uint64) (*decimal.Big, error) {
	subAccount, err := service.GetCardPaymentSubAccount(userID)
	if err != nil || subAccount == nil {
		return decimal.New(0, 0), nil
	}

	userBalances, err := service.FundsEngine.GetAccountBalances(userID, subAccount.ID)
	if err != nil {
		return decimal.New(0, 0), err
	}

	coinAvailableBalance, err := userBalances.GetAvailableBalanceForCoin("eur")
	if err != nil {
		return nil, err
	}

	return coinAvailableBalance, nil
}

// DepositToCardAccount used to transfer EUR from main subAccount to card payment subAccount
func (service *Service) DepositToCardAccount(userID uint64, amount *decimal.Big, coin string) error {
	logger := log.With().
		Str("section", "card").
		Str("action", "DepositEURToCardAccount").
		Uint64("user_id", userID).
		Logger()

	// Find users main sub account, if not exist than return error
	subAccountMain, err := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, userID, model.AccountGroupMain)
	if err == subAccounts.Error_UnableToFind {
		logger.Error().Err(err).Msg("GetUserMainSubAccountError: missing users main sub account")
		return errors.New("users main sub account not found")
	}

	// Find users balance in EUR on main acc and compare with transfer amount, if more than user have than error
	userMainBalances, err := service.FundsEngine.GetAccountBalances(userID, subAccountMain.ID)
	if err != nil {
		logger.Error().Err(err).Msg("GetLiabilityBalancesError: can not get user balances")
		return err
	}

	// Get user's current available balance for the coin
	usersAvailableBalance, err := userMainBalances.GetAvailableBalanceForCoin(coin)
	if err != nil {
		logger.Error().Err(err).Msg("Unable to get user balances for the coin")
		return err
	}

	// Check if user's current balance is equal to or greater then amount
	if usersAvailableBalance.Cmp(amount) == -1 {
		err = errors.New("insufficient funds")
		logger.Error().Err(err).Msg("DepositEURToCardAccountError: not enough balance")
		return err
	}

	// Find users card account or create it if not exists
	subAccountCard, err := service.GetCardPaymentSubAccount(userID)
	if err != nil {
		logger.Error().Err(err).Msg("GetPaymentSubAccountError: can not get card payment sub account")
		return err
	}

	if subAccountCard == nil {
		subAccountCard, err = service.CreateCardPaymentAccount(userID)
		if err != nil {
			logger.Error().Err(err).Msg("CreateCardPaymentAccountError: can not create card payment sub account")
			return err
		}
	}

	tx := service.repo.Conn.Begin()

	op := model.NewOperation(model.OperationType_DepositCardPaymentAccount, model.OperationStatus_Accepted)
	if err := tx.Create(op).Error; err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("NewOperationError: can not create operation")
		return err
	}

	cardAccountBalance, err := service.FundsEngine.GetAccountBalances(userID, subAccountCard.ID)
	if err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("GetAccountBalancesError: can not get account balance")
		return err
	}

	err = service.ops.MoveFundsToSubAccount(tx, userID, op, amount, decimal.New(0, 0).Copy(amount), coin, userMainBalances, cardAccountBalance)
	if err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("MoveFundsToSubAccountError: can not move funds to card payment sub account")
		return err
	}

	if err = tx.Commit().Error; err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("cannot commit transaction")
		return err
	}

	return nil
}

// WithdrawFromCardAccount is used to transfer EUR from card subAccount to main subAccount
func (service *Service) WithdrawFromCardAccount(userID uint64, amount *decimal.Big, coin string) error {
	logger := log.With().
		Str("section", "card").
		Str("action", "DepositEURToCardAccount").
		Uint64("user_id", userID).
		Logger()

	// Find users card account or create it if not exists
	subAccountCard, err := service.GetCardPaymentSubAccount(userID)
	if err != nil || subAccountCard == nil {
		logger.Error().Err(err).Msg("GetCardPaymentSubAccountError: can not get card payment sub account")
		return err
	}

	// Find users balance in EUR on card acc and compare with transfer amount, if more than user have than error
	userCardAccountBalances, err := service.FundsEngine.GetAccountBalances(userID, subAccountCard.ID)
	if err != nil {
		logger.Error().Err(err).Msg("GetAccountBalancesError: can not get user balances")
		return err
	}

	// Get user's current available balance for the coin
	usersAvailableBalance, err := userCardAccountBalances.GetAvailableBalanceForCoin(coin)
	if err != nil {
		logger.Error().Err(err).Msg("Unable to get user balances for the coin")
		return err
	}

	if usersAvailableBalance.Cmp(amount) == -1 {
		err = errors.New("insufficient funds")
		logger.Error().Err(err).Msg("WithdrawEURFromCardAccountError: incorrect amount")
		return err
	}

	// Find users main subAccount, if not exist than return error
	subAccountMain, err := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, userID, model.AccountGroupMain)
	if err == subAccounts.Error_UnableToFind {
		logger.Error().Err(err).Msg("GetUserMainSubAccountError: missing users main sub account")
		return errors.New("users main sub account not found")
	}

	tx := service.repo.Conn.Begin()

	op := model.NewOperation(model.OperationType_WithdrawCardPaymentAccount, model.OperationStatus_Accepted)
	if err := tx.Create(op).Error; err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("NewOperationError: can not create operation")
		return err
	}

	mainAccountBalance, err := service.FundsEngine.GetAccountBalances(userID, subAccountMain.ID)
	if err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("GetAccountBalancesError: can not get account balance")
		return err
	}

	err = service.ops.MoveFundsFromSubAccount(tx, userID, op, amount, decimal.New(0, 0).Copy(amount), coin, userCardAccountBalances, mainAccountBalance)
	if err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("MoveFundsToSubAccountError: can not move funds to card payment sub account")
		return err
	}

	if err = tx.Commit().Error; err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("cannot commit transaction")
		return err
	}

	return nil
}

// GetCardPaymentSubAccount returns or creates card payment subAccount
func (service *Service) GetCardPaymentSubAccount(userID uint64) (*model.SubAccount, error) {
	subAccountCard, err := subAccounts.GetUserCardAccount(model.MarketTypeSpot, userID, model.AccountGroupCardPayment)
	if err == subAccounts.Error_UnableToFind {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return subAccountCard, nil
}

func (service *Service) AddConsumer(userID uint64, cardType model.CardType, coinSymbol string, request *solaris.AddConsumerRequest) error {
	logger := log.With().
		Str("section", "service").
		Str("action", "AddConsumer").
		Logger()

	userCardAccount, err := service.GetCardPaymentSubAccount(userID)
	if err != nil {
		logger.Error().Err(err).Msg("Can not get user card subaccount")
		return err
	}

	userCardAccountBalance, err := service.GetAccountBalanceForSubAccount(userID, userCardAccount.ID)
	if err != nil {
		logger.Error().Err(err).Msg("Can not get user card subaccount balance")
		return err
	}

	err = service.WithdrawFromSubAccount(userID, cardType.ShipmentPrice.V, coinSymbol, userCardAccountBalance)
	if err != nil {
		logger.Error().Err(err).Msg("Can not transfer money")
		return err
	}

	// Perform request
	addConsumerResponse, err := service.SolarisSession.AddConsumer(request)
	if err != nil {
		logger.Error().Err(err).Msg("Error in request add consumer")
		return err
	}

	cardAccount := model.CardAccount{
		UserId:                 userID,
		ConsumerId:             uint64(addConsumerResponse.ConsumerPersonalResList[0].ConsumerID),
		AccountId:              uint64(addConsumerResponse.AccountIdentifier),
		CardTypeId:             cardType.Id,
		AccountNumber:          addConsumerResponse.AccountNumber,
		SortCode:               addConsumerResponse.SortCode,
		Iban:                   addConsumerResponse.IBAN,
		Bic:                    addConsumerResponse.BIC,
		Description:            addConsumerResponse.Description,
		Status:                 model.DetermineCardAccountStatus(addConsumerResponse.Status),
		ResponseCode:           addConsumerResponse.ResponseCode,
		ResponseDateTime:       addConsumerResponse.ResponseDateTime,
		ClientRequestReference: addConsumerResponse.ClientRequestReference,
		RequestId:              strconv.Itoa(int(addConsumerResponse.RequestID)),
	}

	// Save info into card account
	err = service.repo.SaveCardAccount(&cardAccount)
	if err != nil {
		logger.Error().Err(err).Msg("Can not save card account")
		return err
	}

	return nil
}

func (service *Service) ActivateCard(request *solaris.ActivateCardRequest) error {
	logger := log.With().
		Str("section", "service").
		Str("action", "AddConsumer").
		Logger()

	response, err := service.SolarisSession.ActivateCard(request)
	if err != nil {
		logger.Error().Err(err).Msg("error in call ActivateCard")
		return err
	}

	if response.Description == "Success" {
		return nil
	} else {
		err = errors.New(response.Description)
		logger.Error().Err(err).Msg("error in response from ActivateCard")
		return err
	}
}

func (service *Service) AddToCardWaitList(waitList *model.CardWaitList) error {
	logger := log.With().
		Str("section", "value").
		Str("method", "AddCardToWaitList").
		Logger()

	tx := service.repo.Conn.Table("card_waitlist").Begin()
	defer tx.Rollback()

	if db := tx.Create(waitList); db.Error != nil {
		logger.Error().Err(db.Error).Msg("Error querying DB")
		return db.Error
	}

	db := tx.Commit()
	if db.Error != nil {
		logger.Error().Err(db.Error).Msg("Error committing query to DB")
		return db.Error
	}

	return nil
}
