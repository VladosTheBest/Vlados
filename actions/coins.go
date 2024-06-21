package actions

import (
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	coinsCache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/coins"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// GetCoins godoc
// swagger:route GET /coins coin get_coins
// Get coins
//
// Get the list of all supported coins available in the exchange.
//
//	Consumes:
//	- application/x-www-form-urlencoded
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Responses:
//	  200: Coins
//	  404: RequestErrorResp
func (actions *Actions) GetCoins(c *gin.Context) {
	status := c.Query("status")
	data, err := coinsCache.GetAllActive(status)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Invalid Status")
		return
	}

	c.JSON(http.StatusOK, data)
}

// AddCoin godoc
// swagger:route POST /coins admin add_coin
// Add coin
//
// Add a new coin
//
//	    Consumes:
//	    - multipart/form-data
//
//	    Produces:
//	    - application/json
//
//	    Schemes: http, https
//
//	    Security:
//			   AdminToken:
//
//	    Responses:
//	      200: Coin
//	      400: RequestErrorResp
//	      500: RequestErrorResp
func (actions *Actions) AddCoin(c *gin.Context) {
	logger := log.With().
		Str("section", "actions").
		Str("action", "AddCoin").
		Logger()
	name, _ := c.GetPostForm("name")
	sdigits, _ := c.GetPostForm("digits")
	digits, _ := strconv.Atoi(sdigits)
	sprecision, _ := c.GetPostForm("precision")
	precision, _ := strconv.Atoi(sprecision)
	contractAddress, _ := c.GetPostForm("contractAddress")
	symbol, _ := c.GetPostForm("symbol")
	minWithdraw, _ := c.GetPostForm("minWithdraw")
	decMinWithdraw, _ := conv.NewDecimalWithPrecision().SetString(minWithdraw)
	withdrawFee, _ := c.GetPostForm("withdrawFee")
	decWithdrawFee, _ := conv.NewDecimalWithPrecision().SetString(withdrawFee)
	sMinConfirmations, _ := c.GetPostForm("minConfirmations")
	minConfirmations, _ := strconv.Atoi(sMinConfirmations)
	depositFee, _ := c.GetPostForm("depositFee")
	decDepositFee, _ := conv.NewDecimalWithPrecision().SetString(depositFee)
	costSymbol, _ := c.GetPostForm("costSymbol")

	withdrawFeeAdvCashParam, withdrawFeeAdvCashParamExist := c.GetPostForm("withdrawFeeAdvCash")
	if !withdrawFeeAdvCashParamExist {
		withdrawFeeAdvCashParam = "0"
	}
	withdrawFeeAdvCash, _ := conv.NewDecimalWithPrecision().SetString(withdrawFeeAdvCashParam)

	withdrawFeeClearJunctionParam, withdrawFeeClearJunctionParamExist := c.GetPostForm("withdrawFeeClearJunction")
	if !withdrawFeeClearJunctionParamExist {
		withdrawFeeClearJunctionParam = "0"
	}

	withdrawFeeClearJunction, _ := conv.NewDecimalWithPrecision().SetString(withdrawFeeClearJunctionParam)

	if costSymbol == "" {
		costSymbol = symbol
	}
	if costSymbol != symbol {
		_, err := actions.service.GetCoin(costSymbol)
		if err != nil {
			logger.Error().Err(err).Msg("Invalid cost symbol")
			abortWithError(c, 400, "Invalid cost symbol")
			return
		}
	}

	chainSymbol, _ := c.GetPostForm("chainSymbol")
	blockchainExplorer, _ := c.GetPostForm("blockchainExplorer")
	coinType, _ := c.GetPostForm("coinType")
	coinT, err := model.GetCoinTypeFromString(coinType)
	if err != nil {
		logger.Error().Err(err).Msg("Invalid coin type")
		abortWithError(c, 400, "Invalid coin type")
		return
	}

	status, _ := c.GetPostForm("status")
	sts, err := model.GetCoinStatusFromString(status)
	if err != nil {
		abortWithError(c, 400, "Invalid Status")
		return
	}

	shouldGetValue := c.GetBool("should_get_value")

	data, err := actions.service.AddCoin(coinT, chainSymbol, symbol, name, digits, precision, decMinWithdraw, decWithdrawFee, decDepositFee, contractAddress, sts, costSymbol, blockchainExplorer, minConfirmations, shouldGetValue, withdrawFeeAdvCash, withdrawFeeClearJunction)
	if err != nil {
		logger.Error().Err(err).Msg("Unable to get coins value")
		abortWithError(c, 500, "Unable to add coin")
		return
	}
	c.JSON(200, data)
}

// UpdateCoin godoc
// swagger:route PUT /coins/{symbol} admin update_coin
// Update coin
//
// Update coin
//
//	    Consumes:
//	    - multipart/form-data
//
//	    Produces:
//	    - application/json
//
//	    Schemes: http, https
//
//	    Security:
//			   AdminToken:
//
//	    Responses:
//	      200: Coin
//	      400: RequestErrorResp
//	      404: RequestErrorResp
//	      500: RequestErrorResp
func (actions *Actions) UpdateCoin(c *gin.Context) {
	name, _ := c.GetPostForm("name")
	sdigits, _ := c.GetPostForm("digits")
	digits, _ := strconv.Atoi(sdigits)
	contractAddress, _ := c.GetPostForm("contractAddress")
	symbol, _ := c.GetPostForm("symbol")

	minWithdraw, _ := c.GetPostForm("minWithdraw")
	decMinWithdraw, _ := conv.NewDecimalWithPrecision().SetString(minWithdraw)
	withdrawFee, _ := c.GetPostForm("withdrawFee")
	decWithdrawFee, _ := conv.NewDecimalWithPrecision().SetString(withdrawFee)
	depositFee, _ := c.GetPostForm("depositFee")
	decDepositFee, _ := conv.NewDecimalWithPrecision().SetString(depositFee)
	blockchainExplorer, _ := c.GetPostForm("blockchainExplorer")
	sMinConfirmations, _ := c.GetPostForm("minConfirmations")
	minConfirmations, _ := strconv.Atoi(sMinConfirmations)

	costSymbol, _ := c.GetPostForm("costSymbol")
	if costSymbol == "" {
		costSymbol = symbol
	}
	if costSymbol != symbol {
		_, err := actions.service.GetCoin(costSymbol)
		if err != nil {
			abortWithError(c, 400, "Invalid cost symbol")
			return
		}
	}

	coinType, _ := c.GetPostForm("coinType")
	coinT, err := model.GetCoinTypeFromString(coinType)
	if err != nil {
		abortWithError(c, 400, "Invalid coin type")
		return
	}

	chainSymbol, _ := c.GetPostForm("chainSymbol")

	status, _ := c.GetPostForm("status")
	sts, err := model.GetCoinStatusFromString(status)
	if err != nil {
		abortWithError(c, 400, "Invalid status")
		return
	}

	//  get coin from database
	currentSymbol := c.Param("coin_symbol")
	coin, err := actions.service.GetCoin(currentSymbol)
	if err != nil {
		abortWithError(c, 404, "Coin not found")
		return
	}
	if name == "" {
		name = coin.Name
	}
	var shouldGetValue bool

	_, existsShouldGetValue := c.Get("should_get_value")

	if existsShouldGetValue {
		shouldGetValue = c.GetBool("should_get_value")
	} else {
		shouldGetValue = coin.ShouldGetValue
	}

	data, err := actions.service.UpdateCoin(coin, coinT, name, symbol, digits, decMinWithdraw, decWithdrawFee, decDepositFee, contractAddress, sts, costSymbol, blockchainExplorer, minConfirmations, shouldGetValue, chainSymbol)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to update coin")
		return
	}
	c.JSON(http.StatusOK, data)
}

// DeleteCoin godoc
// swagger:route DELETE /coins/{symbol} admin delete_coin
// Deactivate coin
//
// Deactivate a coin
//
//	    Consumes:
//	    - multipart/form-data
//
//	    Produces:
//	    - application/json
//
//	    Schemes: http, https
//
//	    Security:
//			   AdminToken:
//
//	    Responses:
//	      200: StringResp
//	      404: RequestErrorResp
//	      500: RequestErrorResp
func (actions *Actions) DeleteCoin(c *gin.Context) {
	symbol := c.Param("coin_symbol")
	coin, err := actions.service.GetCoin(symbol)
	if err != nil {
		abortWithError(c, 404, "Coin not found")
		return
	}
	err = actions.service.DeleteCoin(coin)
	if err != nil {
		abortWithError(c, 500, "Unable to delete coin")
		return
	}
	c.JSON(200, "Coin has been successfully deleted")
}

// GetCoin godoc
// swagger:route GET /coins/{symbol} coin get_coin
// Get coin
//
// Get a single coin by symbol
//
//	Consumes:
//	- application/x-www-form-urlencoded
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Responses:
//	  200: Coin
//	  404: RequestErrorResp
func (actions *Actions) GetCoin(c *gin.Context) {
	coinSymbol := c.Param("coin_symbol")
	data, err := actions.service.GetCoin(coinSymbol)
	if err != nil {
		abortWithError(c, 404, "Coin not found")
		return
	}
	c.JSON(200, data)
}

// GetCoinsValue godoc
// swagger:route GET /coins-value/{symbol} coin get_coin_value
// Get coin value
//
// Get a coin's value in different coins
//
//	Consumes:
//	- application/x-www-form-urlencoded
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Responses:
//	  200: CoinValue
//	  404: RequestErrorResp
func (actions *Actions) GetCoinsValue(c *gin.Context) {
	data, err := actions.service.GetCoinsValue()
	if err != nil {
		log.Error().Err(err).
			Str("section", "actions").
			Str("action", "GetCoinsValue").
			Msg("Unable to get coins value")
		abortWithError(c, 404, "Coin not found")
		return
	}
	c.JSON(200, data)
}

func (actions *Actions) GetCoinRate(c *gin.Context) {
	coinSymbolCrypto := c.Param("crypto")
	coinSymbolFiat := c.Param("fiat")

	data, err := actions.service.GetCoinRate(coinSymbolCrypto, coinSymbolFiat)
	if err != nil {
		log.Error().Err(err).
			Str("section", "actions").
			Str("action", "GetCoinRate").
			Msg("Unable to get coin rate")
		abortWithError(c, 404, "Coin not found")
		return
	}
	c.JSON(200, data)
}

func (actions *Actions) SetCoinHighlight(c *gin.Context) {
	coinSymbol := c.Param("coin_symbol")
	switcher := c.Param("switcher")

	coin, err := actions.service.GetCoin(coinSymbol)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid coin symbol")
		return
	}

	if err := actions.service.SetCoinHighlight(coin, switcher); err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) SetCoinNewListing(c *gin.Context) {
	coinSymbol := c.Param("coin_symbol")
	switcher := c.Param("switcher")

	coin, err := actions.service.GetCoin(coinSymbol)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid coin symbol")
		return
	}

	if err := actions.service.SetCoinNewListing(coin, switcher); err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, "OK")
}
