package actions

import (
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// GetChains godoc
// swagger:route GET /chains admin get_chains
// Returns the list of chains
//
// Get the list of all active chains available in the exchange.
//
//	    Consumes:
//	    - application/x-www-form-urlencoded
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
//	      200: Chains
//	      500: RequestErrorResp
func (actions *Actions) GetChains(c *gin.Context) {

	log := getlog(c)
	source, _ := c.GetQuery("source")

	var err error
	var data interface{}
	if source == "admin" {
		data, err = actions.service.ListChainsUnrestricted()
	} else {
		data, err = actions.service.ListChains()
	}
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "chains:get").Msg("Unable to get list of chains")
		abortWithError(c, 500, "Unable to get chains")
		return
	}
	c.JSON(200, data)
}

// AddChain godoc
// swagger:route POST /chains admin add_chain
// Add a new chain
//
// Create a new chain entity in the exchange
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
//	      200: Chain
//	      500: RequestErrorResp
func (actions *Actions) AddChain(c *gin.Context) {
	log := getlog(c)
	symbol, _ := c.GetPostForm("symbol")
	name, _ := c.GetPostForm("name")
	status, _ := c.GetPostForm("status")

	sts, err := model.GetStatusFromString(status)
	if err != nil {
		abortWithError(c, 404, "Invalid status")
		return
	}
	data, err := actions.service.AddChain(symbol, name, sts)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "chains:add").Msg("Unable to add chain")
		abortWithError(c, 500, "Unable to add chain")
		return
	}
	c.JSON(200, data)
}

// UpdateChain godoc
// swagger:route PUT /chains/{symbol} admin update_chain
// Update an existing chain
//
// Update an existing chain
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
//	      200: Chain
//	      404: RequestErrorResp
//	      500: RequestErrorResp
func (actions *Actions) UpdateChain(c *gin.Context) {
	log := getlog(c)
	symbol := c.Param("chain_symbol")
	name, _ := c.GetPostForm("name")
	status, _ := c.GetPostForm("status")

	//  get chain from database
	chain, err := actions.service.GetChain(symbol)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "chains:update").Msg("Unable to get chain")
		abortWithError(c, 404, "Chain not found")
		return
	}
	if name == "" {
		name = chain.Name
	}
	sts := chain.Status
	if status != "" {
		sts, err = model.GetStatusFromString(status)
		if err != nil {
			abortWithError(c, 404, "Invalid status")
			return
		}
	}
	data, err := actions.service.UpdateChain(chain, name, sts)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "chains:update").Msg("Unable to update chain")
		abortWithError(c, 404, "Unable to update chain")
		return
	}
	c.JSON(200, data)
}

// DeleteChain godoc
// swagger:route DELETE /chains/{symbol} admin delete_chain
// Set chain to inactive
//
// Set chain to inactive
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
func (actions *Actions) DeleteChain(c *gin.Context) {
	log := getlog(c)
	symbol := c.Param("chain_symbol")
	chain, err := actions.service.GetChain(symbol)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "chains:delete").Msg("Unable to get chain")
		abortWithError(c, 404, "Unable to disable chain")
		return
	}
	err = actions.service.DeleteChain(chain)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "chains:delete").Msg("Unable to disable chain")
		abortWithError(c, 404, "Unable to disable chain")
		return
	}
	c.JSON(200, "Chain has been successfully disabled")
}

// GetChain godoc
// swagger:route GET /chains/{symbol} admin get_chain
// Get details about a single chain
//
// Get details about a single chain
//
//	    Consumes:
//	    - application/x-www-form-urlencoded
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
//	      200: Coin
//	      404: RequestErrorResp
func (actions *Actions) GetChain(c *gin.Context) {
	chainSymbol := c.Param("chain_symbol")
	data, err := actions.service.GetChain(chainSymbol)
	if err != nil {
		abortWithError(c, 404, "Chain not found")
		return
	}
	c.JSON(200, data)
}
