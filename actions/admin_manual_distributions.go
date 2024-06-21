package actions

import (
	"encoding/json"
	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// GetAdminDistributions godoc
func (actions *Actions) GetAdminDistributions(c *gin.Context) {
	page, limit := getPagination(c)
	data, err := actions.service.GetManualDistributionEvents(limit, page)
	if err != nil {
		c.AbortWithStatusJSON(500, map[string]string{"error": "Get manual distribution events", "error_tip": ""})
		return
	}
	c.JSON(200, data)
}

// GetAdminDistribution godoc
func (actions *Actions) GetAdminDistribution(c *gin.Context) {
	distID, _ := strconv.Atoi(c.Param("distribution_id"))
	data, err := actions.service.GetManualDistributionByID(uint64(distID))
	if err != nil {
		c.AbortWithStatusJSON(500, map[string]string{"error": "Unable to get distribution by id", "error_tip": ""})
		return
	}
	c.JSON(200, data)
}

// CompleteAdminDistribution godoc
func (actions *Actions) CompleteAdminDistribution(c *gin.Context) {
	distID, _ := strconv.Atoi(c.Param("distribution_id"))
	value, _ := c.GetPostForm("value")

	valueDec, setted := conv.NewDecimalWithPrecision().SetString(value)
	if !setted {
		valueDec = conv.NewDecimalWithPrecision().SetFloat64(0)
	}

	distribution, err := actions.service.GetManualDistributionByID(uint64(distID))
	if err != nil {
		c.AbortWithStatusJSON(500, map[string]string{"error": "Unable to get distribution by id", "error_tip": ""})
		return
	}
	if distribution.Status == model.DistributionStatus_Accepted || distribution.Status == model.DistributionStatus_Completed {
		c.AbortWithStatusJSON(500, map[string]string{"error": "Distribution already accepted or completed", "error_tip": ""})
		return
	}

	if userId, found := getUserID(c); found {
		distribution.CompletedByUserId = userId
	}

	distribution.Status = model.DistributionStatus_Accepted
	distribution.UpdatedAt = time.Now()
	distribution.MMPercent = &postgres.Decimal{V: valueDec}
	if err := actions.service.GetRepo().Update(distribution); err != nil {
		c.AbortWithStatusJSON(500, map[string]string{"error": "Distribution already accepted or completed", "error_tip": "", "error_msg": err.Error()})
		return
	}

	go func() {
		err := actions.service.GetRepo().ManualDistributionAccept(distribution.ID, actions.cfg.Distribution.Coin, actions.cfg.Distribution.BotID)
		if err != nil {
			log.Error().
				Err(err).
				Str("section", "manual_distribution").
				Str("method", "CompleteAdminDistribution").
				Uint64("distribution_id", distribution.ID).
				Str("coin_symbol", actions.cfg.Distribution.Coin).
				Uint64("bot_id", actions.cfg.Distribution.BotID).
				Msg("Unable to accept distribution")
		}
	}()

	c.JSON(200, distribution)
}

// GetAdminDistributionFunds godoc
func (actions *Actions) GetAdminDistributionFunds(c *gin.Context) {
	distributionID, _ := strconv.Atoi(c.Param("distribution_id"))
	data, err := actions.service.GetManualDistributionFunds(uint64(distributionID))
	if err != nil {
		c.AbortWithStatusJSON(500, map[string]string{"error": "Get manual distribution funds", "error_tip": ""})
		return
	}
	c.JSON(200, data)
}

// UpdateFundsItemRequest godoc
type UpdateFundsItemRequest struct {
	ID               uint64 `json:"id"`
	ConvertedBalance string `json:"converted_balance"`
}

// UpdateFundsRequest godoc
type UpdateFundsRequest struct {
	Funds []UpdateFundsItemRequest `json:"funds"`
}

// UpdateAdminDistributionFunds godoc
func (actions *Actions) UpdateAdminDistributionFunds(c *gin.Context) {
	distributionID, _ := strconv.Atoi(c.Param("distribution_id"))
	data, err := actions.service.GetManualDistributionFunds(uint64(distributionID))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Unable to get manual distribution funds", "error_tip": ""})
		return
	}
	reqFunds := UpdateFundsRequest{}
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return
	}
	_ = json.Unmarshal(body, &reqFunds)
	funds := map[uint64]model.ManualDistributionFund{}
	for _, fund := range data.DistributionFunds {
		funds[fund.ID] = fund
	}
	for i := range reqFunds.Funds {
		reqfund := reqFunds.Funds[i]
		fund := funds[reqfund.ID]
		_, isSetted := fund.ConvertedBalance.V.SetString(reqfund.ConvertedBalance)
		if !isSetted {
			c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Unable to set converted balances. Please fill all field on the form", "error_tip": "", "error_msg": err.Error()})
			return
		}

		fund.ConvertedCoinSymbol = actions.cfg.Distribution.Coin
		fund.Status = "completed"
		fund.UpdatedAt = time.Now()
		if err = actions.service.GetRepo().Update(fund); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Unable to get manual distribution funds", "error_tip": "", "error_msg": err.Error()})
			return
		}
	}
	data, err = actions.service.GetManualDistributionFunds(uint64(distributionID))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Unable to get manual distribution funds", "error_tip": ""})
		return
	}
	c.JSON(200, data)
}

// GetAdminDistributionBalances godoc
func (actions *Actions) GetAdminDistributionBalances(c *gin.Context) {
	page, limit := getPagination(c)
	level := c.Query("level")
	userEmail := c.Query("query")
	distributionID, _ := strconv.Atoi(c.Param("distribution_id"))
	data, err := actions.service.GetManualDistributionBalances(uint64(distributionID), page, limit, userEmail, level)
	if err != nil {
		c.AbortWithStatusJSON(500, map[string]string{"error": "Get manual distribution balances", "error_tip": err.Error()})
		return
	}
	c.JSON(200, data)
}
