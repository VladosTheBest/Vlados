package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/markets"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	cache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/userbalance"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	occ "gitlab.com/paramountdax-exchange/exchange_api_v2/data/order_canceled_confirmation"

	"github.com/Unleash/unleash-client-go/v3"
	ucontext "github.com/Unleash/unleash-client-go/v3/context"
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/logger"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// ErrInactiveMarket godoc
var ErrInactiveMarket = errors.New("Market is not active")
var ErrInvalidAccountParam = errors.New("Account parameter is wrong")
var ErrInvalidUIParam = errors.New("UI parameter is wrong")
var ErrDisabledOrderType = errors.New("This order type is temporarily disabled")
var ErrReplaceOrderCancelled = errors.New("Original order is cancelled, unable to replace")
var ErrReplaceOrderFilled = errors.New("Original order is filled, unable to replace")
var ErrReplaceOrderPartiallyFilled = errors.New("Original order is partially filled, unable to replace")
var ErrPreviusOrderNotFound = errors.New("Previous order not found")
var ErrPreviusOrderNotCancelled = errors.New("Unable to cancel order")

// CreateOrder godoc
// swagger:route POST /orders/{market}/{side} orders add_order
// Create order
//
// Add a new limit, market or stop order in a given market
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
//			   ApiKey:
//
//	    Responses:
//	      201: Order
//	      400: RequestErrorResp
func (actions *Actions) CreateOrder(c *gin.Context) {
	userID, _ := getUserID(c)
	actions.createOrderWithUser(c, userID)
}

// CreateOrderInternal godoc
// swagger:route POST /orders/internal/{market}/{side} orders add_order
// Create internal order
//
// Add a new limit, market or stop order in a given market
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
//			   ApiKey:
//
//	    Responses:
//	      201: Order
//	      400: RequestErrorResp
func (actions *Actions) CreateOrderInternal(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.PostForm("user"), 10, 64)
	actions.createOrderWithUser(c, userID)
}

type OptionalNumber json.Number

type OrderReq struct {
	ID        string            `json:"id" example:"31312"`
	Type      model.OrderType   `json:"type" example:"limit"`
	Status    model.OrderStatus `json:"status" example:"pending"`
	Side      model.MarketSide  `json:"side" example:"buy"`
	Amount    json.Number       `json:"amount" example:"0.3414"`
	Price     json.Number       `json:"price" example:"1.24"`
	Stop      model.OrderStop   `json:"stop" example:"none"`
	StopPrice OptionalNumber    `json:"stop_price" example:"0"`
	Funds     OptionalNumber    `json:"funds" example:"94.214"`
	MarketID  string            `json:"market_id" example:"ethbtc"`
	OwnerID   uint64            `json:"owner_id" example:"142"`

	ParentOrderId *uint64 `json:"parent_order_id" example:"313121234"`
	InitOrderId   *uint64 `json:"init_order_id" example:"313121234"`

	TPPrice        OptionalNumber     `json:"tp_price" example:"1.24"`
	TPRelPrice     OptionalNumber     `json:"tp_rel_price" example:"1.24"`
	TPAmount       OptionalNumber     `json:"tp_amount" example:"1.24"`
	TPFilledAmount OptionalNumber     `json:"tp_filled_amount" example:"1.24"`
	TPStatus       *model.OrderStatus `json:"tp_status" example:"pending"`
	TPOrderId      *uint64            `json:"tp_order_id" example:"313121234"`

	SLPrice        OptionalNumber     `json:"sl_price" example:"1.24"`
	SLRelPrice     OptionalNumber     `json:"sl_rel_price" example:"1.24"`
	SLAmount       OptionalNumber     `json:"sl_amount" example:"1.24"`
	SLFilledAmount OptionalNumber     `json:"sl_filled_amount" example:"1.24"`
	SLStatus       *model.OrderStatus `json:"sl_status" example:"pending"`
	SLOrderId      *uint64            `json:"sl_order_id" example:"313121234"`

	TrailingStopActivationPrice OptionalNumber               `json:"ts_activation_price" example:"1.24"`
	TrailingStopPrice           OptionalNumber               `json:"ts_price" example:"1.24"`
	TrailingStopPriceType       *model.TrailingStopPriceType `json:"ts_price_type" example:"percentage"`

	OtoType           *model.OrderType `json:"oto_type" example:"limit"`
	IsReplace         bool             `json:"-"`
	IsInitOrderFilled bool             `json:"-"`
	SubAccount        uint64           `json:"sub_account"`
	Account           string           `json:"account"`
	RefID             string           `json:"ref_id"`
	UI                model.UIType     `json:"ui"`
	ClientOrderID     string           `json:"client_order_id"`
}

type BulkOrderReq []OrderReq

type BulkCreateResp struct {
	Index         int          `json:"index"`
	Err           string       `json:"error"`
	Order         *model.Order `json:"order"`
	ClientOrderID string       `json:"custom_order_id"`
}

func (actions *Actions) CreateOrderBulk(c *gin.Context) {
	timeCtx, _ := c.Get("_timecontext")
	userID, _ := getUserID(c)
	ctx := timeCtx.(context.Context)
	// add time log
	logger.LogTimestamp(ctx, "action_start", time.Now())

	reqctx := c.Request.Context()
	iMarket, _ := c.Get("data_market")
	market := iMarket.(*model.Market)
	isAPIKey := getBoolFromContext(c, "auth_is_api_key")

	orderReqs := make(BulkOrderReq, 0, 120)
	_ = c.BindJSON(&orderReqs)

	replies := make([]BulkCreateResp, 0, 120)
	for index, req := range orderReqs {
		order, err := actions.createOrderFromReq(reqctx, req, market, userID, isAPIKey)
		errStr := ""
		if err != nil {
			errStr = err.Error()
			order = nil
		}
		replies = append(replies, BulkCreateResp{
			Index:         index,
			Err:           errStr,
			Order:         order,
			ClientOrderID: req.ClientOrderID,
		})
	}

	c.JSON(200, map[string]interface{}{
		"success": true,
		"message": "Orders successfully created in bulk",
		"data":    replies,
	})
}

func optionalJsonNumberToString(nr OptionalNumber) string {
	return json.Number(nr).String()
}

func (actions *Actions) createOrderFromReq(ctx context.Context, req OrderReq, market *model.Market, userID uint64, isAPIKey bool) (*model.Order, error) {
	orderType := model.OrderType(req.Type)
	side := model.MarketSide(req.Side)
	amount := req.Amount.String()
	price := req.Price.String()
	if req.Stop == "" {
		req.Stop = model.OrderStop_None
	}
	stop := model.OrderStop(req.Stop)
	// todo check if nil:
	stopPrice := optionalJsonNumberToString(req.StopPrice)
	tpPrice := optionalJsonNumberToString(req.TPPrice)
	tsPrice := optionalJsonNumberToString(req.TrailingStopPrice)
	tpRelPrice := optionalJsonNumberToString(req.TPRelPrice)
	slPrice := optionalJsonNumberToString(req.SLPrice)
	slRelPrice := optionalJsonNumberToString(req.SLRelPrice)

	parentOrderId := req.ParentOrderId
	otoType := req.OtoType
	orderId := req.ID
	ui := req.UI
	clientOrderID := req.ClientOrderID
	tsActivationPrice := optionalJsonNumberToString(req.TrailingStopActivationPrice)
	var tsPriceType *model.TrailingStopPriceType = nil
	if req.TrailingStopPriceType != nil {
		tsPriceTypeModel := model.TrailingStopPriceType(*req.TrailingStopPriceType)
		tsPriceType = &tsPriceTypeModel
	}

	if isAPIKey {
		ui = model.UIType_Api
	}

	if !ui.IsValidUIType() {
		return nil, ErrInvalidUIParam
	}

	accountID, err := subAccounts.ConvertAccountGroupToAccount(userID, req.Account)
	if err != nil {
		return nil, ErrInvalidAccountParam
	}

	// check if market is available & active
	if market.Status != model.MarketStatusActive {
		return nil, ErrInactiveMarket
	}

	fctx := ucontext.Context{
		UserId: fmt.Sprintf("%d", userID),
	}
	allowOrderType := featureflags.IsEnabled(fmt.Sprintf("api.order.allow_%s", string(orderType)), unleash.WithContext(fctx))
	if !allowOrderType {
		return nil, ErrDisabledOrderType
	}

	disableOrderTypeByCoin := featureflags.IsEnabled(fmt.Sprintf("api.order.disable_%s_%s_%s", market.ID, string(orderType), string(side)), unleash.WithContext(fctx))
	if disableOrderTypeByCoin {
		return nil, ErrDisabledOrderType
		// abortWithError(c, 400, fmt.Sprintf("%s with order type '%s' is temporarily disabled on the %s pair", strings.Title(string(side)), string(orderType), market.Name))
	}

	var previousOrder *model.Order

	if orderId != "" {
		uintOrderId, _ := strconv.ParseUint(orderId, 10, 64)
		previousOrder, err := actions.service.GetOrderByID(uintOrderId)
		if err != nil {
			return nil, ErrPreviusOrderNotFound
		}

		if !previousOrder.IsCustomOrderType() {
			// limit/stop orders replace is done here
			// in case of limit/stop order transition to oto - prevOrder should be cancelled here, new order - created by oco_oto_processor
			switch previousOrder.Status {
			case model.OrderStatus_Cancelled:
				return nil, ErrReplaceOrderCancelled
			case model.OrderStatus_Filled:
				return nil, ErrReplaceOrderFilled
			case model.OrderStatus_PartiallyFilled:
				return nil, ErrReplaceOrderPartiallyFilled
			case model.OrderStatus_Pending, model.OrderStatus_Untouched:
				isCancelled, _, err := actions.service.CancelOrder(ctx, market, previousOrder)
				if !isCancelled {
					return nil, err
				}
				freshOrder, _ := actions.service.GetOrder(market.ID, uintOrderId)
				if freshOrder != nil && freshOrder.FilledAmount.V.Cmp(conv.NewDecimalWithPrecision().SetUint64(0)) > 0 {
					return nil, ErrReplaceOrderPartiallyFilled
				}
				// if prev order is not custom order type - create new order in database instead of replacing old one
				orderId = ""
			default:
				return nil, ErrPreviusOrderNotCancelled
			}
		}
	}

	// create order in database and then publish it on apache kafka
	order, err := actions.service.CreateOrder(
		ctx,
		userID,
		market,
		orderId,
		previousOrder,
		side,
		orderType,
		amount,
		price,
		stop,
		stopPrice,
		tpPrice,
		tpRelPrice,
		slPrice,
		slRelPrice,
		parentOrderId,
		otoType,
		accountID,
		ui,
		clientOrderID,
		tsActivationPrice,
		tsPrice,
		tsPriceType,
	)
	return order, err
}

// createOrderWithUser ...
func (actions *Actions) createOrderWithUser(c *gin.Context, userID uint64) {
	timeCtx, _ := c.Get("_timecontext")
	ctx := timeCtx.(context.Context)
	// add time log
	logger.LogTimestamp(ctx, "action_start", time.Now())
	iMarket, _ := c.Get("data_market")
	market := iMarket.(*model.Market)
	orderType := model.OrderType(c.PostForm("type"))
	sideParam, _ := c.Params.Get("side")
	side := model.MarketSide(sideParam)
	amount := c.PostForm("amount")
	price := c.PostForm("price")
	stop := model.OrderStop(c.PostForm("stop"))
	stopPrice := c.PostForm("stop_price")
	tpPrice := c.PostForm("tp_price")
	tpRelPrice := c.PostForm("tp_rel_price")
	slPrice := c.PostForm("sl_price")
	slRelPrice := c.PostForm("sl_rel_price")
	parentOrderId, _ := getParentOrderID(c)
	otoType, _ := getOtoOrderType(c)
	orderId, _ := c.Params.Get("order_id")
	ui := model.UIType(c.PostForm("ui"))
	clientOrderID := c.PostForm("client_order_id")
	tsActivationPrice := c.PostForm("ts_activation_price")
	tsPrice := c.PostForm("ts_price")
	tsPriceType, _ := getTsPriceType(c)

	if price == "" {
		price = "0"
	}

	isValidPriceflag := validatePrice(price)
	if !isValidPriceflag {
		_ = c.Error(errors.New("price parameter is wrong"))
		abortWithError(c, http.StatusBadRequest, "Price parameter is wrong")
		return
	}

	if getBoolFromContext(c, "auth_is_api_key") {
		ui = model.UIType_Api
	}

	if !ui.IsValidUIType() {
		abortWithError(c, http.StatusBadRequest, "UI parameter is wrong")
		return
	}

	accountID, err := subAccounts.ConvertAccountGroupToAccount(userID, c.PostForm("account"))
	if err != nil {
		_ = c.Error(errors.New("account parameter is wrong"))
		abortWithError(c, http.StatusBadRequest, "Account parameter is wrong")
		return
	}

	c.Set("sub_account", accountID)

	// check if market is available & active
	if market.Status != model.MarketStatusActive {
		_ = c.Error(ErrInactiveMarket)
		abortWithError(c, ServerError, "Market is not active")
		return
	}

	fctx := ucontext.Context{
		UserId: fmt.Sprintf("%d", userID),
	}
	allowOrderType := featureflags.IsEnabled(fmt.Sprintf("api.order.allow_%s", string(orderType)), unleash.WithContext(fctx))
	if !allowOrderType {
		abortWithError(c, 400, "This order type is temporarily disabled")
		return
	}

	disableOrderTypeByCoin := featureflags.IsEnabled(fmt.Sprintf("api.order.disable_%s_%s_%s", market.ID, string(orderType), string(side)), unleash.WithContext(fctx))
	if disableOrderTypeByCoin {
		abortWithError(c, 400, fmt.Sprintf("%s with order type '%s' is temporarily disabled on the %s pair", strings.Title(string(side)), string(orderType), market.Name))
		return
	}

	var previousOrder *model.Order

	if orderId != "" {
		iOrder, _ := c.Get("data_order")
		previousOrder = iOrder.(*model.Order)

		if !previousOrder.IsCustomOrderType() {
			// limit/stop orders replace is done here
			// in case of limit/stop order transition to oto - prevOrder should be cancelled here, new order - created by oco_oto_processor
			switch previousOrder.Status {
			case model.OrderStatus_Cancelled:
				abortWithError(c, 400, "Original order is cancelled, unable to replace.")
				return
			case model.OrderStatus_Filled:
				abortWithError(c, 400, "Original order is filled, unable to replace.")
				return
			case model.OrderStatus_PartiallyFilled:
				abortWithError(c, 400, "Original order is partially filled, unable to replace.")
				return
			case model.OrderStatus_Pending, model.OrderStatus_Untouched:
				isCancelled, _, err := actions.service.CancelOrder(ctx, market, previousOrder)
				if !isCancelled {
					_ = c.Error(err)
					abortWithError(c, 400, "Unable to cancel order.")
					return
				}
				uintOrderId, _ := strconv.ParseUint(orderId, 10, 64)
				freshOrder, _ := actions.service.GetOrder(market.ID, uintOrderId)
				if freshOrder != nil && freshOrder.FilledAmount.V.Cmp(conv.NewDecimalWithPrecision().SetUint64(0)) > 0 {
					abortWithError(c, 400, "Original order is partially filled, unable to replace.")
					return
				}
				// if prev order is not custom order type - create new order in database instead of replacing old one
				orderId = ""
			default:
				abortWithError(c, 400, "Unable to cancel order.")
				return
			}
		}
	}

	// create order in database and then publish it on apache kafka
	order, err := actions.service.CreateOrder(ctx, userID, market, orderId, previousOrder, side, orderType, amount, price, stop, stopPrice, tpPrice, tpRelPrice, slPrice, slRelPrice, parentOrderId, otoType, accountID, ui, clientOrderID, tsActivationPrice, tsPrice, tsPriceType)
	// set user for balance update
	cache.Set(userID, accountID)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, 400, err.Error())
		return
	}
	c.JSON(201, order)
}

func validatePrice(price string) bool {
	dotFlag := false
	for _, ch := range price {
		if unicode.IsDigit(ch) {
			continue
		} else if ch == '.' && !dotFlag {
			dotFlag = true
		} else {
			return false
		}
	}

	return true
}

// ListOrders godoc
// swagger:route GET /orders/{market} orders get_orders
// List orders
//
// List orders for a user
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
//			   ApiKey:
//
//	    Responses:
//	      200: Orders
//	      500: RequestErrorResp
func (actions *Actions) ListOrders(c *gin.Context) {
	userID, _ := getUserID(c)
	marketID, _ := c.Params.Get("market")

	account, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 1000)
	orders, err := actions.service.ListUserOrders(userID, marketID, nil, account, limit, page)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, 500, err.Error())
		return
	}
	c.JSON(200, orders)
}

func (actions *Actions) ListAllOrders(c *gin.Context) {
	email, _ := c.GetQuery("email")
	orderType, _ := c.GetQuery("type")
	side, _ := c.GetQuery("side")
	status, _ := c.GetQuery("status")
	createdAt, _ := c.GetQuery("date")
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)

	orders, err := actions.service.ListOrders(limit, page, email, createdAt, side, status, orderType)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, 500, err.Error())
		return
	}

	c.JSON(200, orders)
}

func (actions *Actions) ListOpenOrders(c *gin.Context) {
	userID, _ := getUserID(c)
	accountID, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	actions.ListOpenOrdersWithUser(userID, accountID, c)
}

// ListOpenOrders godoc
func (actions *Actions) ListOpenOrdersWithUser(userID, accountID uint64, c *gin.Context) {

	marketID, _ := c.Params.Get("market")

	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 15)
	stimestamp := c.Query("since")
	itimestamp, err := strconv.ParseInt(stimestamp, 10, 64)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, 500, err.Error())
		return
	}

	timestamp := time.Unix(itimestamp, 0)
	if itimestamp > 0 && timestamp.Before(time.Now().AddDate(0, 3, 0)) {
		err := errors.New("since parameter is wrong")
		_ = c.Error(err)
		abortWithError(c, 500, err.Error())
		return
	}

	utimestamp := timestamp.Unix()

	statuses := []model.OrderStatus{
		model.OrderStatus_Pending,
		model.OrderStatus_Untouched,
		model.OrderStatus_PartiallyFilled,
	}

	ordersByMarkets := actions.service.OMS.GetOrdersByUser(userID)

	var orders = []model.Order{}
	for ordersMarketID, ordersList := range ordersByMarkets {
		// market filter
		if marketID != "all" && marketID != ordersMarketID {
			continue
		}

		for _, order := range ordersList {
			// status filter
			if !order.Status.In(statuses) {
				continue
			}
			// child order filter
			if order.ParentOrderId != nil {
				continue
			}
			// subAccount filter
			if order.SubAccount != accountID {
				continue
			}
			// timestamp filter
			if itimestamp > 0 && order.CreatedAt.Unix() >= utimestamp {
				continue
			}

			orders = append(orders, order)
		}
	}

	// sort by timestamp
	sort.Slice(orders, func(i, j int) bool {
		// need to check the way of sort
		return orders[j].CreatedAt.Before(orders[i].CreatedAt)
	})

	if err != nil {
		_ = c.Error(err)
		abortWithError(c, 500, err.Error())
		return
	}

	totalOrders := len(orders)

	// pagination
	if page-1 > 0 {
		offset := (page - 1) * limit
		if offset < totalOrders {
			orders = orders[offset:]
		}
	}

	if limit < len(orders) {
		orders = orders[:limit]
	}

	meta := model.PagingMeta{
		Page:  int(page),
		Count: int64(totalOrders),
		Limit: int(limit),
		Filter: map[string]interface{}{
			"status": statuses,
			"since":  timestamp,
		},
	}

	c.JSON(200, map[string]interface{}{
		"data": orders,
		"meta": meta,
	})
}

func (actions *Actions) ListClosedOrders(c *gin.Context) {
	userID, _ := getUserID(c)
	accountID, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	actions.ListClosedOrdersWithUser(userID, accountID, c)
}

// ListClosedOrders godoc
func (actions *Actions) ListClosedOrdersWithUser(userID, accountID uint64, c *gin.Context) {
	marketID, _ := c.Params.Get("market")

	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 15)
	stimestamp := c.Query("since")
	timestamp, err := strconv.ParseInt(stimestamp, 10, 64)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, 500, err.Error())
		return
	}
	sortAsc := false
	orders, meta, err := actions.service.ListUserOrdersByStatus(userID, marketID, []model.OrderStatus{
		model.OrderStatus_Filled,
		model.OrderStatus_Cancelled,
	}, timestamp, accountID, limit, page, sortAsc, "updated_at")
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, 500, err.Error())
		return
	}
	c.JSON(200, map[string]interface{}{
		"data": orders,
		"meta": meta,
	})
}

// ListMarketOrders godoc
// swagger:route GET /admin/orders/{market} admin get_market_orders
// List Market Orders
//
// List all orders for all users in a market
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
//	      200: ExtendedOrders
//	      500: RequestErrorResp
func (actions *Actions) ListMarketOrders(c *gin.Context) {
	marketID, _ := c.Params.Get("market")
	email, _ := c.GetQuery("email")
	typeP, _ := c.GetQuery("type")
	side, _ := c.GetQuery("side")
	status, _ := c.GetQuery("status")
	createdAt, _ := c.GetQuery("date")
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	orders, err := actions.service.ListMarketOrders(marketID, limit, page, email, createdAt, side, status, typeP)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, 500, err.Error())
		return
	}
	c.JSON(200, orders)
}

// GetUserOrdersWithTrades godoc
// swagger:route GET /users/orders orders get_orders_with_trades
// Get orders with trades
//
// List all orders of the user with trades for the selected markets
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
//	      200: OrdersWithTrades
//	      404: RequestErrorResp
//	      500: RequestErrorResp
func (actions *Actions) GetUserOrdersWithTrades(c *gin.Context) {
	userID, _ := getUserID(c)
	page, limit := getPagination(c)

	status := c.Query("status")
	marketCoinSymbol := c.Query("market_coin_symbol")
	quoteCoinSymbol := c.Query("quote_coin_symbol")
	side := c.Query("sideParam")
	ui := c.Query("ui")
	clientOrderID := c.Query("client_order_id")

	account, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	markets, err := actions.service.LoadMarketIDsByCoin(marketCoinSymbol, quoteCoinSymbol)
	if err != nil {
		log.Error().
			Str("actions", "order.go").
			Str("GetUserOrdersWithTrades", "LoadMarketIDsByCoin").
			Msg("Unable to load market id's by coin")
		abortWithError(c, http.StatusInternalServerError, err.Error())
	}

	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)
	excludeSelfTrades, _ := strconv.ParseBool(c.Query("excludeSelfTrades"))

	orders, err := actions.service.GetUserOrders(userID, status, limit, page, from, to, markets, side, account, ui, clientOrderID)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, NotFound, err.Error())
		return
	}

	trades, err := actions.service.GetTradesByOrders(orders, excludeSelfTrades)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, NotFound, err.Error())
		return
	}

	data := model.OrderListWithTrades{
		Trades: *trades,
		Orders: orders.Orders,
		Meta:   orders.Meta,
	}

	c.JSON(200, data)
}

// GetOrder stores an order based on a param ID in the current connection context
func (actions *Actions) GetOrder(restrictBy string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := getUserID(c)
		id, err := strconv.Atoi(c.Param("order_id"))
		if err != nil {
			_ = c.Error(err)
			abortWithError(c, BadRequest, "Invalid order id")
			return
		}

		order, err := actions.service.GetOrderByID(uint64(id))
		if err != nil {
			abortWithError(c, NotFound, "Order not found")
			return
		}

		if restrictBy == "user_id" {
			if order.OwnerID != userID {
				log.Error().Err(err).Uint64("orderID", order.ID).
					Uint64("orderUserID", order.OwnerID).
					Uint64("user_id", userID).Msg("Order not found")
				abortWithError(c, NotFound, "Order not found")
				return
			}
		}

		c.Set("data_order", order)
		c.Next()
	}
}

type BulkCancelResp struct {
	Index         int    `json:"index"`
	OrderID       uint64 `json:"order_id"`
	IsCancelled   bool   `json:"is_cancelled"`
	Err           string `json:"error"`
	Status        string `json:"status"`
	ClientOrderID string `json:"client_order_id"`
}

func (actions *Actions) CancelOrderBulk(c *gin.Context) {
	ctx := c.Request.Context()
	iMarket, _ := c.Get("data_market")
	market := iMarket.(*model.Market)

	orderIDs := make([]uint64, 0, 120)
	_ = c.BindJSON(&orderIDs)

	orders := make([]*model.Order, 0, len(orderIDs))
	var err error
	var order *model.Order
	for _, orderID := range orderIDs {
		order, err = actions.service.GetOrderByID(orderID)
		if err == nil {
			orders = append(orders, order)
		}
	}

	replies := make([]BulkCancelResp, 0, len(orders))
	for index, order := range orders {
		isCancelled, status, err := actions.service.CancelOrder(ctx, market, order)
		if err != nil {
			replies = append(replies, BulkCancelResp{
				Index:         index,
				IsCancelled:   isCancelled,
				Status:        status.String(),
				OrderID:       order.ID,
				ClientOrderID: order.ClientOrderID,
				Err:           err.Error(),
			})
			log.Error().Err(err).Str("method", "CancelOrderBulk").Interface("order_id", order.ID).Msg("Error cancelling order")
		} else {
			replies = append(replies, BulkCancelResp{
				Index:         index,
				IsCancelled:   isCancelled,
				Status:        status.String(),
				OrderID:       order.ID,
				ClientOrderID: order.ClientOrderID,
				Err:           "",
			})
		}
	}

	c.JSON(200, map[string]interface{}{
		"success": true,
		"data":    replies,
	})
}

// CancelOrder godoc
// swagger:route DELETE /orders/{market}/{order_id} orders cancel_order
// Cancel order
//
// Cancel order by id if not already completed
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
//			   ApiKey:
//
//	    Responses:
//	      200: SuccessMessageResp
//	      400: RequestErrorResp
//	      404: RequestErrorResp
//	      500: RequestErrorResp
func (actions *Actions) CancelOrder(c *gin.Context) {

	// Validate token - check if token is in Redis
	timeCtx, ok := c.Get("_timecontext")
	var ctx context.Context
	if ok {
		ctx = timeCtx.(context.Context)
	} else {
		ctx = context.Background()
	}

	logger.LogTimestamp(ctx, "action_start", time.Now()) // start section performance check

	iOrder, _ := c.Get("data_order")
	order := iOrder.(*model.Order)
	iMarket, _ := c.Get("data_market")
	market := iMarket.(*model.Market)

	isCancelled, statusCode, err := actions.service.CancelOrder(ctx, market, order)

	if !isCancelled && err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respCode := http.StatusOK

	if unleash.IsEnabled("api.orders.enable-cancel-order-different-codes") {
		switch statusCode {
		case occ.OrderCanceledStatus_AlreadyFilled:
			respCode = http.StatusAccepted
		case occ.OrderCanceledStatus_AlreadyCancelled:
			respCode = http.StatusAlreadyReported
		}
	}

	c.JSON(respCode, map[string]interface{}{
		"success": true,
		"message": "Order successfully cancelled",
	})
}

// ToolRevertOrder godoc
func (actions *Actions) ToolRevertOrder(c *gin.Context) {
	iOrder, _ := c.Get("data_order")
	order := iOrder.(*model.Order)
	iMarket, _ := c.Get("data_market")
	market := iMarket.(*model.Market)
	err := actions.service.RevertOrder(market, order)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, 500, "Unable to revert order")
		return
	}
	c.JSON(200, map[string]interface{}{
		"success": true,
		"message": "Order successfully added in the revert order queue",
	})
}

// CancelUserOrders cancels all orders for the selected user
func (actions *Actions) CancelUserOrdersForAllMarkets(c *gin.Context) {

	id := c.Param("user_id")
	userID, err := strconv.Atoi(id)

	if err != nil {
		abortWithError(c, 404, "Unable to cancel orders")
		return
	}

	log.Info().Int("user_id", userID).Msg("Start cancel orders for all markets")

	go actions.cancelUserOrdersForAllMarkets(userID)

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Orders cancelling process successfully started",
	})
}

func (actions *Actions) cancelUserOrdersForAllMarkets(userID int) {
	statuses := []model.OrderStatus{
		model.OrderStatus_Pending,
		model.OrderStatus_Untouched,
		model.OrderStatus_PartiallyFilled,
	}

	for _, market := range markets.GetAll() {
		for _, status := range statuses {
			if err := actions.service.CancelUserOrders(market.ID, uint64(userID), status.String()); err != nil {
				log.Error().Err(err).Str("market_id", market.ID).Int("user_id", userID).Msg("Cancel orders for all markets")
			}
		}
	}
}

func (actions *Actions) CancelUserOrders(c *gin.Context) {
	marketID := c.Param("market_id")
	status := c.Param("status")
	id := c.Param("user_id")
	userID, _ := strconv.Atoi(id)

	if marketID == "" {
		abortWithError(c, 404, "Invalid market id")
		return
	}
	if status == "" {
		abortWithError(c, 404, "Invalid status")
		return
	}

	err := actions.service.CancelUserOrders(marketID, uint64(userID), status)
	if err != nil {
		abortWithError(c, 404, "Unable to cancel orders")
		return
	}

	c.JSON(200, map[string]interface{}{
		"success": true,
		"message": "Orders successfully cancelled",
	})
}

// CancelMarketOrders cancels all orders for the selected market
func (actions *Actions) CancelMarketOrders(c *gin.Context) {
	iMarket, _ := c.Get("data_market")
	market := iMarket.(*model.Market)
	if market.Status == model.MarketStatusActive {
		abortWithError(c, 404, "Unable to cancel orders, market is active")
		return
	}

	go func(m *model.Market) {
		status := []model.OrderStatus{
			model.OrderStatus_Pending,
			model.OrderStatus_Untouched,
			model.OrderStatus_PartiallyFilled,
		}
		err := actions.service.CancelMarketOrders(m, status)
		if err != nil {
			abortWithError(c, 404, "Unable to cancel orders")
			return
		}
	}(market)

	c.JSON(200, map[string]interface{}{
		"success": true,
		"message": "Orders successfully cancelled",
	})
}

// ExportUserOrders godoc
// swagger:route GET /users/orders/export orders export_orders_with_trades
// Export orders or trades
//
// Exports data either for CSV or PDF
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
//	      200: GeneratedFile
//	      500: RequestErrorResp
func (actions *Actions) ExportUserOrders(c *gin.Context) {
	userID, _ := getUserID(c)
	format := c.Query("format")
	status := c.Query("status")
	side := c.Query("sideParam")
	sort := c.Query("sort")
	marketCoinSymbol := c.Query("market_coin_symbol")
	quoteCoinSymbol := c.Query("quote_coin_symbol")
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)
	ui := c.Query("ui")
	clientOrderID := c.Query("client_order_id")

	account, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	markets, err := actions.service.LoadMarketIDsByCoin(marketCoinSymbol, quoteCoinSymbol)
	if err != nil {
		log.Error().
			Str("actions", "order.go").
			Str("GetUserOrdersWithTrades", "LoadMarketIDsByCoin").
			Msg("Unable to load market id's by coin")
		abortWithError(c, http.StatusInternalServerError, err.Error())
	}

	if status == "trades" {
		// 0 limit to get all data, no paging
		trades, err := actions.service.GetUserTradeHistory(userID, status, 1000, 1, from, to, side, markets, "", sort, account)
		if err != nil {
			abortWithError(c, 500, err.Error())
			return
		}

		data, err := actions.service.ExportUserTrades(userID, format, status, trades.Trades.Trades)
		if err != nil {
			abortWithError(c, 500, "Unable to export data")
			return
		}
		c.JSON(200, data)
	} else {
		// 0 limit to get all data, no paging
		orders, err := actions.service.GetUserOrders(userID, status, 1000, 1, from, to, markets, side, account, ui, clientOrderID)
		if err != nil {

			abortWithError(c, 500, err.Error())
			return
		}
		data, err := actions.service.ExportUserOrders(userID, format, status, orders.Orders)
		if err != nil {
			abortWithError(c, 500, "Could not export")
			return
		}
		c.JSON(200, data)
	}
}

// CancelUserOrdersForAllMarketsByUser cancels all users orders by user
func (actions *Actions) CancelUserOrdersForAllMarketsByUser(c *gin.Context) {

	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusUnauthorized, "user not found")
		return
	}

	go actions.cancelUserOrdersForAllMarkets(int(userID))

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Orders cancelling process successfully started",
	})
}

// CancelUserOrdersByMarketsByUser cancels all users orders by market
func (actions *Actions) CancelUserOrdersByMarketsByUser(c *gin.Context) {

	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusUnauthorized, "user not found")
		return
	}

	iMarket, _ := c.Get("data_market")
	market := iMarket.(*model.Market)

	go func(m *model.Market) {
		statuses := []model.OrderStatus{
			model.OrderStatus_Pending,
			model.OrderStatus_Untouched,
			model.OrderStatus_PartiallyFilled,
		}
		for _, status := range statuses {
			err := actions.service.CancelUserOrders(m.ID, userID, status.String())
			if err != nil {
				abortWithError(c, http.StatusInternalServerError, "Unable to cancel orders, error = "+err.Error())
				return
			}
		}
	}(market)

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Orders cancelling process successfully started",
	})
}
