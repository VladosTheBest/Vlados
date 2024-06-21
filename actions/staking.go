package actions

import (
	"fmt"
	"github.com/ericlagergren/decimal"
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/coins"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"net/http"
	"strconv"
)

func (actions *Actions) CreateStaking(c *gin.Context) {
	if !featureflags.IsEnabled("api.staking.enable-deposit") {
		abortWithError(c, http.StatusBadRequest, "service temporary unavailable")
		return
	}

	userID, _ := getUserID(c)

	var data = &model.CreateStakingRequest{}

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

	if !featureflags.IsEnabled(fmt.Sprintf("api.staking.coins.%s", coin.Symbol)) {
		abortWithError(c, http.StatusBadRequest, fmt.Sprintf("deposit to %s coin not allowed", coin.Symbol))
		return
	}

	if !featureflags.IsEnabled("api.staking.disable-limits") {

		limits := actions.cfg.Staking.Limits

		if limit, ok := limits[coin.Symbol]; !ok {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("unable to load maximum limit for %s", coin.Symbol))
			return
		} else {
			limitMax := new(decimal.Big).SetFloat64(limit.Max)
			if data.Amount.Cmp(limitMax) == 1 {
				abortWithError(c, http.StatusBadRequest, fmt.Sprintf("maximum allowed amount is %f %s", limit.Max, coin.Symbol))
				return
			}

			limitMin := new(decimal.Big).SetFloat64(limit.Min)
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

	if err := actions.service.CreateStaking(userID, data, account); err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusBadRequest, fmt.Sprintf("unable to create contract. Reason: %s", err.Error()))
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Funds moved successfully to staking account",
	})
}

func (actions *Actions) GetStakingSettings(c *gin.Context) {

	coinsList := map[string]bool{}

	for _, coin := range coins.GetAll() {
		if featureflags.IsEnabled(fmt.Sprintf("api.staking.coins.%s", coin.Symbol)) {
			coinsList[coin.Symbol] = true
		}
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"periods": actions.cfg.Staking.Periods,
			"limits":  actions.cfg.Staking.Limits,
			"coins":   coinsList,
		},
	})
}

func (actions *Actions) GetStakingList(c *gin.Context) {
	userID, _ := getUserID(c)

	limit := getQueryAsInt(c, "limit", 3)

	var data []model.StakingWithEarningsAggregation
	if err := actions.service.GetRepo().ConnReader.Table("stakings s").
		Joins("left join staking_earnings se ON se.staking_id = s.id").
		Where("s.user_id = ?", userID).
		Where("s.status = ?", model.StakingStatusActive).
		Group("s.id").
		Select("s.*, coalesce(sum(se.amount), 0.0) as total_earnings, coalesce(sum(se.amount) + s.amount, 0.0) as total_balance").
		Limit(limit).
		Find(&data).Error; err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

func (actions *Actions) GetStakingEarnings(c *gin.Context) {
	userID, _ := getUserID(c)

	sID, _ := c.Params.Get("staking_id")

	stakingID, err := strconv.ParseUint(sID, 10, 64)
	if sID == "all" || err != nil {
		stakingID = 0
	}

	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 1000)
	status := model.StakingEarningStatusPayed

	data, err := actions.service.ListStakingEarnings(userID, stakingID, &status, limit, page)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, 500, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    data,
	})
}
