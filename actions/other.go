package actions

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	orderCache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/order"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"net/http/httputil"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	cache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/userbalance"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/httputils"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/logger"
)

// Ping godoc
// swagger:route GET /ping misc ping
// Ping
//
// Ping the server
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
//	  200: StringResp
func Ping(c *gin.Context) {
	c.JSON(200, "pong stage")
}

func abortWithError(c *gin.Context, code int, message string) {
	l := getlog(c)
	l.Debug().Stack().Int("resp_code", code).Msg(message)
	c.AbortWithStatusJSON(code, httputils.RequestError{Error: message})
}

func getUserID(c *gin.Context) (uint64, bool) {
	iUserID, ok := c.Get("auth_user_id")
	if !ok {
		return 0, false
	}
	return iUserID.(uint64), true
}

func getParentOrderID(c *gin.Context) (*uint64, bool) {
	iParentOrderId := c.PostForm("parentOrderId")
	if iParentOrderId == "" {
		return nil, false
	}
	orderId, err := strconv.ParseUint(iParentOrderId, 10, 64)
	if err != nil {
		return nil, false
	}
	return &orderId, true
}

func getOtoOrderType(c *gin.Context) (*model.OrderType, bool) {
	otoTypeS := c.PostForm("oto_type")
	if len(otoTypeS) == 0 {
		return nil, false
	}
	otoType := model.OrderType(otoTypeS)
	return &otoType, true
}

func getTsPriceType(c *gin.Context) (*model.TrailingStopPriceType, bool) {
	tsPriceTypeS := c.PostForm("ts_price_type")
	if len(tsPriceTypeS) == 0 {
		return nil, false
	}
	tsPriceType := model.TrailingStopPriceType(tsPriceTypeS)
	return &tsPriceType, true
}

func getBoolFromContext(c *gin.Context, key string) bool {
	iBool, ok := c.Get(key)
	if !ok {
		return false
	}
	return iBool.(bool)
}

func getUserRole(c *gin.Context) (string, bool) {
	role, ok := c.Get("auth_role_alias")
	if !ok {
		return "", false
	}
	return role.(string), true
}

// RequestLogger - debug every request
func (actions *Actions) RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		dump, _ := httputil.DumpRequest(c.Request, true)
		fmt.Println(string(dump))
		c.Next()
	}
}

func getlog(c *gin.Context) zerolog.Logger {
	return logger.GetLogger(c)
}

func getPagination(c *gin.Context) (int, int) {
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	return page, limit
}

// getIPFromRequest - get the first IP from request
func getIPFromRequest(ip string) string {
	if ip == "" {
		return ip
	}
	return strings.SplitAfter(ip, ",")[0]
}

// BalanceUpdate process that looking to update balance of given cache users
func (actions *Actions) BalanceUpdate(ctx context.Context, wait *sync.WaitGroup) {
	log.Info().Str("cron", "balance_update").Str("action", "start").Msg("Balance update cron - started")
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// get users map and clear it in cache
			usersToUpdate := cache.PopAll()
			// send update to each user
			for userID, accounts := range usersToUpdate {
				for accountId := range accounts {
					if account, err := subAccounts.GetUserSubAccountByID(model.MarketTypeSpot, userID, accountId); err == nil {
						actions.PublishBalanceUpdate(userID, account)
					} else {
						log.Info().Err(err).
							Str("action", "BalanceUpdate").
							Msg("Unable to trigger balance update")
					}
				}
			}
		case <-ctx.Done():
			log.Info().Str("cron", "balance_update").Str("action", "stop").Msg("6 => Balance update cron - stopped")
			wait.Done()
			return
		}
	}
}

func (actions *Actions) OrderUpdate(ctx context.Context, wait *sync.WaitGroup) {
	log.Info().Str("cron", "order_update").Str("action", "start").Msg("Order update cron - started")
	ticker := time.NewTicker(500 * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			subscribes := orderCache.GetAllSubscribes()
			for _, subAccountSubscribes := range subscribes {
				for subAccountId, userId := range subAccountSubscribes {
					// get users map and clear it in cache
					var orderUpdate []*model.Order
					if subAccountId == -1 {
						orderUpdate = orderCache.GetAllByUserId(userId)
					} else {
						orderUpdate = orderCache.GetBySubAccounts(userId, uint64(subAccountId))
					}
					// send update to each user
					for _, order := range orderUpdate {
						actions.PublishOrderUpdate(userId, subAccountId, order)
					}
				}
			}

		case <-ctx.Done():
			ticker.Stop()
			log.Info().Str("cron", "order_update").Str("action", "stop").Msg("7 => Order update cron - stopped")
			wait.Done()
			return
		}
	}
}
