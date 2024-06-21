package actions

import (
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// GetActiveMarket middleware
// - use this to limit requests to an action based on a given param
func (actions *Actions) GetActiveMarket(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		symbol := c.Param(param)
		// TODO Define a method to get this data from a list of active markets
		// that is kept in memory and updated when something changes, effectively
		// eliminating a call to the database for every call
		market, err := actions.service.GetMarketByID(symbol)
		if err != nil {
			c.AbortWithStatusJSON(404, map[string]string{"error": "Invalid or inactive market"})
			return
		}
		c.Set("data_market", market)
		c.Next()
	}
}

func getQueryAsInt(c *gin.Context, name string, def int) int {
	val := c.Query(name)
	if val == "" {
		return def
	}
	param, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return param
}

// GetMarket godoc
// swagger:route GET /markets/{pair} market get_market
// Get market
//
// Get information about a single market
//
//	Consumes:
//	- application/x-www-form-urlencoded
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Security:
//
//	Responses:
//	  200: Market
//	  404: RequestErrorResp
func (actions *Actions) GetMarket(c *gin.Context) {
	id := c.Param("market_id")
	data, err := actions.service.GetMarket(id)
	if err != nil {
		abortWithError(c, 404, "Market not found")
		return
	}
	c.JSON(200, data)
}

// GetMarkets godoc
// swagger:route GET /markets market get_markets
// Get markets
//
// Get all available markets
//
//	Consumes:
//	- application/x-www-form-urlencoded
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Security:
//
//	Responses:
//	  200: Markets
//	  500: RequestErrorResp
func (actions *Actions) GetMarkets(c *gin.Context) {
	data, err := actions.service.ListMarkets()
	if err != nil {
		abortWithError(c, 500, "Unable to get list of markets")
		return
	}
	c.JSON(200, data)
}

// GetMarkets godoc
// swagger:route GET /api/v2/cgk/markets market get_markets
// Get markets
//
// Get all available markets
//
//	Consumes:
//	- application/x-www-form-urlencoded
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Security:
//
//	Responses:
//	  200: Markets
//	  500: RequestErrorResp
func (actions *Actions) GetMarketsCGK(c *gin.Context) {
	data, err := actions.service.ListMarkets()
	if err != nil {
		abortWithError(c, 500, "Unable to get list of markets")
		return
	}

	cgkData := []*model.MarketCGK{}

	for _, market := range data {
		cgkMarket := &model.MarketCGK{
			Ticker: strings.ToUpper(market.MarketCoinSymbol + "-" + market.QuoteCoinSymbol),
			Base:   strings.ToUpper(market.MarketCoinSymbol),
			Target: strings.ToUpper(market.QuoteCoinSymbol),
		}
		cgkData = append(cgkData, cgkMarket)
	}

	c.JSON(200, cgkData)
}

// GetMarketsDetailed - Returns the list of markets
func (actions *Actions) GetMarketsDetailed(c *gin.Context) {
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	query := c.Query("query")

	data, err := actions.service.GetMarketsDetailed(limit, page, query)
	if err != nil {
		c.AbortWithStatusJSON(404, map[string]string{
			"error": "Unable to get list of markets",
		})
		return
	}
	c.JSON(200, data)
}

// AddMarket godoc
// swagger:route POST /markets admin add_market
// Add market
//
// Add a new market
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
//	      200: Market
//	      400: RequestErrorResp
//	      404: RequestErrorResp
func (actions *Actions) AddMarket(c *gin.Context) {
	log := getlog(c)
	name, _ := c.GetPostForm("name")
	baseSymbol, _ := c.GetPostForm("baseCoin")
	quoteSymbol, _ := c.GetPostForm("quoteCoin")
	status, _ := c.GetPostForm("status")
	sts, err := model.GetMarketStatusFromString(status)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid status")
		return
	}

	basePrecision, _ := c.GetPostForm("basePrecision")
	mPrec, _ := strconv.ParseInt(basePrecision, 10, 32)
	quotePrecision, _ := c.GetPostForm("quotePrecision")
	qPrec, _ := strconv.ParseInt(quotePrecision, 10, 32)

	basePrecisionFormat, _ := c.GetPostForm("basePrecisionFormat")
	mPrecFormat, _ := strconv.ParseInt(basePrecisionFormat, 10, 32)
	quotePrecisionFormat, _ := c.GetPostForm("quotePrecisionFormat")
	qPrecFormat, _ := strconv.ParseInt(quotePrecisionFormat, 10, 32)

	baseMinVolume, _ := c.GetPostForm("baseMinVolume")
	minMVol, _ := conv.NewDecimalWithPrecision().SetString(baseMinVolume)

	quoteMinVolume, _ := c.GetPostForm("quoteMinVolume")
	minQVol, _ := conv.NewDecimalWithPrecision().SetString(quoteMinVolume)

	baseMaxPrice, _ := c.GetPostForm("baseMaxPrice")
	maxMPrice, _ := conv.NewDecimalWithPrecision().SetString(baseMaxPrice)

	quoteMaxPrice, _ := c.GetPostForm("quoteMaxPrice")
	maxQPrice, _ := conv.NewDecimalWithPrecision().SetString(quoteMaxPrice)

	usdtSpendLimit, _ := c.GetPostForm("maxUsdtSpendLimit")
	maxUSDTSpendLimit, _ := conv.NewDecimalWithPrecision().SetString(usdtSpendLimit)

	data, err := actions.service.CreateMarket(name, baseSymbol, quoteSymbol, sts, int(mPrec), int(qPrec), int(mPrecFormat), int(qPrecFormat), minMVol, minQVol, maxMPrice, maxQPrice, maxUSDTSpendLimit)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "market:add").Msg("Unable to add market")
		abortWithError(c, http.StatusInternalServerError, "Unable to add market")
		return
	}
	c.JSON(http.StatusOK, data)
}

// UpdateMarket godoc
// swagger:route PUT /markets/{pair} admin update_market
// Update market
//
// Update an existing market
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
//	      200: Market
//	      400: RequestErrorResp
//	      404: RequestErrorResp
func (actions *Actions) UpdateMarket(c *gin.Context) {
	log := getlog(c)
	pair := c.Param("market_id")
	name, _ := c.GetPostForm("name")

	market, err := actions.service.GetMarketByID(pair)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Market not found")
		return
	}

	status, _ := c.GetPostForm("status")
	sts, err := model.GetMarketStatusFromString(status)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid status")
		return
	}

	basePrecision, _ := c.GetPostForm("basePrecision")
	mPrec, _ := strconv.ParseInt(basePrecision, 10, 32)

	quotePrecision, _ := c.GetPostForm("quotePrecision")
	qPrec, _ := strconv.ParseInt(quotePrecision, 10, 32)

	basePrecisionFormat, _ := c.GetPostForm("basePrecisionFormat")
	mPrecFormat, _ := strconv.ParseInt(basePrecisionFormat, 10, 32)
	quotePrecisionFormat, _ := c.GetPostForm("quotePrecisionFormat")
	qPrecFormat, _ := strconv.ParseInt(quotePrecisionFormat, 10, 32)

	baseMinVolume, _ := c.GetPostForm("baseMinVolume")
	minMVol, _ := conv.NewDecimalWithPrecision().SetString(baseMinVolume)

	quoteMinVolume, _ := c.GetPostForm("quoteMinVolume")
	minQVol, _ := conv.NewDecimalWithPrecision().SetString(quoteMinVolume)

	baseMaxPrice, _ := c.GetPostForm("baseMaxPrice")
	maxMPrice, _ := conv.NewDecimalWithPrecision().SetString(baseMaxPrice)

	quoteMaxPrice, _ := c.GetPostForm("quoteMaxPrice")
	maxQPrice, _ := conv.NewDecimalWithPrecision().SetString(quoteMaxPrice)

	usdtSpendLimit, _ := c.GetPostForm("maxUsdtSpendLimit")
	maxUSDTSpendLimit, _ := conv.NewDecimalWithPrecision().SetString(usdtSpendLimit)

	data, err := actions.service.UpdateMarket(market, name, sts, int(mPrec), int(qPrec), int(mPrecFormat), int(qPrecFormat), minMVol, minQVol, maxMPrice, maxQPrice, maxUSDTSpendLimit)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "market:update").Msg("Unable to update market")
		abortWithError(c, http.StatusInternalServerError, "Unable to update market")
		return
	}

	c.JSON(http.StatusOK, data)
}

// DeleteMarket godoc
// swagger:route DELETE /markets/{pair} admin delete_market
// Disable market
//
// Disable an existing market
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
//	      400: RequestErrorResp
//	      404: RequestErrorResp
func (actions *Actions) DeleteMarket(c *gin.Context) {
	log := getlog(c)
	pair := c.Param("market_id")
	market, err := actions.service.GetMarketByID(pair)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "market:delete").Msg("Unable to get market")
		abortWithError(c, 404, "Market not found")
		return
	}
	err = actions.service.DeleteMarket(market)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "market:delete").Msg("Unable to disable market")
		abortWithError(c, 500, "Unable to disable market")
		return
	}
	c.JSON(200, "Market has been successfully disabled")
}

// SetMarketPairFavorite godoc
// swagger:route POST /profile/favourites user set_favorite_market
// Add favorite
//
// Mark a market pair as favourite
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
//			   UserToken:
//
//	    Responses:
//	      200: FavoriteMarketPair
//	      500: RequestErrorResp
func (actions *Actions) SetMarketPairFavorite(c *gin.Context) {
	log := getlog(c)
	userID, _ := getUserID(c)
	pair, _ := c.GetPostForm("pair")
	data, err := actions.service.SetMarketPairFavorite(userID, pair)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "favourites:set").Msg("Unable to update favourites")
		abortWithError(c, 500, "Unable to update favourite market")
		return
	}
	c.JSON(200, data)
}

// GetMarketPairFavorites godoc
// swagger:route GET /profile/favourites user get_favorite_markets
// Get Favorites
//
// Get user's favourite market pairs
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
//			   UserToken:
//
//	    Responses:
//	      200: FavoriteMarketPairs
//	      500: RequestErrorResp
func (actions *Actions) GetMarketPairFavorites(c *gin.Context) {
	userID, _ := getUserID(c)
	data, err := actions.service.GetMarketPairFavorites(userID)
	if err != nil {
		abortWithError(c, 500, "Unable to get favourite markets")
		return
	}
	c.JSON(200, data)
}

func (actions *Actions) SetMarketHighlight(c *gin.Context) {
	marketID := c.Param("market_id")
	switcher := c.Param("switcher")

	market, err := actions.service.GetMarket(marketID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid market_id")
		return
	}

	if err := actions.service.SetMarketHighlight(market, switcher); err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, "OK")
}
