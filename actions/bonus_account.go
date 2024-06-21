package actions

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Unleash/unleash-client-go/v3"
	"github.com/ericlagergren/decimal"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/coins"
	marketCache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/markets"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

func (actions *Actions) BonusAccountDeposit(c *gin.Context) {
	if !featureflags.IsEnabled("api.bonus-account.enable-deposit") {
		abortWithError(c, http.StatusBadRequest, "service temporary unavailable")
		return
	}

	userID, _ := getUserID(c)

	var data = &model.DepositBonusContractRequest{}

	if err := c.Bind(data); err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	coin, err := coins.Get(data.CoinSymbol)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "wrong coin")
		return
	}

	data.Amount.Context = decimal.Context128
	data.Amount.Context.RoundingMode = decimal.ToZero
	data.Amount.Quantize(coin.TokenPrecision)

	periods := actions.cfg.BonusAccount.GetPeriodsMap()

	period, ok := periods[data.Period]
	if !ok {
		abortWithError(c, http.StatusBadRequest, "wrong period")
		return
	}

	if !featureflags.IsEnabled(fmt.Sprintf("api.bonus-account.coins.%s", coin.Symbol)) {
		abortWithError(c, http.StatusBadRequest, fmt.Sprintf("deposit to %s coin not allowed", coin.Symbol))
		return
	}

	if !featureflags.IsEnabled("api.bonus-account.disable-number-of-contract-limit") {
		var numberOfContracts int64
		err := actions.service.GetRepo().Conn.Table("bonus_account_contracts").
			Where("user_id = ?", userID).
			Where("status = ?", model.BonusAccountContractStatusActive).
			Count(&numberOfContracts).Error
		if err != nil {
			abortWithError(c, http.StatusBadRequest, "unable to count bonus contracts")
			return
		}

		if uint64(numberOfContracts) >= actions.cfg.BonusAccount.MaxContractsPerUser {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("maximum number of contracts is %d", actions.cfg.BonusAccount.MaxContractsPerUser))
			return
		}
	}

	if !featureflags.IsEnabled("api.bonus-account.disable-limits") {

		limits := actions.cfg.BonusAccount.Limits

		if limit, ok := limits[coin.Symbol]; !ok {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("unable to load maximum limit for %s", coin.Symbol))
			return
		} else {
			limitMax := conv.NewDecimalWithPrecision().SetFloat64(limit.Max)
			if data.Amount.Cmp(limitMax) == 1 {
				abortWithError(c, http.StatusBadRequest, fmt.Sprintf("maximum allowed amount is %f %s", limit.Max, coin.Symbol))
				return
			}

			limitMin := conv.NewDecimalWithPrecision().SetFloat64(limit.Min)
			if data.Amount.Cmp(limitMin) == -1 {
				abortWithError(c, http.StatusBadRequest, fmt.Sprintf("minimum allowed amount is %f %s", limit.Min, coin.Symbol))
				return
			}
		}
	}

	account, err := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, userID, model.AccountGroupMain)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "account not found")
		return
	}

	// Initialize the balances for the new user in FMS
	_, err = actions.service.FundsEngine.InitAccountBalances(account, false)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("bonus_account", "BonusAccountDeposit").Msg("Unable to initialize balances for new bot")
		abortWithError(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
		return
	}

	if err := actions.service.CreateBonusContract(userID, data, period, account); err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusBadRequest, fmt.Sprintf("unable to create contract. Reason: %s", err.Error()))
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Funds moved successfully to bonus account",
	})
}

func (actions *Actions) GetBonusAccountSettings(c *gin.Context) {

	coinsList := map[string]bool{}

	for _, coin := range coins.GetAll() {
		coinsList[coin.Symbol] = featureflags.IsEnabled(fmt.Sprintf("api.bonus-account.coins.%s", coin.Symbol))
	}

	marketsList := map[string]bool{}

	for _, market := range marketCache.GetAllActive() {
		marketsList[market.ID] = unleash.IsEnabled(fmt.Sprintf("api.bonus-account.market.%s", market.ID))
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"periods": actions.cfg.BonusAccount.Periods,
			"limits":  actions.cfg.BonusAccount.Limits,
			"coins":   coinsList,
			"markets": marketsList,
		},
	})
}

func (actions *Actions) GetBonusAccountLandingSettings(c *gin.Context) {

	coinsList, err := actions.service.GetBonusAccountLandingSettings()
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	for coin, coinInfo := range coinsList {
		if multipliers, ok := actions.cfg.BonusAccount.Landing.Multipliers[coin]; ok {
			if multipliers.Contracts > 0 {
				coinInfo.Contracts *= multipliers.Contracts
			}

			if multipliers.Invested > 0 {
				coinInfo.Invested = new(decimal.Big).Mul(coinInfo.Invested, new(decimal.Big).SetFloat64(multipliers.Invested))
			}

			if multipliers.BonusPayed > 0 {
				coinInfo.BonusPayed = new(decimal.Big).Mul(coinInfo.BonusPayed, new(decimal.Big).SetFloat64(multipliers.BonusPayed))
			}
		}
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"periods": actions.cfg.BonusAccount.Periods,
			"limits":  actions.cfg.BonusAccount.Limits,
			"coins":   coinsList,
		},
	})
}

func (actions *Actions) GetBonusAccountLandingChart(c *gin.Context) {
	abortWithError(c, http.StatusNotImplemented, "Not implemented yet")
}

func (actions *Actions) GetBonusAccountContractsList(c *gin.Context) {
	userID, _ := getUserID(c)
	limit := getQueryAsInt(c, "limit", 0)
	coin := c.Query("coin")
	page := getQueryAsInt(c, "page", 0)
	id := uint64(0)
	var err error
	idString := c.Query("id")
	if idString != "" {
		id, err = strconv.ParseUint(idString, 10, 64)
		if err != nil {
			abortWithError(c, http.StatusBadRequest, err.Error())
			return
		}
	}

	statusStringArray := c.QueryArray("status")
	statusArray := []model.BonusAccountContractStatus{}
	for _, st := range statusStringArray {
		tmpStatus := model.GetBonusAccountContractStatusFromString(st)
		if tmpStatus != "" {
			statusArray = append(statusArray, tmpStatus)
		}
	}

	contracts, err := actions.service.GetBonusAccountContracts(userID, limit, page, coin, id, statusArray)

	if err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    contracts,
	})
}

func (actions *Actions) GetAdminBonusAccountContractsList(c *gin.Context) {
	id := c.Param("user_id")
	uid, _ := strconv.Atoi(id)
	userID := uint64(uid)
	coin := c.Query("coin")
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)
	fromDD := c.Query("fromDurationDate")
	fromDurationDate, _ := strconv.Atoi(fromDD)
	toDD := c.Query("toDurationDate")
	toDurationDate, _ := strconv.Atoi(toDD)

	contracts, err := actions.service.GetAdminBonusAccountContracts(userID, []model.BonusAccountContractStatus{
		model.BonusAccountContractStatusActive,
		model.BonusAccountContractStatusPendingExpiration,
		model.BonusAccountContractStatusExpired,
		model.BonusAccountContractStatusPayed,
	}, coin, from, to, fromDurationDate, toDurationDate)

	if err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    contracts,
	})
}

func (actions *Actions) GetBonusAccountContractsHistoryList(c *gin.Context) {
	userID, _ := getUserID(c)
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)
	pair := c.Query("pair")
	status := c.Query("status")

	_, err := actions.service.GetBonusAccountContractsHistory(userID, pair, status, from, to)
	if err != nil {
		abortWithError(c, 500, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    userID,
	})
}

func (actions *Actions) ExportBonusAccountContractsHistoryList(c *gin.Context) {
	userID, _ := getUserID(c)
	format := c.Query("format")
	side := c.Query("sideParam")
	marketCoinSymbol := c.Query("market_coin_symbol")
	quoteCoinSymbol := c.Query("quote_coin_symbol")
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)
	query := c.Query("query")
	status := "fees"
	sort := c.Query("sort")
	contractID, err := strconv.Atoi(c.Param("contract_id"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	contract, err := actions.service.GetBonusAccountContractsHistoryByContractID(userID, uint64(contractID))
	if err != nil {
		abortWithError(c, 500, err.Error())
		return
	}

	markets, err := actions.service.LoadMarketIDsByCoin(marketCoinSymbol, quoteCoinSymbol)
	if err != nil {
		log.Error().
			Str("actions", "bonus_account.go").
			Str("ExportBonusAccountContractsHistoryList", "LoadMarketIDsByCoin").
			Msg("Unable to load market id's by coin")
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 0 limit to get all data, no paging
	trades, err := actions.service.GetUserTradeHistory(userID, status, 0, 1, from, to, side, markets, query, sort, contract.SubAccount)
	if err != nil {
		abortWithError(c, 500, err.Error())
		return
	}

	data, err := actions.service.ExportBonusAccountContractsHistory(format, status, uint64(contractID), contract, trades.Trades.Trades)
	if err != nil {
		abortWithError(c, 500, "Unable to export data")
		return
	}
	c.JSON(200, data)
}
