package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Unleash/unleash-client-go/v3"
	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/coins"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/markets"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/userbalance"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	botGridConnector "gitlab.com/paramountdax-exchange/exchange_api_v2/service/bots/grid"
	botTrendConnector "gitlab.com/paramountdax-exchange/exchange_api_v2/service/bots/trend"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

func (service *Service) GetBots(userID uint64, status []model.BotStatus) ([]*model.BotWithVersions, error) {
	var bots []*model.BotWithVersions
	var err error

	bots, err = service.GetRepo().GetBotsWithVersionsForUser(userID, status)

	if err != nil {
		return nil, err
	}

	return bots, nil
}

func (service *Service) GetBotsByUser(userID uint64) (*model.BotWithTotalActiveBotsAndLockFunds, error) {
	var bots *model.BotWithTotalActiveBotsAndLockFunds
	var err error

	bots, err = service.GetRepo().GetBotWithTotalActiveBotsAndLockFunds(userID)
	if err != nil {
		return nil, err
	}

	crossRates, err := service.coinValues.GetAll()
	if err != nil {
		return nil, err
	}

	totalLockedBalance := conv.NewDecimalWithPrecision()
	for _, bot := range bots.BotWithVersion {
		account, err := subAccounts.GetUserSubAccountByID(model.MarketTypeSpot, bot.Bot.UserId, 0)
		if err != nil {
			return nil, err
		}
		balances, err := service.GetLiabilityBalances(bot.Bot.UserId, account)
		if err != nil {
			return nil, err
		}

		for coinSymbol, balance := range balances {
			totalCross := conv.NewDecimalWithPrecision()
			if balance.Locked != nil && crossRates[strings.ToUpper(coinSymbol)]["USDT"] != nil {
				totalCross = conv.NewDecimalWithPrecision().Mul(crossRates[strings.ToUpper(coinSymbol)]["USDT"], balance.Locked)
			}
			totalLockedBalance.Add(totalLockedBalance, totalCross)
		}
	}

	bots.TotalOfLockedFunds = totalLockedBalance

	return bots, nil
}

func (service *Service) GetBotsAll(status []model.BotStatus) ([]*model.BotWithVersions, error) {
	var bots []*model.BotWithVersions
	var err error

	bots, err = service.GetRepo().GetBotsAllWithVersions(status)

	if err != nil {
		return nil, err
	}

	return bots, nil
}

func (service *Service) GetBot(botID uint) (*model.Bot, error) {
	var bot model.Bot
	if err := service.GetRepo().FindByID(&bot, botID); err != nil {
		return nil, err
	}

	return &bot, nil
}

func (service *Service) BotSeparateLiquidate(bot *model.Bot) error {

	logger := log.With().
		Str("service", "bots").
		Str("method", "BotSeparateLiquidate").
		Uint64("userId", bot.UserId).
		Uint64("botID", bot.ID).
		Logger()

	tx := service.repo.Conn.Begin()

	var err error
	var successfully bool
	if err, successfully = service.BotChangeStatus(tx, bot.UserId, bot.ID, model.BotStatusLiquidated, false); err == nil {
		if successfully {
			if err := tx.Commit().Error; err != nil {
				logger.Error().Err(err).Msg("unable to commit bot stop changes")
				return err
			}
		}
	} else {
		logger.Error().Err(err).Msg("unable to stop the bot")
		return err
	}

	// TMP solution
	tx = service.repo.Conn.Begin()

	accountBalancesFrom, err := service.FundsEngine.GetAccountBalances(bot.UserId, bot.SubAccount)
	if err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("unable to get bot funds account")
		return err
	}
	accountBalancesFrom.LockAccount()
	defer accountBalancesFrom.UnlockAccount()

	accountBalancesTo, err := service.FundsEngine.GetAccountBalances(bot.UserId, model.SubAccountDefaultMain)
	if err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("unable to get main funds account")
		return err
	}
	accountBalancesTo.LockAccount()
	defer accountBalancesTo.UnlockAccount()

	op := model.NewOperation(model.OperationType_WithdrawBot, model.OperationStatus_Accepted)
	op.RefID = bot.RefID // TMP solution for connect bot deposit/withdrawal with liabilities
	if err := tx.Create(op).Error; err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("unable to create operation for bot liquidation")
		return err
	}

	_ = service.CancelUserOrdersBySubAccount(bot.UserId, bot.SubAccount)

	botCoin, _ := coins.Get(bot.CoinSymbol)

	balanceTotal := conv.NewDecimalWithPrecision()
	balanceTotal.Context = decimal.Context128
	balanceTotal.Context.RoundingMode = decimal.ToZero
	balanceTotal.Quantize(botCoin.TokenPrecision)

	accountFrom, err := subAccounts.GetUserSubAccountByID(model.MarketTypeSpot, bot.UserId, bot.SubAccount)
	if err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("unable to load bot subAccount")
		return err
	}

	for coin, balance := range accountBalancesFrom.GetAll() {
		if balance.Available.Cmp(model.ZERO) > 0 {
			fromCoin, _ := coins.Get(coin)
			if fromCoin.Symbol == botCoin.Symbol {
				balanceTotal.Add(balanceTotal, balance.Available)
				continue
			}

			var amountConverted *decimal.Big
			if amountConverted, err = service.BotWalletReBalance(tx, nil, bot, accountFrom, fromCoin, botCoin, balance.Available, op.RefID); err != nil {
				logger.Error().Err(err).Msg("Unable to convert funds to bot coin symbol")
				continue
			}

			if amountConverted != nil {
				balanceTotal.Add(balanceTotal, amountConverted)
			}
		}
	}

	//accountBalance, err := service.FundsEngine.GetAccountBalances(bot.UserId, accountFrom.ID)
	//if err != nil {
	//	tx.Rollback()
	//	logger.Error().Err(err).Msg("unable to get bot account balance")
	//	return err
	//}
	//balanceTotal, err = accountBalance.GetTotalBalanceForCoin(botCoin.Symbol)
	//if err != nil {
	//	tx.Rollback()
	//	logger.Error().Err(err).Msg("unable to get bot account balance total")
	//	return err
	//}

	//balanceTotal, err = service.repo.GetUserAvailableBalanceForCoin(bot.UserId, botCoin.Symbol, accountFrom)
	//if err != nil {
	//	tx.Rollback()
	//	logger.Error().Err(err).Msg("unable to get liability balances")
	//	return err
	//}

	re := model.BotRebalanceEvents{
		FromSubAccount: accountFrom.ID,
		ToSubAccount:   accountBalancesTo.GetSubAccountID(),
		FromCoinSymbol: bot.CoinSymbol,
		FromAmount:     &postgres.Decimal{V: balanceTotal},
		ToAmount:       &postgres.Decimal{V: balanceTotal},
		ToCoinSymbol:   bot.CoinSymbol,
	}

	if err := tx.Create(re).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := service.ops.MoveFundsFromSubAccount(tx, bot.UserId, op, balanceTotal, balanceTotal, bot.CoinSymbol, accountBalancesFrom, accountBalancesTo); err != nil {
		return err
	}

	unlockDepositLiability := model.NewLiability(bot.CoinSymbol, model.AccountType_Locked, op.RefType, op.RefID, bot.UserId, bot.Amount.V, model.Zero, accountBalancesTo.GetSubAccountID(), re.ID)
	_ = service.ops.OMS.SaveLiabilities(unlockDepositLiability)

	return tx.Commit().Error
}

func (service *Service) BotChangeStatus(tx *gorm.DB, userID, botID uint64, status model.BotStatus, isSystem bool) (error, bool) {

	var bot model.Bot
	if err := tx.First(&bot, "user_id = ? AND id = ?", userID, botID).Error; err != nil {
		return err, false
	}

	var botVersion model.BotVersion
	if err := tx.Order("created_at desc").First(&botVersion, "bot_id = ?", botID).Error; err != nil {
		return err, false
	}

	if bot.Status == status {
		tx.Rollback()
		return nil, false
	}

	if !bot.Status.IsStatusAllowed(status, isSystem) {
		tx.Rollback()
		return fmt.Errorf("unable to change the status of bot from %s to %s", bot.Status, status), false
	}

	if err := service.ops.BotStatusChange(tx, &bot, status); err != nil {
		return err, false
	}

	switch bot.Type {
	case model.BotTypeGrid:
		switch status {
		case model.BotStatusActive:

			botSystemId, err := botGridConnector.Start(botVersion.Settings.RawMessage)
			if err != nil {
				if featureflags.IsEnabled("api.bots.grid.requests-enabled") {
					tx.Rollback()
					return err, false
				}
			}

			_, err = service.ops.CreateBotVersion(tx, botVersion.Settings.RawMessage, bot.ID, botSystemId)
			if err != nil {
				tx.Rollback()
				return err, false
			}
		case model.BotStatusStopped:
			if err := botGridConnector.Stop(botVersion.BotSystemID, bot.SubAccount); err != nil {
				if !isSystem && featureflags.IsEnabled("api.bots.grid.requests-enabled") {
					tx.Rollback()
					return err, false
				}
			}
		case model.BotStatusArchived, model.BotStatusStoppedBySystemTrigger, model.BotStatusLiquidated:
			if bot.Status == model.BotStatusActive {
				if err := botGridConnector.Stop(botVersion.BotSystemID, bot.SubAccount); err != nil {
					if !isSystem && featureflags.IsEnabled("api.bots.grid.requests-enabled") {
						tx.Rollback()
						return err, false
					}
				}
			}
		}
	case model.BotTypeTrend:
		switch status {
		case model.BotStatusActive:

			botSystemId, err := botTrendConnector.Start(botVersion.Settings.RawMessage)
			if err != nil {
				if featureflags.IsEnabled("api.bots.trend.requests-enabled") {
					tx.Rollback()
					return err, false
				}
			}

			_, err = service.ops.CreateBotVersion(tx, botVersion.Settings.RawMessage, bot.ID, botSystemId)
			if err != nil {
				tx.Rollback()
				return err, false
			}
		case model.BotStatusStopped:
			if err := botTrendConnector.Stop(botVersion.BotSystemID); err != nil {
				if !isSystem && featureflags.IsEnabled("api.bots.trend.requests-enabled") {
					tx.Rollback()
					return err, false
				}
			}
		case model.BotStatusArchived, model.BotStatusStoppedBySystemTrigger, model.BotStatusLiquidated:
			if bot.Status == model.BotStatusActive {
				if err := botTrendConnector.Stop(botVersion.BotSystemID); err != nil {
					if !isSystem && featureflags.IsEnabled("api.bots.grid.requests-enabled") {
						tx.Rollback()
						return err, false
					}
				}
			}
		}
	default:
		tx.Rollback()
		return errors.New("wrong bot type"), false
	}

	return nil, true
}

func (service *Service) BotCreate(tx *gorm.DB, op *model.Operation, userID uint64, subAccountID uint64, botType model.BotType, settings string, logger zerolog.Logger, amount *decimal.Big, coinSymbol string) (*model.Bot, []*model.BotVersion, error) {
	if !botType.IsTurnedOn() {
		err := errors.New("current type of bot is unavailable")
		logger.Error().Err(err).Msg(err.Error())
		tx.Rollback()
		return nil, nil, err
	}

	if len(settings) == 0 {
		logger.Error().Msg("empty settings")
		tx.Rollback()
		return nil, nil, errors.New("bot settings is empty")
	}

	bot, err := service.ops.CreateBot(tx, op, userID, subAccountID, botType, model.BotStatusActive, amount, coinSymbol)
	if err != nil {
		logger.Error().Err(err).Msg("CreateBotError: can not create bot")
		return nil, nil, err
	}

	switch botType {
	case model.BotTypeGrid:

		var settingsMapped = &model.BotSettingsGrid{}
		if err := json.Unmarshal([]byte(settings), &settingsMapped); err != nil {
			logger.Error().Err(err).Msg("can not unmarshal bot settings")
			tx.Rollback()
			return nil, nil, errors.New("wrong bot settings")
		}

		market, err := markets.Get(settingsMapped.CurrencyPair)
		if err != nil {
			logger.Error().Err(err).Msg("can not get markets")
			tx.Rollback()
			return nil, nil, err
		}

		if !unleash.IsEnabled(fmt.Sprintf("api.bonus-account.market.%s", market.ID)) {
			logger.Error().Msg("orders not allowed")
			tx.Rollback()
			return nil, nil, fmt.Errorf("orders not allowed on %s/%s market", market.MarketCoinSymbol, market.QuoteCoinSymbol)
		}

		settingsMapped.UserAccountId = strconv.FormatUint(subAccountID, 10)
		settingsMapped.BotId = bot.ID

		b, err := json.Marshal(settingsMapped)
		if err != nil {
			logger.Error().Err(err).Msg("can not marshal settings")
			tx.Rollback()
			return nil, nil, err
		}

		botSystemId, err := botGridConnector.Start(b)
		if err != nil {
			if featureflags.IsEnabled("api.bots.grid.requests-enabled") {
				logger.Error().Err(err).Msg("can not start bot")
				tx.Rollback()
				return nil, nil, err
			}
		}

		botVersion, err := service.ops.CreateBotVersion(tx, b, bot.ID, botSystemId)
		if err != nil {
			logger.Error().Err(err).Msg("CreateBotVersionError: can not create bot version")
			tx.Rollback()
			return nil, nil, err
		}

		return bot, []*model.BotVersion{botVersion}, nil
	case model.BotTypeTrend:
		var settingsMapped = &model.BotSettingsTrend{}
		if err := json.Unmarshal([]byte(settings), &settingsMapped); err != nil {
			logger.Error().Err(err).Msg("can not unmarshal bot settings, trend bot")
			tx.Rollback()
			return nil, nil, errors.New("wrong bot settings")
		}

		market, err := markets.Get(settingsMapped.CurrencyPair)
		if err != nil {
			logger.Error().Err(err).Msg("can not get markets")
			tx.Rollback()
			return nil, nil, err
		}

		if !unleash.IsEnabled(fmt.Sprintf("api.bonus-account.market.%s", market.ID)) {
			logger.Error().Msg("orders not allowed trend bot")
			tx.Rollback()
			return nil, nil, fmt.Errorf("orders not allowed on %s/%s market", market.MarketCoinSymbol, market.QuoteCoinSymbol)
		}

		settingsMapped.UserAccountId = strconv.FormatUint(subAccountID, 10)
		settingsMapped.BotId = bot.ID

		b, err := json.Marshal(settingsMapped)
		if err != nil {
			logger.Error().Err(err).Msg("can not marshall settings trend bot")
			tx.Rollback()
			return nil, nil, err
		}

		botSystemId, err := botTrendConnector.Start(b)
		if err != nil {
			if featureflags.IsEnabled("api.bots.trend.requests-enabled") {
				tx.Rollback()
				return nil, nil, err
			}
		}

		botVersion, err := service.ops.CreateBotVersion(tx, b, bot.ID, botSystemId)
		if err != nil {
			logger.Error().Err(err).Msg("can not create bot version bot trend")
			tx.Rollback()
			return nil, nil, err
		}

		return bot, []*model.BotVersion{botVersion}, nil
	default:
		logger.Error().Msg("wrong bot type")
		tx.Rollback()
		return nil, nil, errors.New("wrong bot type")
	}
}

func (service *Service) BonusContractAssignBot(tx *gorm.DB, contract *model.BonusAccountContract, bot *model.Bot) error {
	err := tx.Table("bonus_account_contract_bots").Where("contract_id = ?", contract.ID).Update("active", false).Error
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Raw(""+
		"INSERT INTO bonus_account_contract_bots (contract_id, bot_id, active)"+
		"VALUES (?, ?, true)"+
		"ON CONFLICT ON CONSTRAINT bonus_account_contract_bots_bot_id_contract_id_key "+
		"DO UPDATE SET active = true;", contract.ID, bot.ID).Rows()

	if err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func (service *Service) BotChangeSettings(tx *gorm.DB, userID, botID uint64, settings string) (*model.Bot, []*model.BotVersion, error) {

	if len(settings) == 0 {
		tx.Rollback()
		return nil, nil, errors.New("bot settings is empty")
	}

	var bot model.Bot
	if err := tx.First(&bot, "id = ? AND user_id = ?", botID, userID).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	var botVersion model.BotVersion
	if err := tx.Order("created_at desc").First(&botVersion, "bot_id = ?", botID).Error; err != nil {
		return nil, nil, err
	}

	if err := service.ops.BotStatusChange(tx, &bot, model.BotStatusActive); err != nil {
		return nil, nil, err
	}

	account, err := subAccounts.GetUserSubAccountByID(model.MarketTypeSpot, bot.UserId, bot.SubAccount)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	switch bot.Type {
	case model.BotTypeGrid:
		var settingsMapped = &model.BotSettingsGrid{}
		if err := json.Unmarshal([]byte(settings), &settingsMapped); err != nil {
			tx.Rollback()
			return nil, nil, errors.New("wrong bot settings")
		}

		settingsMapped.UserAccountId = strconv.FormatUint(account.ID, 10)
		settingsMapped.BotId = bot.ID

		b, err := json.Marshal(settingsMapped)
		if err != nil {
			tx.Rollback()
			return nil, nil, err
		}

		if bot.Status == model.BotStatusActive {
			if err := botGridConnector.Stop(botVersion.BotSystemID, bot.SubAccount); err != nil {
				if featureflags.IsEnabled("api.bots.grid.requests-enabled") {
					tx.Rollback()
					return nil, nil, err
				}
			}
		}

		botSystemId, err := botGridConnector.Start(b)
		if err != nil {
			if featureflags.IsEnabled("api.bots.grid.requests-enabled") {
				tx.Rollback()
				return nil, nil, err
			}
		}

		_, err = service.ops.CreateBotVersion(tx, b, bot.ID, botSystemId)
		if err != nil {
			tx.Rollback()
			return nil, nil, err
		}

		var botVersions []*model.BotVersion
		if err := tx.Order("created_at desc").
			Find(&botVersions, "bot_id = ?", botID).Error; err != nil {
			tx.Rollback()
			return nil, nil, err
		}

		return &bot, botVersions, nil
	case model.BotTypeTrend:
		var settingsMapped = &model.BotSettingsTrend{}
		if err := json.Unmarshal([]byte(settings), &settingsMapped); err != nil {
			tx.Rollback()
			return nil, nil, errors.New("wrong bot settings")
		}

		settingsMapped.UserAccountId = strconv.FormatUint(account.ID, 10)
		settingsMapped.BotId = bot.ID

		b, err := json.Marshal(settingsMapped)
		if err != nil {
			tx.Rollback()
			return nil, nil, err
		}

		if bot.Status == model.BotStatusActive {
			if err := botTrendConnector.Stop(botVersion.BotSystemID); err != nil {
				if featureflags.IsEnabled("api.bots.grid.requests-enabled") {
					tx.Rollback()
					return nil, nil, err
				}
			}
		}

		botSystemId, err := botTrendConnector.Start(b)
		if err != nil {
			if featureflags.IsEnabled("api.bots.grid.requests-enabled") {
				tx.Rollback()
				return nil, nil, err
			}
		}

		_, err = service.ops.CreateBotVersion(tx, b, bot.ID, botSystemId)
		if err != nil {
			tx.Rollback()
			return nil, nil, err
		}

		var botVersions []*model.BotVersion
		if err := tx.Order("created_at desc").
			Find(&botVersions, "bot_id = ?", botID).Error; err != nil {
			tx.Rollback()
			return nil, nil, err
		}

		return &bot, botVersions, nil
	default:
		tx.Rollback()
		return nil, nil, errors.New("wrong bot type")
	}
}

func (service *Service) BotWalletReBalance(tx *gorm.DB, re *model.BotRebalanceEvents, bot *model.Bot, account *model.SubAccount, fromCoin, toCoin *model.Coin, amount *decimal.Big, guid string) (*decimal.Big, error) {
	logger := log.With().
		Str("section", "service").
		Str("method", "BotWalletReBalance").
		Logger()

	if conv.NewDecimalWithPrecision().CheckNaNs(amount, nil) {
		logger.Error().Msg("amount param is wrong")
		return nil, errors.New("amount param is wrong")
	}

	if amount.Cmp(model.Zero) <= 0 {
		logger.Error().Msg("amount param less than zero")
		return nil, errors.New("amount param is wrong")
	}

	balance, err := service.GetRepo().GetUserAvailableBalanceForCoin(bot.UserId, fromCoin.Symbol, account)
	if err != nil {
		logger.Error().Err(err).Msg("Unable to get balance")
		return nil, err
	}

	if balance.Cmp(amount) == -1 {
		logger.Error().Msg("insufficient funds for move to another coin")
		return nil, errors.New("insufficient funds for move to another coin")
	}

	crossRates, err := service.coinValues.GetAll()
	if err != nil {
		logger.Error().Err(err).Msg("Unable to get cross rates")
		return nil, err
	}

	toAmount := conv.NewDecimalWithPrecision()
	if crossRates[strings.ToUpper(fromCoin.Symbol)][strings.ToUpper(toCoin.Symbol)] != nil {
		toAmount = toAmount.Mul(crossRates[strings.ToUpper(fromCoin.Symbol)][strings.ToUpper(toCoin.Symbol)], amount)
	}

	amount.Context = decimal.Context128
	amount.Context.RoundingMode = decimal.ToZero
	amount.Quantize(fromCoin.TokenPrecision)

	toAmount.Context = decimal.Context128
	toAmount.Context.RoundingMode = decimal.ToZero
	toAmount.Quantize(toCoin.TokenPrecision)

	if err := service.ops.BotWalletReBalance(tx, re, bot, account, fromCoin, toCoin, amount, toAmount, guid); err != nil {
		logger.Error().Err(err).Msg("Error in BotWalletReBalance operation method")
		return nil, err
	}

	return toAmount, nil
}

func (service *Service) BotFixIssueEmptyBotID() {
	time.Sleep(20 * time.Second)

	loggerRoot := log.With().
		Str("section", "bots").
		Str("action", "BotFixIssueEmptyBotID").
		Logger()

	bots, err := service.GetBotsAll([]model.BotStatus{model.BotStatusActive})
	if err != nil {
		loggerRoot.Error().Err(err).
			Msg("Unable to get all active bots")
		return

	}
	for _, bot := range bots {
		var lastBotVersion *model.BotVersion
		for _, version := range bot.BotVersions {
			if lastBotVersion == nil || version.ID > lastBotVersion.ID {
				lastBotVersion = version
			}
		}

		if lastBotVersion == nil {
			continue
		}

		if lastBotVersion.BotSystemID != "" {
			continue
		}

		logger := loggerRoot.With().
			Str("bot_type", bot.Type.String()).
			Uint64("bot_id", lastBotVersion.BotId).
			Interface("last_bot_version", lastBotVersion).
			Str("config", string(lastBotVersion.Settings.RawMessage)).
			Uint64("bot_version_id", lastBotVersion.ID).
			Logger()

		switch bot.Type {
		case model.BotTypeGrid:
			botSystemId, err := botGridConnector.Start(lastBotVersion.Settings.RawMessage)
			if err != nil {
				logger.Error().Err(err).
					Str("api_url", botGridConnector.GetStartUrl()).
					Msg("Unable to Start the bot")
				continue
			}

			service.repo.Conn.Table("bot_version").Where("id = ?", lastBotVersion.ID).Update("bot_system_id", botSystemId)
		case model.BotTypeTrend:
			botSystemId, err := botTrendConnector.Start(lastBotVersion.Settings.RawMessage)
			if err != nil {
				logger.Error().Err(err).
					Str("api_url", botTrendConnector.GetStartUrl()).
					Msg("Unable to Start the bot")
				continue
			}
			service.repo.Conn.Table("bot_version").Where("id = ?", lastBotVersion.ID).Update("bot_system_id", botSystemId)
		}
	}
}

func (service *Service) GetTotalBots() (int64, error) {
	return service.GetRepo().GetTotalBots()
}

func (service *Service) GetTotalBotsByUserId(userID uint64) (int64, error) {
	return service.GetRepo().GetTotalActiveBotsByUserID(userID)
}

func (service *Service) GetBotsInfo(status model.BotStatus, page, limit int, email string, orderBy model.BotOrderBy,
	order model.BotOrder, botID uint64, from, to int) (*model.BotWithVersionWithContractWithMeta, error) {
	bots, rowCount, err := service.GetRepo().GetAllBots(status, page, limit, email, orderBy, order, botID, from, to)
	if err != nil {
		return nil, err
	}

	data := model.BotWithVersionWithContractWithMeta{
		BotWithVersionWithContract: bots,
		Meta: model.PagingMeta{
			Page:   page,
			Count:  *rowCount,
			Limit:  limit,
			Filter: make(map[string]interface{})},
	}

	return &data, nil
}

func (service *Service) GetBotsTotalOfLockedFunds(status []model.BotStatus) (*decimal.Big, error) {
	bots, err := service.GetBotsAll(status)
	if err != nil {
		return nil, err
	}

	crossRates, err := service.coinValues.GetAll()
	if err != nil {
		return nil, err
	}

	totalOfLockedFunds := conv.NewDecimalWithPrecision()
	for _, bot := range bots {
		account, err := subAccounts.GetUserSubAccountByID(model.MarketTypeSpot, bot.Bot.UserId, 0)
		if err != nil {
			return nil, err
		}
		balances, err := service.GetLiabilityBalances(bot.Bot.UserId, account)
		if err != nil {
			return nil, err
		}

		for coinSymbol, balance := range balances {
			totalCross := conv.NewDecimalWithPrecision()
			if balance.Locked != nil && crossRates[strings.ToUpper(coinSymbol)]["USDT"] != nil {
				totalCross = conv.NewDecimalWithPrecision().Mul(crossRates[strings.ToUpper(coinSymbol)]["USDT"], balance.Locked)
			}
			totalOfLockedFunds.Add(totalOfLockedFunds, totalCross)
		}
	}

	return totalOfLockedFunds, nil
}

func (service *Service) BotSendNotify(bot *model.Bot, message string) error {

	contract, _ := service.repo.GetBonusAccountContract(bot.UserId, bot.ID)

	userDetails, err := service.GetUserDetails(bot.UserId)
	if err != nil {
		return err
	}
	err = service.SendEmailForBotNotify(bot, contract, message, userDetails.Language.String())
	if err != nil {
		return err
	}

	botID := strconv.FormatUint(bot.ID, 10)
	_, err = service.SendNotification(bot.UserId, model.NotificationType_System,
		model.NotificationTitle_BotNotify.String(),
		fmt.Sprintf(model.NotificationMessage_BotNotify.String(), message),
		model.Notification_Bot_Notify, botID)
	if err != nil {
		return err
	}

	return nil
}

func (service *Service) GetGridBotsStatistics(pair string, page, limit, from, to int, botID uint64) (*model.BotWithStatisticsWithMeta, error) {
	var bots *model.BotWithStatisticsWithMeta
	var err error

	bots, err = service.GetRepo().GetGridBotsStatistics(pair, limit, page, from, to, botID)
	if err != nil {
		return nil, err
	}

	return bots, nil
}

func (service *Service) GetTrendBotsStatistics(page, limit, from, to int, botID uint64, pair string) (*model.BotWithTrendStatisticsWithMeta, error) {
	bots, err := service.GetRepo().GetBotsStatisticsTrend(limit, page, from, to, botID, pair)
	if err != nil {
		return nil, err
	}

	return bots, nil
}

func (service *Service) ExportGridBotStatistics(format string, bot *model.BotWithStatisticsWithMeta) (*model.GeneratedFile, error) {
	data := [][]string{}
	data = append(data, []string{"ID", "User", "Status", "Contract ID", "Created At", "Expired At", "Min Price", "Max Price"})
	widths := []int{25, 45, 20, 35, 35, 35, 50, 50}

	for _, b := range bot.BotWithStatistics {
		for _, botWithPrice := range b.BotWithPrice {
			for _, statistics := range botWithPrice {
				data = append(data, []string{
					fmt.Sprint(statistics.BotID),
					fmt.Sprint(statistics.Email),
					fmt.Sprint(statistics.Status),
					fmt.Sprint(statistics.ContractID),
					statistics.CreatedAt.Format("2 Jan 2006 15:04:05"),
					statistics.ExpiredAT.Format("2 Jan 2006 15:04:05"),
					fmt.Sprint(statistics.MinPrice),
					fmt.Sprint(statistics.MaxPrice)})
			}
		}
	}

	var resp []byte
	var err error

	title := "Grid Bot Statistics Report"

	if format == "csv" {
		resp, err = CSVExport(data)
	} else {
		resp, err = PDFExport(data, widths, title)
	}

	generatedFile := model.GeneratedFile{
		Type:     format,
		DataType: "bot statistic",
		Data:     resp,
	}
	return &generatedFile, err
}

func (service *Service) ExportTrendBotStatistics(format string, bot *model.BotWithTrendStatisticsWithMeta) (*model.GeneratedFile, error) {
	data := [][]string{}
	data = append(data, []string{"ID", "User", "Pair", "Contract ID", "Expired At", "Profit", "Profit %"})
	widths := []int{35, 55, 40, 40, 35, 50, 50}

	for _, statistics := range bot.BotWithStatistics {
		data = append(data, []string{
			fmt.Sprint(statistics.BotID),
			fmt.Sprint(statistics.Email),
			fmt.Sprint(statistics.Pair),
			fmt.Sprint(statistics.ContractID),
			statistics.ExpiredAT.Format("2 Jan 2006 15:04:05"),
			fmt.Sprint(statistics.Profit),
			fmt.Sprint(statistics.ProfitPercent)})
	}

	var resp []byte
	var err error

	title := "Trend Bot Statistics Report"

	if format == "csv" {
		resp, err = CSVExport(data)
	} else {
		resp, err = PDFExport(data, widths, title)
	}

	generatedFile := model.GeneratedFile{
		Type:     format,
		DataType: "bot statistic",
		Data:     resp,
	}
	return &generatedFile, err
}

func (service *Service) GetBotsPnlForAdmin() ([]*model.BotPnl, error) {
	var bots []*model.BotPnl
	var err error

	crossRates, err := service.coinValues.GetAll()
	if err != nil {
		return nil, err
	}

	bots, err = service.GetRepo().GetBotsPnlForAdmin(crossRates)

	if err != nil {
		return nil, err
	}

	return bots, nil
}

func (service *Service) BotsGetPnlForUser(userID uint64) ([]*model.BotPnl, []*model.BotPnl, error) {

	crossRates, err := service.coinValues.GetAll()
	if err != nil {
		return nil, nil, err
	}

	var pnlGroupedByBots []*model.BotPnl
	var pnlGroupedByVersions []*model.BotPnl

	wg, _ := errgroup.WithContext(context.Background())

	wg.Go(func() error {
		var err error
		pnlGroupedByBots, err = service.GetRepo().GetBotsPnlForUser(crossRates, userID, false)
		return err
	})

	wg.Go(func() error {
		var err error
		pnlGroupedByVersions, err = service.GetRepo().GetBotsPnlForUser(crossRates, userID, true)
		return err
	})

	if err := wg.Wait(); err != nil {
		return nil, nil, err
	}

	return pnlGroupedByBots, pnlGroupedByVersions, nil
}

func (service *Service) CreateBotSeparate(userId uint64, data *model.CreateBotSeparateRequest, fromSubAccount *model.SubAccount) error {

	logger := log.With().
		Str("service", "bots").
		Str("method", "CreateBotSeparate").
		Uint64("userId", userId).
		Interface("data", data).
		Logger()

	tx := service.repo.Conn.Begin()

	accountBalancesFrom, err := service.FundsEngine.GetAccountBalances(userId, fromSubAccount.ID)
	if err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("unable to get bot funds account")
		return err
	}

	if fromSubAccount.ID != model.SubAccountDefaultMain {
		accountBalancesFrom.LockAccount()
		defer accountBalancesFrom.UnlockAccount()
	}

	available, err := accountBalancesFrom.GetAvailableBalanceForCoin(data.CoinSymbol)
	if err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("unable to get available balance")
		return err
	}

	if available.Cmp(data.Amount) == -1 {
		tx.Rollback()
		return errors.New("insufficient funds")
	}

	var toSubAccount *model.SubAccount
	toSubAccount, err = service.ops.CreateSubAccount(tx,
		userId,
		model.AccountGroupMain,
		model.MarketTypeSpot,
		false,
		false,
		false,
		false,
		"Bot Account",
		"",
		model.SubAccountStatusActive,
	)

	if err != nil {
		logger.Error().Err(err).Msg("unable to create subAccount for bot")
		return err
	}

	subAccounts.Set(toSubAccount, false)

	op := model.NewOperation(model.OperationType_DepositBot, model.OperationStatus_Accepted)
	if err := tx.Create(op).Error; err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("unable to create operation for bot")
	}

	accountBalancesTo, err := service.FundsEngine.InitAccountBalances(toSubAccount, false)
	if err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("unable to create operation for bot")
	}

	if err := service.TransferBetweenSubAccounts(userId, data.Amount, data.CoinSymbol, accountBalancesFrom, accountBalancesTo); err != nil {
		logger.Error().Err(err).Msg("unable to move funds to bot account")
		return err
	}

	_, _, err = service.BotCreate(tx, op, userId, toSubAccount.ID, data.BotType, data.BotSettings, logger, data.Amount, data.CoinSymbol)
	if err != nil {
		logger.Error().Err(err).Msg("unable to create bot")
		return err
	}

	if err := tx.Commit().Error; err != nil {
		logger.Error().Err(err).
			Msg("unable to commit transaction")
		return err
	}

	userbalance.SetWithPublish(userId, fromSubAccount.ID)
	userbalance.SetWithPublish(userId, toSubAccount.ID)

	return nil
}
