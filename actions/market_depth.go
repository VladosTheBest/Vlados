package actions

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/markets"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/monitor"
)

func (actions *Actions) DepthLevel2(c *gin.Context) {
	start := time.Now()

	iMarket, _ := c.Get("data_market")
	market := iMarket.(*model.Market)

	limit := getQueryAsInt(c, "limit", 200)
	if limit > 200 {
		limit = 200
	}

	depth, err := actions.service.OMS.GetMarketDepthLevel2(market, limit)
	if err != nil {
		log.Error().Err(err).Str("marketID", market.ID).Msg("Unable to get market depth")
		abortWithError(c, http.StatusInternalServerError, "Unable to get market depth")
		return
	}

	c.JSON(http.StatusOK, depth)

	monitor.MarketDepthRequestDelay.WithLabelValues(market.ID, "level2").Set(float64(time.Since(start)))
	monitor.MarketDepthRequestCount.WithLabelValues(market.ID, "level2").Inc()
}

func (actions *Actions) DepthLevel1(c *gin.Context) {
	start := time.Now()

	iMarket, _ := c.Get("data_market")
	market := iMarket.(*model.Market)

	depth, err := actions.service.OMS.GetMarketDepthLevel1(market)
	if err != nil {
		log.Error().Err(err).Str("marketID", market.ID).Msg("Unable to get market depth")
		abortWithError(c, http.StatusInternalServerError, "Unable to get market depth")
		return
	}

	c.JSON(http.StatusOK, depth)

	monitor.MarketDepthRequestDelay.WithLabelValues(market.ID, "level1").Set(float64(time.Since(start)))
	monitor.MarketDepthRequestCount.WithLabelValues(market.ID, "level1").Inc()
}

func (actions *Actions) DepthLevel2GTK(c *gin.Context) {
	start := time.Now()

	marketID := c.Query("ticker_id")
	internalMarketID := strings.ToLower(strings.ReplaceAll(marketID, "-", ""))

	market, err := markets.Get(internalMarketID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, map[string]string{"error": "Invalid or inactive market"})
		return
	}

	depth, err := actions.service.OMS.GetMarketDepthLevel2(market, -1)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, map[string]string{"error": "Unable to get market depth"})
		return
	}

	lvlCgk := model.MarketDepthLevel2CGK{
		Asks: make([][2]string, 0),
		Bids: make([][2]string, 0),
	}

	lvlCgk.TickerID = marketID
	lvlCgk.Timestamp = depth.Timestamp
	lvlCgk.Asks = depth.Asks
	lvlCgk.Bids = depth.Bids

	c.JSON(http.StatusOK, lvlCgk)

	monitor.MarketDepthRequestDelay.WithLabelValues(marketID, "level2Cgk").Set(float64(time.Since(start)))
	monitor.MarketDepthRequestCount.WithLabelValues(marketID, "level2Cgk").Inc()
}
