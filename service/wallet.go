package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	gouuid "github.com/nu7hatch/gouuid"
	"github.com/rs/zerolog/log"
	kafkaGo "github.com/segmentio/kafka-go"
	coins "gitlab.com/paramountdax-exchange/exchange_api_v2/apps/coinsLocal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	data "gitlab.com/paramountdax-exchange/exchange_api_v2/data/wallet"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// GetAllDepositAddresses godoc
func (service *Service) GetAllDepositAddresses(userID uint64) ([]model.Address, error) {
	return service.repo.GetAllDepositAddresses(userID)
}

// GetDepositAddress return a deposit address if found or an error if not found
func (service *Service) GetDepositAddress(userID uint64, symbol string) (*model.Address, error) {
	coin, err := service.GetCoin(symbol)
	if err != nil {
		return nil, err
	}
	return service.repo.GetDepositAddress(userID, coin)
}

// GetWithdrawRequest return a withdraw request by id
func (service *Service) GetWithdrawRequest(id string) (*model.WithdrawRequest, error) {
	return service.repo.GetWithdrawRequest(id)
}

// CancelWithdrawRequest cancel a withdraw request by id
func (service *Service) CancelWithdrawRequest(userID uint64, id string) (*model.WithdrawRequest, error) {
	// PutPendingCancellationStatus -  update status for withdraw request to pending_cancellation
	request := &model.WithdrawRequest{}
	updateDB := service.repo.Conn.Model(&request).Where("user_id = ? and id = ? and status = ?", userID, id, model.WithdrawStatus_Pending).Update("status", model.WithdrawStatus_Pending_Canceled)
	if updateDB.Error != nil {
		return nil, updateDB.Error
	}
	return request, updateDB.Error
}

// WalletCreateMissingDepositAddresses signal
// - call this method on user registration
func (service *Service) WalletCreateMissingDepositAddresses(userID uint64) error {
	// 1. get chains for which we should execute create address commands
	// 2. get all current addresses for the user and only generate events for chains without an address
	chains, err := service.GetChainsWithoutDepositAddresses(userID)
	if err != nil {
		return err
	}
	// 3. generate queue messages with create address commands
	msgs := []kafkaGo.Message{}
	for _, chain := range chains {
		msgs = append(msgs, service.buildNewAddressMessage(userID, chain.Symbol))
	}
	// 4. trigger those commands
	return service.dm.Publish("wallet_commands", map[string]string{}, msgs...)
}

// WalletCreateDepositAddress adds a new request in the queue to generate a deposit address for a user
func (service *Service) WalletCreateDepositAddress(userID uint64, coin *model.Coin) error {

	logger := log.With().Str("service", "WalletCreateDepositAddress").Logger()

	pd, err := service.GetUserPaymentDetails(userID)
	if err != nil {
		return err
	}

	var isDepositAddressMap = map[string]bool{}
	if pd.IsDepositAddress.RawMessage != nil {
		if err := json.Unmarshal(pd.IsDepositAddress.RawMessage, &isDepositAddressMap); err != nil {
			logger.Error().
				Str("object", "userPaymentDetails").
				Err(err).Msg("Unable to unmarshal IsDepositAddress")
			return errors.New("Unable to create deposit address. Please try again later.")
		}

		if isDepositAddressMap[coin.Symbol] {
			return errors.New("The deposit address is still being generated. Please try again later")
		}
	}

	q := service.repo.Conn.
		Table("user_payment_details").
		Where("user_id = ?", userID)

	var address string
	if err = service.repo.ConnReader.
		Where("user_id = ?", userID).
		Where("chain_symbol = ?", coin.ChainSymbol).
		Where("status = ?", model.AddressStatus_Active).
		Order("created_at DESC").
		First(address).Error; err == nil {
		isDepositAddressMap[coin.Symbol] = true
		isDepositAddressJSON, err := json.Marshal(isDepositAddressMap)
		if err != nil {
			logger.Error().Err(err).Msg("Unable to marshal isDepositAddressMap")
			return errors.New("Unable to create deposit address. Please try again later.")
		}

		if err = q.Model(&pd).Update("is_deposit_address", isDepositAddressJSON).Error; err != nil {
			logger.Error().Err(err).Msg("Unable to update user payment details.")
			return errors.New("Unable to create deposit address. Please try again later.")
		}

		return errors.New("Address is already created.")
	}

	isDepositAddressMap[coin.Symbol] = true
	isDepositAddressJSON, err := json.Marshal(isDepositAddressMap)
	if err != nil {
		logger.Error().Err(err).Msg("Unable to marshal isDepositAddressMap")
		return errors.New("Unable to create deposit address. Please try again later.")
	}

	if err = q.Model(&pd).Update("is_deposit_address", isDepositAddressJSON).Error; err != nil {
		log.Error().
			Err(err).
			Msg("Unable to update user payment details.")
		return errors.New("Unable to create deposit address. Please try again later.")
	}

	msg := service.buildNewAddressMessage(userID, coin.ChainSymbol)
	err = service.dm.Publish("wallet_commands", map[string]string{}, msg)
	if err != nil {
		return errors.New("Unable to create missing deposit addresses for user.")
	}

	return nil
}

// WalletCreateWithdrawRequest - make a request to withdraw funds from the wallet
func (service *Service) WalletCreateWithdrawRequest(userID uint64, id, chain, coin string, amount *decimal.Big, to string, decimals int, externalSystem model.WithdrawExternalSystem) error {
	msg := service.buildWithdrawRequestMessage(userID, id, chain, coin, amount, to, decimals, externalSystem)
	// send the message through kafka

	if externalSystem == model.WithdrawExternalSystem_Bitgo {
		return service.dm.Publish("wallet_commands", map[string]string{}, msg)
	} else {
		return service.dm.Publish("wallet_commands_common", map[string]string{}, msg)
	}
}

// WalletGetDeposits - history of deposits
func (service *Service) WalletGetDeposits(userID uint64, operationType model.RequestOperationType, limit, page int) (*model.TransactionWithUserList, error) {
	transactions := make([]model.TransactionWithUser, 0)
	var rowCount int64 = 0

	q := service.repo.ConnReader.
		Table("transactions").
		Where("user_id = ?", userID).
		Joins("inner join coins on transactions.coin_symbol = coins.symbol")
	q = q.Where("tx_type = ? ", model.TxType_Deposit)

	switch operationType {
	case model.RequestOperationType_Crypto:
		{
			q = q.Where("coins.type = ?", model.CoinTypeCrypto)
		}
	case model.RequestOperationType_Fiat:
		{
			q = q.Where("coins.type = ?", model.CoinTypeFiat)
		}
	}

	dbc := q.Table("transactions").Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	db := q.Select("transactions.*, coins.blockchain_explorer").Order("created_at DESC")
	if limit == 0 {
		db = db.Find(&transactions)
	} else {
		db = db.Limit(limit).Offset((page - 1) * limit).Find(&transactions)
	}

	transactionList := model.TransactionWithUserList{
		Transactions: transactions,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: make(map[string]interface{})},
	}
	transactionList.Meta.Filter["status"] = "deposits"

	return &transactionList, db.Error
}

// ExportWalletDeposits - export payment receipt
func (service *Service) ExportWalletDeposits(id string) (*model.GeneratedFile, error) {
	transaction := model.ClearJunctionRequest{}
	userDetails := model.UserDetails{}

	q := service.repo.ConnReader.
		Table("clear_junction_requests").
		Where("order_ref_id = ?", id)

	db := q.Find(&transaction)
	if db.Error != nil {
		return nil, db.Error
	}

	user, err := service.GetUserByID(uint(transaction.UserID))
	if err != nil {
		return nil, err
	}

	d := service.repo.ConnReader.First(&userDetails, "user_id = ?", user.ID)
	if d.Error != nil {
		return nil, d.Error
	}

	resp, err := PDFExportWithdrawPaymentReceipt(user, &transaction, userDetails.Country, service.cfg.ClearJunction)

	generatedFile := model.GeneratedFile{
		Type:     "pdf",
		DataType: "filled",
		Data:     resp,
	}
	return &generatedFile, err
}

// WalletGetWithdrawals - history of withdrawals
func (service *Service) WalletGetWithdrawals(userID uint64, operationType model.RequestOperationType, limit, page int) (*model.WithdrawRequestWithBlockchainLinkList, error) {
	withdrawals := make([]model.WithdrawRequestWithBlockchainLink, 0)
	var rowCount int64 = 0

	q := service.repo.ConnReader.
		Table("withdraw_requests").
		Where("user_id = ?", userID).
		Joins("inner join coins on withdraw_requests.coin_symbol = coins.symbol")
	switch operationType {
	case model.RequestOperationType_Crypto:
		{
			q = q.Where("coins.type = ?", model.CoinTypeCrypto)
		}
	case model.RequestOperationType_Fiat:
		{
			q = q.Where("coins.type = ?", model.CoinTypeFiat)
		}
	}

	dbc := q.Table("withdraw_requests").Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	db := q.Select("withdraw_requests.*, coins.blockchain_explorer").Order("created_at DESC")
	if limit == 0 {
		db = db.Find(&withdrawals)
	} else {
		db = db.Limit(limit).Offset((page - 1) * limit).Find(&withdrawals)
	}

	withdrawRequestList := model.WithdrawRequestWithBlockchainLinkList{
		WithdrawRequests: withdrawals,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: make(map[string]interface{})},
	}
	withdrawRequestList.Meta.Filter["status"] = "withdrawals"

	return &withdrawRequestList, db.Error
}

// Get24HWithdrawals - get sum total of users withdrawals for past 24h
func (service *Service) Get24HWithdrawals(userID uint64, externalSystemValue model.WithdrawExternalSystem, coinValues coins.CoinValueData) (*decimal.Big, error) {
	sumTotal := decimal.New(0, 1)

	rows, err := service.repo.ConnReader.
		Table("withdraw_requests").
		Select("coin_symbol, sum(amount) as amount, sum(fee_amount) as fee_amount").
		Where("user_id = ? AND status <> ?", userID, model.WithdrawStatus_Failed).
		Where("created_at >= NOW() - INTERVAL '24 HOURS'").
		Group("coin_symbol").
		Rows()
	if err != nil {
		return sumTotal, err
	}
	defer rows.Close()
	for rows.Next() {
		coin := ""
		amount := &postgres.Decimal{}
		feeAmount := &postgres.Decimal{}
		_ = rows.Scan(&coin, &amount, &feeAmount)

		var value *decimal.Big
		switch externalSystemValue {
		default:
			fallthrough
		case model.WithdrawExternalSystem_Bitgo:
			value = coinValues[strings.ToUpper(coin)]["BTC"]
		case model.WithdrawExternalSystem_Advcash,
			model.WithdrawExternalSystem_ClearJunction:
			value = coinValues[strings.ToUpper(coin)]["USDT"]
		}

		//  add amount to result
		sumTotal = sumTotal.Add(sumTotal, conv.NewDecimalWithPrecision().Mul(amount.V, value))
		//  add fee amount to result
		sumTotal = sumTotal.Add(sumTotal, conv.NewDecimalWithPrecision().Mul(feeAmount.V, value))
	}

	return sumTotal, err
}

// CanUserWithdraw - check limits based on user level
func (service *Service) CanUserWithdraw(userID uint64, coinSymbolIn string, amount *decimal.Big, externalSystemValue model.WithdrawExternalSystem) (bool, error) {
	logger := log.With().Str("service", "CanUserWithdraw").Stack().
		Uint64("user_id", userID).
		Str("coinSymbolIn", coinSymbolIn).
		Str("amountToWithdraw", amount.String()).
		Str("externalSystemValue", externalSystemValue.String()).
		Logger()

	coinSymbolIn = strings.ToUpper(coinSymbolIn)
	// get user level
	userSettings, err := service.GetProfileSettings(userID)
	if err != nil {
		logger.Error().Err(err).Msg("CanUserWithdrawError: can not get user profile settings")
		return false, err
	}
	// user has unlimited withdraw
	//disabled due to PDAX2-882 task
	//if userSettings.UserLevel > 2 {
	//	return true, nil
	//}

	// get coin values
	coinValues, err := service.coinValues.GetAll()
	if err != nil {
		logger.Error().Err(err).Msg("CanUserWithdrawError: can not get coin values")
		return false, err
	}

	// get withdraw total for past 24H
	totalWithdraw24h, err := service.Get24HWithdrawals(userID, externalSystemValue, coinValues)
	if err != nil {
		logger.Error().Err(err).Msg("CanUserWithdrawError: can not get 24hour withdrawals")
		return false, err
	}

	pd, err := service.GetUserPaymentDetails(userID)
	if err != nil {
		logger.Error().Err(err).Msg("CanUserWithdrawError: can not get user payment details")
		return false, err
	}

	logger = logger.With().Str("totalWithdraw24h", totalWithdraw24h.String()).Logger()

	var withdrawLimit *decimal.Big
	coinSymbolLimit := ""
	switch externalSystemValue {
	default:
		fallthrough
	case model.WithdrawExternalSystem_Default:
		withdrawLimit = pd.DefaultWithdrawLimit.V
		coinSymbolLimit = "USDT"
	case model.WithdrawExternalSystem_Bitgo:
		withdrawLimit = pd.WithdrawLimit.V
		coinSymbolLimit = "BTC"
	case model.WithdrawExternalSystem_Advcash:
		withdrawLimit = pd.AdvCashWithdrawLimit.V
		coinSymbolLimit = "USDT"
	case model.WithdrawExternalSystem_ClearJunction:
		withdrawLimit = pd.ClearJunctionWithdrawLimit.V
		coinSymbolLimit = "USDT"
	}

	logger = logger.With().
		Str("userLimitSymbol", coinSymbolLimit).
		Str("userLimitAmount", withdrawLimit.String()).
		Bool("userLimitNotSetted", withdrawLimit.Sign() < 0).
		Logger()

	// if user limit not setted (-1) -- need to use global limit
	if withdrawLimit.Sign() < 0 {
		// get limits
		var limits []model.AdminFeatureSettings
		limits, err = service.GetWithdrawLimits()
		if err != nil {
			logger.Error().Err(err).Msg("CanUserWithdrawError: can not get withdrawals limits")
			return false, err
		}

		withdrawLimits := map[string]interface{}{}
		for _, limit := range limits {
			withdrawLimits[limit.Feature] = limit.Value
		}

		withdrawLimitLevel := ""
		switch externalSystemValue {
		default:
			fallthrough
		case model.WithdrawExternalSystem_Default:
			withdrawLimitLevel = "default_withdraw_limit_level_%d"
		case model.WithdrawExternalSystem_Bitgo:
			withdrawLimitLevel = "withdraw_limit_level_%d"
		case model.WithdrawExternalSystem_Advcash:
			withdrawLimitLevel = "adv_cash_withdraw_limit_level_%d"
		case model.WithdrawExternalSystem_ClearJunction:
			withdrawLimitLevel = "clear_junction_withdraw_limit_level_%d"
		}

		withdrawLimitLevelKey := fmt.Sprintf(withdrawLimitLevel, userSettings.UserLevel)

		withdrawLimit, _ = conv.NewDecimalWithPrecision().SetString(withdrawLimits[withdrawLimitLevelKey].(string))

		logger = logger.With().Str("withdrawLimitLevel", withdrawLimitLevelKey).
			Str("withdrawLimitGlobal", withdrawLimit.String()).
			Logger()
	}

	crossRate := coinValues[coinSymbolIn][coinSymbolLimit]

	realLimit := conv.NewDecimalWithPrecision().Sub(withdrawLimit, totalWithdraw24h)

	realLimitInSelectedCoin := conv.NewDecimalWithPrecision().Quo(realLimit, crossRate)

	logger.Warn().
		Stack().
		Str("realLimit", realLimit.String()).
		Str("realLimitInSelectedCoin", realLimitInSelectedCoin.String()).
		Bool("withdrawLimitHigherThanAvailable", realLimitInSelectedCoin.Cmp(amount) == 1).
		Msg("Debug withdrawal limits")

	if amount.Cmp(realLimitInSelectedCoin) == 1 {
		logger.Error().Err(err).Msg("CanUserWithdrawError: invalid realLimitInSelectedCoin")
		return false, nil
	}

	return true, nil
}

/**
 * Private methods
 */

func (service *Service) buildNewAddressMessage(userID uint64, chainSymbol string) kafkaGo.Message {
	u, _ := gouuid.NewV4()
	cmd := data.Command{
		ID:     u.String(),
		UserID: userID,
		Action: "create_address",
		Chain:  chainSymbol,
		Meta:   map[string]string{},
		Payload: map[string]string{
			"type": "deposit",
		},
	}
	bytes, _ := cmd.ToBinary()
	return kafkaGo.Message{Value: bytes}
}

func (service *Service) buildWithdrawRequestMessage(userID uint64, id, chain, coin string, amount *decimal.Big, to string, decimals int, externalSystem model.WithdrawExternalSystem) kafkaGo.Message {
	units := conv.NewDecimalWithPrecision().
		Mul(amount, decimal.New(1, -1*decimals)).
		Reduce().
		RoundToInt()
	cmd := data.Command{
		ID:     id,
		UserID: userID,
		Action: "withdraw_request",
		Chain:  chain,
		Coin:   coin,
		Meta:   map[string]string{},
		Payload: map[string]string{
			"amount": fmt.Sprintf("%f", units),
			"to":     to,
		},
		System: string(externalSystem),
	}
	bytes, _ := cmd.ToBinary()
	return kafkaGo.Message{Value: bytes}
}

// IsWithdrawBlocked - checks if withdraw blocked
func (service *Service) IsWithdrawBlocked(userID uint64, symbol string, externalSystem model.WithdrawExternalSystem) error {
	adminFeatureSettings := model.AdminFeatureSettings{}
	coin := model.Coin{}

	q := service.repo.ConnReader

	db := q.First(&adminFeatureSettings, "feature = ?", "block_withdraw")
	if db.Error != nil {
		return db.Error
	}
	if adminFeatureSettings.Value == "true" {
		return errors.New("withdrawal has been blocked")
	}

	db = q.First(&coin, "symbol = ?", symbol)
	if db.Error != nil {
		return db.Error
	}
	if coin.BlockWithdraw {
		return fmt.Errorf("withdrawal by coin %s has been blocked", symbol)
	}

	userPaymentDetails, err := service.GetUserPaymentDetails(userID)
	if err != nil {
		return errors.New("Unable to withdraw. Please try again later.")
	}

	if userPaymentDetails.BlockWithdrawFiat && (externalSystem == model.WithdrawExternalSystem_Advcash || externalSystem == model.WithdrawExternalSystem_ClearJunction) {
		return fmt.Errorf("withdrawal by external system - %s has been blocked for user %d", externalSystem, userID)
	}

	if userPaymentDetails.BlockWithdrawCrypto && externalSystem == model.WithdrawExternalSystem_Bitgo {
		return fmt.Errorf("withdrawal by external system - %s has been blocked for user %d", externalSystem, userID)
	}

	return nil
}

func (service *Service) SaveWalletAddress(userWithdrawAddress model.UserWithdrawAddress) error {
	var userWithdrawAddresses *model.UserWithdrawAddress
	q := service.repo.ConnReader.Table("withdraw_wallet_addresses")
	if err := q.Where("address = ?", userWithdrawAddress.Address).First(&userWithdrawAddresses).Error; err == nil {
		return errors.New("address is already in use")
	}
	return service.repo.Conn.Table("withdraw_wallet_addresses").Create(&userWithdrawAddress).Error
}

func (service *Service) WalletAddressesByUser(userID uint64) ([]*model.UserWithdrawAddress, error) {
	var userWithdrawAddresses []*model.UserWithdrawAddress
	if err := service.repo.ConnReader.Table("withdraw_wallet_addresses").Find(&userWithdrawAddresses, "user_id = ?", userID).Error; err != nil {
		return nil, err
	}

	return userWithdrawAddresses, nil
}

func (service *Service) DeleteWalletAddress(userID uint64, address string) error {
	var userWithdrawAddress *model.UserWithdrawAddress
	return service.repo.Conn.Table("withdraw_wallet_addresses").Delete(&userWithdrawAddress, "address = ?", address).Error
}
