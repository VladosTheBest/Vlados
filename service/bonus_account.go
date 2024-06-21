package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Unleash/unleash-client-go/v3"
	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/coins"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/userbalance"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/config"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

func (service *Service) CreateBonusContract(userId uint64, data *model.DepositBonusContractRequest, period *config.BonusAccountPeriod, fromSubAccount *model.SubAccount) error {

	logger := log.With().
		Str("service", "bonus_contract").
		Str("method", "CreateBonusContract").
		Uint64("userId", userId).
		Uint64("fromSubAccountID", fromSubAccount.ID).
		Interface("data", data).
		Logger()

	tx := service.repo.Conn.Begin()

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

	if unleash.IsEnabled("api.bonus-account.risk_checks.25k") {
		// limit of number of contracts
		prdxTotal, err := fromFundsAccount.GetTotalBalanceForCoin("prdx")
		if err != nil {
			tx.Rollback()
			logger.Error().Err(err).Str("coin", "prdx").Msg("unable to get total funds")
			return err
		}
		var totalNumbersOfContracts int64
		err = tx.Table("bonus_account_contracts").
			Where("user_id = ?", fromSubAccount.UserId).
			Where("status = ?", model.BonusAccountContractStatusActive).
			Count(&totalNumbersOfContracts).Error
		if err != nil {
			tx.Rollback()
			logger.Error().Err(err).Msg("unable to get number of contracts")
			return err
		}

		totalNumbersOfContractsDec := conv.NewDecimalWithPrecision().SetUint64(uint64(totalNumbersOfContracts))
		amountForContractStepDec := conv.NewDecimalWithPrecision().SetUint64(uint64(service.cfg.BonusAccount.AmountForContractStep))
		prdxTotalFilledByContracts := conv.NewDecimalWithPrecision().Mul(totalNumbersOfContractsDec, amountForContractStepDec)

		if prdxTotalFilledByContracts.Cmp(prdxTotal) > 0 {
			tx.Rollback()
			diff := conv.NewDecimalWithPrecision().Sub(prdxTotalFilledByContracts, prdxTotal)
			return fmt.Errorf("to create the new contract you should add %s PRDX to your account", utils.FmtDecimal(&postgres.Decimal{V: diff}))
		}
	}

	var toSubAccount *model.SubAccount

	if data.BotType == model.BotTypeNone {
		toSubAccount, err = subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, userId, model.AccountGroupBonus)
		if err != nil {
			if err == subAccounts.Error_UnableToFind {
				toSubAccount, err = service.ops.CreateSubAccount(tx,
					userId,
					model.AccountGroupBonus,
					model.MarketTypeSpot,
					false,
					false,
					false,
					true,
					"Bonus Account",
					"",
					model.SubAccountStatusActive,
				)
				if err != nil {
					logger.Error().Err(err).Msg("unable to create subAccount")
					return err
				}
			} else {
				tx.Rollback()
				logger.Error().Err(err).Msg("unable to get subAccount")
				return err
			}
		}
	} else {
		toSubAccount, err = service.ops.CreateSubAccount(tx,
			userId,
			model.AccountGroupBonus,
			model.MarketTypeSpot,
			false,
			false,
			false,
			false,
			"Bonus Account",
			"",
			model.SubAccountStatusActive,
		)

		if err != nil {
			logger.Error().Err(err).Msg("unable to create subAccount for bot")
			return err
		}
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

	contract, op, err := service.ops.CreateBonusAccountContract(tx, userId, data, period, fromFundsAccount, toFundsAccount)
	if err != nil {
		logger.Error().Err(err).Msg("unable to deposit to the bonus account")
		return err
	}

	if featureflags.IsEnabled("api.bonus-account.with-bot") {
		bot, _, err := service.BotCreate(tx, op, userId, toFundsAccount.GetSubAccountID(), data.BotType, data.BotSettings, logger, data.Amount, data.CoinSymbol)
		if err != nil {
			logger.Error().Err(err).Msg("unable to create bot")
			return err
		}

		if err = service.BonusContractAssignBot(tx, contract, bot); err != nil {
			logger.Error().Err(err).
				Interface("contract", contract).
				Interface("bot", bot).
				Msg("unable to assign bot")
			return err
		}
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

func (service *Service) GetBonusAccountContracts(userId uint64, limit int, page int, coin string, id uint64, status []model.BonusAccountContractStatus) ([]*model.BonusAccountContractViewWithProfitLoss, error) {
	return service.repo.GetBonusAccountContracts(userId, limit, page, coin, id, status)
}

func (service *Service) GetAdminBonusAccountContracts(userId uint64, status []model.BonusAccountContractStatus, pair string,
	from, to, fromDurationDate, toDurationDate int) ([]*model.BonusAccountContractViewWithProfitLoss, error) {
	return service.repo.GetAdminBonusAccountContracts(userId, status, pair, from, to, fromDurationDate, toDurationDate)
}

func (service *Service) GetBonusAccountContractsHistory(userId uint64, pair, status string, from, to int) ([]*model.BonusAccountContractHistory, error) {
	return service.repo.GetBonusAccountContractsHistory(userId, pair, status, from, to)
}

func (service *Service) GetBonusAccountContractsHistoryByContractID(userId, contractID uint64) (*model.BonusAccountContractHistory, error) {
	return service.repo.GetBonusAccountContractsHistoryByContractID(userId, contractID)
}

func (service *Service) ExportBonusAccountContractsHistory(format, tradeType string, contractID uint64,
	bonusAccountContractHistory *model.BonusAccountContractHistory, tradeData []model.Trade) (*model.GeneratedFile, error) {
	data := [][]string{}
	data = append(data, []string{"Contract ID", "Date & Time", "Expired Date", "Pair", "Token", "Liquidation Amount"})
	widths := []int{25, 45, 45, 20, 20, 40}

	if bonusAccountContractHistory.ID == contractID {
		data = append(data, []string{
			fmt.Sprint(bonusAccountContractHistory.ID),
			bonusAccountContractHistory.CreatedAt.Format("2 Jan 2006 15:04:05"),
			bonusAccountContractHistory.ExpiredAt.Format("2 Jan 2006 15:04:05"),
			fmt.Sprint(bonusAccountContractHistory.Pair),
			fmt.Sprint(bonusAccountContractHistory.CoinSymbol),
			fmt.Sprint(utils.Fmt(bonusAccountContractHistory.ProfitLoss))})
	}

	tradesByContractID := [][]string{}
	tradesByContractID = append(tradesByContractID, []string{"ID", "Date & Time", "Side", "Pair", "Amount", "Fees", "Start Price", "End Price"})
	widthsTradesByContractID := []int{45, 45, 20, 20, 45, 55, 45, 45}

	crossRates, err := service.coinValues.GetAll()
	if err != nil {
		return nil, err
	}
	endPrice := crossRates[strings.ToUpper(bonusAccountContractHistory.CoinSymbol)]["USDT"]

	for _, trade := range tradeData {
		var fee *decimal.Big
		coinSymbol := ""
		market, err := service.GetMarketByID(trade.MarketID)

		if err != nil {
			return nil, err
		}
		if trade.TakerSide == "buy" {
			fee = trade.BidFeeAmount.V.Quantize(market.QuotePrecision)
			coinSymbol = market.MarketCoinSymbol
		}
		if trade.TakerSide == "sell" {
			fee = trade.AskFeeAmount.V.Quantize(market.QuotePrecision)
			coinSymbol = market.QuoteCoinSymbol
		}
		tradesByContractID = append(tradesByContractID, []string{
			fmt.Sprint(trade.ID),
			trade.CreatedAt.Format("2 Jan 2006 15:04:05"),
			fmt.Sprint(trade.TakerSide),
			fmt.Sprint(trade.MarketID),
			fmt.Sprint(utils.Fmt(trade.Volume.V.Quantize(market.MarketPrecision))),
			fmt.Sprintf("%f %s", fee, coinSymbol),
			fmt.Sprint(utils.Fmt(trade.Price.V.Quantize(market.MarketPrecision))),
			fmt.Sprint(endPrice)})
	}

	var resp []byte
	//	var err error

	title := "Contracts Trades History"
	titel2 := "Trades"

	if format == "csv" {
		resp, err = CSVExport(data)
	} else {
		resp, err = PDFExportContracts(data, widths, title, tradesByContractID, widthsTradesByContractID, titel2)
	}

	generatedFile := model.GeneratedFile{
		Type:     format,
		DataType: tradeType,
		Data:     resp,
	}
	return &generatedFile, err
}

func (service *Service) GetBonusAccountLandingSettings() (map[string]*model.ContractsInfo, error) {

	coinsList := map[string]*model.ContractsInfo{}

	for _, coin := range coins.GetAll() {
		if featureflags.IsEnabled(fmt.Sprintf("api.bonus-account.coins.%s", coin.Symbol)) {
			coinsList[coin.Symbol] = &model.ContractsInfo{
				Coin:       coin.Symbol,
				Invested:   new(decimal.Big),
				BonusPayed: new(decimal.Big),
			}
		}
	}

	if len(coinsList) == 0 {
		return nil, fmt.Errorf("bonus contract coins temporary unavailable")
	}

	type queryResult struct {
		CoinSymbol string
		RefType    model.OperationType
		Amount     *postgres.Decimal
		Count      float64
	}
	var stats []queryResult

	err := service.repo.ConnReader.Table("liabilities").
		Select("coin_symbol, ref_type, sum(credit) AS amount, count(coin_symbol)").
		Where("((ref_type = ? AND account = ?) OR (ref_type = ? AND account = ? AND comment = 'bonus')) AND sub_account = 0",
			model.OperationType_DepositBonusAccount, model.AccountType_Locked,
			model.OperationType_WithdrawBonusAccount, model.AccountType_Main).
		Group("coin_symbol, ref_type").
		Find(&stats).Error
	if err != nil {
		return nil, fmt.Errorf("unable to load information about contracts: %w", err)
	}

	for _, stat := range stats {
		if coinsList[stat.CoinSymbol] == nil {
			coinsList[stat.CoinSymbol] = &model.ContractsInfo{
				Coin:       stat.CoinSymbol,
				Invested:   new(decimal.Big),
				BonusPayed: new(decimal.Big),
			}
		}
		switch stat.RefType {
		case model.OperationType_DepositBonusAccount:
			coinsList[stat.CoinSymbol].Contracts += stat.Count
			coinsList[stat.CoinSymbol].Invested = new(decimal.Big).Add(coinsList[stat.CoinSymbol].Invested, stat.Amount.V)
		case model.OperationType_WithdrawBonusAccount:
			coinsList[stat.CoinSymbol].BonusPayed = new(decimal.Big).Add(coinsList[stat.CoinSymbol].BonusPayed, stat.Amount.V)
		}
	}

	return coinsList, nil
}
