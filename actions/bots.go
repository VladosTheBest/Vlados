package actions

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/gin-gonic/gin"
	gouuid "github.com/nu7hatch/gouuid"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/coins"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/userbalance"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service"
)

func (actions *Actions) BotLoadingMiddleware(checkUser bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		botID, err := strconv.Atoi(c.Param("bot_id"))
		if err != nil {
			abortWithError(c, NotFound, err.Error())
			return
		}

		bot, err := actions.service.GetBot(uint(botID))
		if err != nil {
			abortWithError(c, NotFound, err.Error())
			return
		}

		if checkUser {
			userID, exist := getUserID(c)
			if !exist {
				abortWithError(c, http.StatusBadRequest, "user not found")
				return
			}

			if bot.UserId != userID {
				abortWithError(c, NotFound, "")
				return
			}
		}

		account, err := subAccounts.GetUserSubAccountByID(model.MarketTypeSpot, bot.UserId, bot.SubAccount)
		if err != nil {
			abortWithError(c, NotFound, err.Error())
			return
		}

		c.Set("bot", bot)
		c.Set("account", account)
	}
}

func (actions *Actions) BotsGetList(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}

	statusParam := model.BotStatus(c.Query("status"))

	var status []model.BotStatus
	if statusParam.IsValid() && statusParam == model.BotStatusArchived {
		status = []model.BotStatus{
			model.BotStatusArchived,
			model.BotStatusStoppedBySystemTrigger,
			model.BotStatusLiquidated,
		}
	} else {
		status = []model.BotStatus{
			model.BotStatusActive,
			model.BotStatusStopped,
		}
	}

	bots, err := actions.service.GetBots(userID, status)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    bots,
	})
}

func (actions *Actions) AdminBotsGetList(c *gin.Context) {
	id := c.Param("user_id")
	uID, err := strconv.Atoi(id)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}
	userID := uint64(uID)

	bots, err := actions.service.GetBotsByUser(userID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    bots,
	})
}

func (actions *Actions) AdminBotGetAnalytics(c *gin.Context) {
	id := c.Param("user_id")
	uID, err := strconv.Atoi(id)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}

	botID, err := strconv.Atoi(c.Param("bot_id"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	bot, err := actions.service.GetBot(uint(botID))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	userID := uint64(uID)
	if bot.UserId != userID {
		abortWithError(c, NotFound, "")
		return
	}

	version, err := strconv.Atoi(c.Param("version"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	var analytics []model.BotAnalytics

	err = actions.service.GetRepo().ConnReader.
		Where("bot_id = ?", bot.ID).
		Where("version = ?", version).
		Order("created_at DESC").Find(&analytics).Error

	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	for _, analytic := range analytics {
		coin, err := actions.service.GetCoin(analytic.ProfitCoin)
		if err != nil {
			abortWithError(c, NotFound, err.Error())
			return
		}
		analytic.Profit.V.Quantize(coin.TokenPrecision)
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    analytics,
	})
}

func (actions *Actions) BotsSettings(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    actions.cfg.Bots,
	})
}

func (actions *Actions) BotGetAnalytics(c *gin.Context) {

	iBot, exist := c.Get("bot")
	if !exist {
		abortWithError(c, NotFound, "")
		return
	}
	bot := iBot.(*model.Bot)

	version, err := strconv.Atoi(c.Param("version"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	var analytics []model.BotAnalytics

	err = actions.service.GetRepo().ConnReader.
		Where("bot_id = ?", bot.ID).
		Where("version = ?", version).
		Order("created_at DESC").Find(&analytics).Error

	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    analytics,
	})
}

func (actions *Actions) BotsGetAllAnalytics(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "User is not logged")
		return
	}

	status := c.Param("status")

	bots, err := actions.service.GetRepo().GetAllBotsByStatusAndUserID(userID, model.BotStatus(status))
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Error when getting bots")
		return
	}

	var botIDs []uint64
	for _, bot := range bots {
		botIDs = append(botIDs, bot.ID)
	}

	var allAnalytics []model.BotAnalytics

	if len(botIDs) > 0 {
		err = actions.service.GetRepo().ConnReader.
			Where("bot_id IN (?)", botIDs).
			Order("created_at DESC").Find(&allAnalytics).Error

		if err != nil {
			abortWithError(c, http.StatusNotFound, err.Error())
			return
		}
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    allAnalytics,
	})
}

func (actions *Actions) BotChangeSettings(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}

	botSettings := &model.BotCreateUpdateRequest{}
	if err := c.ShouldBindJSON(botSettings); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	tx := actions.service.GetRepo().Conn.Begin()
	bot, versions, err := actions.service.BotChangeSettings(tx, userID, botSettings.ID, botSettings.Settings)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := tx.Commit().Error; err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	data := model.BotWithVersions{
		Bot:         bot,
		ContractID:  0,
		BotVersions: versions,
	}

	var botContract model.BonusAccountContractBots

	if err = tx.First(&botContract, "bot_id = ? AND active = true", bot.ID).Error; err == nil {
		data.ContractID = botContract.ContractID
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

func (actions *Actions) BotChangeStatus(c *gin.Context) {
	botStatusChange := &model.BotStatusChangeRequest{}

	if err := c.ShouldBindJSON(&botStatusChange); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}

	if botStatusChange.Status == model.BotStatusStoppedBySystemTrigger {
		abortWithError(c, http.StatusBadRequest, "user can't apply the system status to the bot")
		return
	}

	tx := actions.service.GetRepo().Conn.Begin()
	if err, successfully := actions.service.BotChangeStatus(tx, userID, botStatusChange.ID, botStatusChange.Status, false); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	} else if successfully {
		if err := tx.Commit().Error; err != nil {
			abortWithError(c, http.StatusBadRequest, err.Error())
			return
		}
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (actions *Actions) BotSeparateLiquidate(c *gin.Context) {
	iBot, exist := c.Get("bot")
	if !exist {
		abortWithError(c, NotFound, "unable to load the bot")
		return
	}

	if !featureflags.IsEnabled("api.bots.separate.liquidate-allow") {
		abortWithError(c, http.StatusNotImplemented, "Liquidation process temporarily disabled")
		return
	}

	bot := iBot.(*model.Bot)

	if err := actions.service.BotSeparateLiquidate(bot); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	userbalance.SetWithPublish(bot.UserId, bot.SubAccount)

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (actions *Actions) BotNotify(c *gin.Context) {
	iBot, exist := c.Get("bot")
	if !exist {
		abortWithError(c, NotFound, "")
		return
	}
	bot := iBot.(*model.Bot)

	type request struct {
		Message string `json:"message"`
	}
	var message request
	err := c.ShouldBindJSON(&message)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err = actions.service.BotSendNotify(bot, message.Message); err != nil {
		log.Error().Err(err).
			Str("section", "bots").
			Str("action", "BotSendNotify").
			Msg("Unable to send notify")
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (actions *Actions) BotGetWalletBalances(c *gin.Context) {
	iBot, exist := c.Get("bot")
	if !exist {
		abortWithError(c, NotFound, "")
		return
	}
	bot := iBot.(*model.Bot)

	iAccount, exist := c.Get("account")
	if !exist {
		abortWithError(c, NotFound, "")
		return
	}
	account := iAccount.(*model.SubAccount)

	balances := service.NewBalances()
	err := actions.service.GetAllLiabilityBalancesForSubAccount(balances, bot.UserId, account)

	if err != nil {
		log.Error().Err(err).
			Str("section", "app:wallet").
			Str("action", "BotGetWalletBalances").
			Msg("Unable to get balances")
		abortWithError(c, 500, "Unable to retrieve balances at this time. Please try again later.")
		return
	}

	c.JSON(200, balances.GetAll())
}

func (actions *Actions) BotCreateOrder(c *gin.Context) {
	iBot, exist := c.Get("bot")
	if !exist {
		abortWithError(c, NotFound, "")
		return
	}
	bot := iBot.(*model.Bot)

	iAccount, exist := c.Get("account")
	if !exist {
		abortWithError(c, NotFound, "")
		return
	}

	account, ok := iAccount.(*model.SubAccount)
	if !ok || account == nil {
		abortWithError(c, NotFound, "")
		return
	}

	if err := c.Request.ParseForm(); err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	c.Request.Form.Set("account", account.GetIDString())

	actions.createOrderWithUser(c, bot.UserId)
}

func (actions *Actions) BotGetListOpenOrders(c *gin.Context) {
	iBot, exist := c.Get("bot")
	if !exist {
		abortWithError(c, NotFound, "")
		return
	}
	bot := iBot.(*model.Bot)

	actions.ListOpenOrdersWithUser(bot.UserId, bot.SubAccount, c)
}

func (actions *Actions) BotGetListClosedOrders(c *gin.Context) {
	iBot, exist := c.Get("bot")
	if !exist {
		abortWithError(c, NotFound, "")
		return
	}
	bot := iBot.(*model.Bot)

	actions.ListClosedOrdersWithUser(bot.UserId, bot.SubAccount, c)
}

func (actions *Actions) BotsGetListAll(c *gin.Context) {

	var status = []model.BotStatus{
		model.BotStatusActive,
		model.BotStatusStopped,
		model.BotStatusStoppedBySystemTrigger,
	}

	bots, err := actions.service.GetBotsAll(status)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    bots,
	})
}

func (actions *Actions) BotWalletReBalance(c *gin.Context) {

	iBot, exist := c.Get("bot")
	if !exist {
		abortWithError(c, NotFound, "")
		return
	}

	bot := iBot.(*model.Bot)

	log.Info().Str("section", "actions").
		Str("method", "BotWalletReBalance").
		Msg(strconv.FormatUint(bot.ID, 10))

	iAccount, exist := c.Get("account")
	if !exist {
		abortWithError(c, NotFound, "")
		return
	}
	account := iAccount.(*model.SubAccount)

	fromCoinParam, fromExist := c.GetPostForm("from_coin")
	if !fromExist {
		abortWithError(c, http.StatusBadRequest, "from_coin param is empty")
		return
	}

	fromCoin, err := coins.Get(fromCoinParam)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "from_coin param is wrong")
		return
	}

	toCoinParam, toExist := c.GetPostForm("to_coin")
	if !toExist {
		abortWithError(c, http.StatusBadRequest, "to_coin param is empty")
		return
	}

	toCoin, err := coins.Get(toCoinParam)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "to_coin param is wrong")
		return
	}

	amount, amountSetted := conv.NewDecimalWithPrecision().SetString(c.PostForm("amount"))
	if !amountSetted {
		abortWithError(c, http.StatusBadRequest, "amount param is wrong")
		return
	}

	tx := actions.service.GetRepo().Conn.Begin()
	guid, _ := gouuid.NewV4()

	if _, err := actions.service.BotWalletReBalance(tx, nil, bot, account, fromCoin, toCoin, amount, guid.String()); err != nil {
		log.Error().Err(err).
			Str("section", "bots").
			Str("action", "BotWalletReBalance").
			Msg("Unable to move funds")
		abortWithError(c, http.StatusInternalServerError, "Unable to move funds")
		return
	}

	if err := tx.Commit().Error; err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to move funds")
		return
	}

	userbalance.SetWithPublish(bot.UserId, account.ID)

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (actions *Actions) BotAddStatistics(c *gin.Context) {

	logger := log.With().
		Str("module", "actions").
		Str("method", "BotAddStatistics").
		Logger()

	iBot, exist := c.Get("bot")
	if !exist {
		abortWithError(c, NotFound, "")
		return
	}
	bot := iBot.(*model.Bot)

	var req model.BotAnalyticsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error().Err(err).
			Str("func", "ShouldBindJSON").
			Msg("Unable to parse json")
		abortWithError(c, NotFound, err.Error())
		return
	}

	version, err := actions.service.GetRepo().GetBotVersionBySystemID(req.BotSystemID)
	if err != nil {
		logger.Error().Err(err).
			Str("func", "GetBotVersionBySystemID").
			Msg("Version not found")
		abortWithError(c, NotFound, err.Error())
		return
	}

	if len(req.Orders) == 0 {
		abortWithError(c, NotFound, "current analytics request don't have any orders")
		return
	}

	orQueries := []string{}
	for _, orderID := range req.Orders {
		orQueries = append(orQueries, fmt.Sprintf("%d = ANY(orders)", orderID))
	}

	query := fmt.Sprintf("SELECT * FROM bot_analytics WHERE bot_id = %d AND version = %d AND (%s)",
		bot.ID, version.ID, strings.Join(orQueries, " OR "))
	q := actions.service.GetRepo().ConnReader.
		Raw(query)

	item := model.BotAnalytics{}
	if err := q.First(&item).Error; err != nil {
		item.BotId = bot.ID
		item.Type = bot.Type
		item.BotSystemID = req.BotSystemID
		item.Version = version.ID
	}

	item.Orders = req.Orders
	item.ProfitPercent = &postgres.Decimal{V: req.ProfitPercent}
	item.Profit = &postgres.Decimal{V: req.Profit}
	item.ProfitCoin = req.ProfitCoin

	db := actions.service.GetRepo().Conn.Table("bot_analytics")
	if item.ID == 0 {
		if err := db.Create(&item).Error; err != nil {
			logger.Error().Err(err).
				Str("func", "db.Create").
				Msg("Unable to create new item")
			abortWithError(c, NotFound, err.Error())
			return
		}
	} else {
		if err := db.Where("id = ?", item.ID).Save(&item).Error; err != nil {
			logger.Error().Err(err).
				Str("func", "db.Create").
				Msg("Unable to update item")
			abortWithError(c, NotFound, err.Error())
			return
		}
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (actions *Actions) BotGetAnalyticsNumbers(c *gin.Context) {
	iBot, exist := c.Get("bot")
	if !exist {
		abortWithError(c, NotFound, "")
		return
	}
	bot := iBot.(*model.Bot)

	var day, week, month, total int

	rows, _ := actions.service.GetRepo().
		ConnReader.
		Raw("select "+
			"(select count(o.id) from orders o where o.sub_account = ? AND o.status = 'filled' AND o.created_at >= date_trunc('day', now() + '-1 day')) as day,"+
			"(select count(o.id) from orders o where o.sub_account = ? AND o.status = 'filled' AND o.created_at >= date_trunc('day', now() + '-7 day')) as week,"+
			"(select count(o.id) from orders o where o.sub_account = ? AND o.status = 'filled' AND o.created_at >= date_trunc('day', now() + '-1 month')) as month,"+
			"(select count(o.id) from orders o where o.sub_account = ? AND o.status = 'filled') as total", bot.SubAccount, bot.SubAccount, bot.SubAccount, bot.SubAccount).Rows()

	if !exist {
		abortWithError(c, NotFound, "")
		return
	}

	defer rows.Close()

	for rows.Next() {
		_ = rows.Scan(&day, &week, &month, &total)
	}

	c.JSON(200, map[string]interface{}{
		"success": true,
		"data": map[string]int{
			"day":   day,
			"week":  week,
			"month": month,
			"total": total,
		},
	})
}

func (actions *Actions) AdminBotGetAnalyticsNumbers(c *gin.Context) {
	id := c.Param("user_id")
	uID, err := strconv.Atoi(id)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}

	botID, err := strconv.Atoi(c.Param("bot_id"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	bot, err := actions.service.GetBot(uint(botID))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	userID := uint64(uID)
	if bot.UserId != userID {
		abortWithError(c, NotFound, "")
		return
	}

	var day, week, month, total int

	rows, _ := actions.service.GetRepo().
		ConnReader.
		Raw("select "+
			"(select count(o.id) from orders o where o.sub_account = ? AND o.status = 'filled' AND o.created_at >= date_trunc('day', now() + '-1 day')) as day,"+
			"(select count(o.id) from orders o where o.sub_account = ? AND o.status = 'filled' AND o.created_at >= date_trunc('day', now() + '-7 day')) as week,"+
			"(select count(o.id) from orders o where o.sub_account = ? AND o.status = 'filled' AND o.created_at >= date_trunc('day', now() + '-1 month')) as month,"+
			"(select count(o.id) from orders o where o.sub_account = ? AND o.status = 'filled') as total", bot.SubAccount, bot.SubAccount, bot.SubAccount, bot.SubAccount).Rows()

	defer rows.Close()

	for rows.Next() {
		_ = rows.Scan(&day, &week, &month, &total)
	}

	c.JSON(200, map[string]interface{}{
		"success": true,
		"data": map[string]int{
			"day":   day,
			"week":  week,
			"month": month,
			"total": total,
		},
	})
}

func (actions *Actions) GetTotalBots(c *gin.Context) {
	data, err := actions.service.GetTotalBots()
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, data)
}

func (actions *Actions) GetTotalBotsByUserId(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	data, err := actions.service.GetTotalBotsByUserId(userID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, data)
}

func (actions *Actions) GetInfoAboutBots(c *gin.Context) {
	email := c.Query("email")
	statusStr := c.Query("status")
	orderBy := model.BotOrderBy(c.Query("order_by"))
	order := model.BotOrder(c.Query("order"))
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	status := model.BotStatus(statusStr)
	botIDStr := c.Query("bot_id")
	botID, _ := strconv.Atoi(botIDStr)
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)

	if !order.IsValid() {
		abortWithError(c, http.StatusBadRequest, errors.New("bot order type is wrong").Error())
		return
	}

	if !orderBy.IsValid() {
		abortWithError(c, http.StatusBadRequest, errors.New("bot order by type is wrong").Error())
		return
	}

	bots, err := actions.service.GetBotsInfo(status, page, limit, email, orderBy, order, uint64(botID), from, to)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    bots,
	})
}

func (actions *Actions) GetTotalOfLockedFunds(c *gin.Context) {
	var status = []model.BotStatus{
		model.BotStatusActive,
		model.BotStatusStopped,
		model.BotStatusStoppedBySystemTrigger,
	}

	totalOfLockedFunds, err := actions.service.GetBotsTotalOfLockedFunds(status)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    totalOfLockedFunds,
	})
}

func (actions *Actions) GetBotsStatistics(c *gin.Context) {
	pair := c.Query("pair")
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	fromDate := getQueryAsInt(c, "from_date", 0)
	toDate := getQueryAsInt(c, "to_date", 0)
	botType := model.BotType(c.Param("bot_type"))
	botIDStr := c.Query("bot_id")
	botID, _ := strconv.Atoi(botIDStr)

	if !botType.IsValid() {
		abortWithError(c, http.StatusBadRequest, "bot type parameter is wrong")
		return
	}

	var bots interface{}
	var err error
	switch botType {
	case model.BotTypeGrid:
		bots, err = actions.service.GetGridBotsStatistics(pair, page, limit, fromDate, toDate, uint64(botID))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, err.Error())
			return
		}
	case model.BotTypeTrend:
		bots, err = actions.service.GetTrendBotsStatistics(page, limit, fromDate, toDate, uint64(botID), pair)
		if err != nil {
			abortWithError(c, http.StatusBadRequest, err.Error())
			return
		}
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    bots,
	})
}

func (actions *Actions) ExportBotsStatistics(c *gin.Context) {
	pair := c.Query("pair")
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	fromDate := getQueryAsInt(c, "from_date", 0)
	toDate := getQueryAsInt(c, "to_date", 0)
	botType := model.BotType(c.Param("bot_type"))
	botIDStr := c.Query("bot_id")
	botID, _ := strconv.Atoi(botIDStr)
	format := c.Query("format")

	if !botType.IsValid() {
		abortWithError(c, http.StatusBadRequest, "bot type parameter is wrong")
		return
	}

	var data *model.GeneratedFile
	switch botType {
	case model.BotTypeGrid:
		bots, err := actions.service.GetGridBotsStatistics(pair, page, limit, fromDate, toDate, uint64(botID))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, err.Error())
			return
		}

		data, err = actions.service.ExportGridBotStatistics(format, bots)
		if err != nil {
			abortWithError(c, 500, "Unable to export data")
			return
		}
	case model.BotTypeTrend:
		bots, err := actions.service.GetTrendBotsStatistics(page, limit, fromDate, toDate, uint64(botID), pair)
		if err != nil {
			abortWithError(c, http.StatusBadRequest, err.Error())
			return
		}

		data, err = actions.service.ExportTrendBotStatistics(format, bots)
		if err != nil {
			abortWithError(c, 500, "Unable to export data")
			return
		}
	}

	c.JSON(http.StatusOK, data)
}

func (actions *Actions) BotsGetPnlForUser(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, 500, "Unable to load PNL")
		return
	}

	pnlGroupedByBots, pnlGroupedByVersions, err := actions.service.BotsGetPnlForUser(userID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"pnlGroupedByBots":     pnlGroupedByBots,
			"pnlGroupedByVersions": pnlGroupedByVersions,
		},
	})
}

func (actions *Actions) GetBotsPnlForAdmin(c *gin.Context) {
	bots, err := actions.service.GetBotsPnlForAdmin()
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    bots,
	})
}

func (actions *Actions) BotCreateSeparate(c *gin.Context) {
	if !featureflags.IsEnabled("api.bots.create") {
		abortWithError(c, http.StatusBadRequest, "service temporary unavailable")
		return
	}

	userID, _ := getUserID(c)

	var data = &model.CreateBotSeparateRequest{}

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

	if !featureflags.IsEnabled(fmt.Sprintf("api.bots.coins.%s", coin.Symbol)) {
		abortWithError(c, http.StatusBadRequest, fmt.Sprintf("deposit to %s coin not allowed", coin.Symbol))
		return
	}

	if !featureflags.IsEnabled("api.bots.disable-limits") {

		limits := actions.cfg.Bots.Limits

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

	// Initialize the balances for the new bot in FMS
	_, err = actions.service.FundsEngine.InitAccountBalances(account, false)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("bots", "BotCreateSeparate").Msg("Unable to initialize balances for new bot")
		abortWithError(c, http.StatusInternalServerError, "Unable to initialize balances for new bot")
		return
	}

	if err := actions.service.CreateBotSeparate(userID, data, account); err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusBadRequest, fmt.Sprintf("unable to create the bot. Reason: %s", err.Error()))
		return
	}

	userbalance.SetWithPublish(userID, account.ID)

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Funds moved successfully to the bot account",
	})
}
