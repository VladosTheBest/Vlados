package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/cancelConfirmation"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/last_prices"
	marketCache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/markets"
	occ "gitlab.com/paramountdax-exchange/exchange_api_v2/data/order_canceled_confirmation"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/monitor"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/order_queues"

	"github.com/ericlagergren/decimal"
	kafkaGo "github.com/segmentio/kafka-go"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/data"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/logger"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
	"gorm.io/gorm"
)

// ErrMarketRequired godoc
var ErrMarketRequired = errors.New("At least one market must be selected for this query")

// CreateOrder and send it to the matching engine based on the given fields
func (service *Service) CreateOrder(ctx context.Context, userID uint64, market *model.Market, orderId string, previousOrder *model.Order, side model.MarketSide, orderType model.OrderType, amount, price string, stop model.OrderStop, stopPrice, tpPrice, tpRelPrice, slPrice, slRelPrice string, parentOrderId *uint64, otoType *model.OrderType, account uint64, ui model.UIType, clientOrderID, tsActivationPrice, tsPrice string, tsPriceType *model.TrailingStopPriceType) (*model.Order, error) {
	marketPrecision := uint8(market.MarketPrecision)
	quotePrecision := uint8(market.QuotePrecision)
	amountInUnits := conv.ToUnits(amount, marketPrecision)
	priceInUnits := conv.ToUnits(price, quotePrecision)
	stopPriceInUnits := conv.ToUnits(stopPrice, quotePrecision)
	tpPriceInUnits := conv.ToUnits(tpPrice, quotePrecision)
	tpRelPriceInUnits := conv.ToUnits(tpRelPrice, 4)
	slPriceInUnits := conv.ToUnits(slPrice, quotePrecision)
	slRelPriceInUnits := conv.ToUnits(slRelPrice, 4)
	tsActivationPriceInUnits := conv.ToUnits(tsActivationPrice, quotePrecision)
	amountAsDecimal := conv.NewDecimalWithPrecision()
	amountAsDecimal.SetString(conv.FromUnits(amountInUnits, marketPrecision))
	priceAsDecimal := conv.NewDecimalWithPrecision()
	priceAsDecimal.SetString(conv.FromUnits(priceInUnits, quotePrecision))
	stopPriceAsDecimal := conv.NewDecimalWithPrecision()
	stopPriceAsDecimal.SetString(conv.FromUnits(stopPriceInUnits, quotePrecision))
	tpPriceAsDecimal := conv.NewDecimalWithPrecision()
	tpPriceAsDecimal.SetString(conv.FromUnits(tpPriceInUnits, quotePrecision))
	tpRelPriceAsDecimal := conv.NewDecimalWithPrecision()
	tpRelPriceAsDecimal.SetString(conv.FromUnits(tpRelPriceInUnits, 4))
	slPriceAsDecimal := conv.NewDecimalWithPrecision()
	slPriceAsDecimal.SetString(conv.FromUnits(slPriceInUnits, quotePrecision))
	slRelPriceAsDecimal := conv.NewDecimalWithPrecision()
	slRelPriceAsDecimal.SetString(conv.FromUnits(slRelPriceInUnits, 4))
	tsActivationPriceAsDecimal := conv.NewDecimalWithPrecision()
	tsActivationPriceAsDecimal.SetString(conv.FromUnits(tsActivationPriceInUnits, quotePrecision))
	tsPriceAsDecimal := conv.NewDecimalWithPrecision()
	var tsPriceInUnits uint64
	if tsPriceType != nil {
		if *tsPriceType == model.TrailingStopPriceType_Absolute {
			tsPriceInUnits = conv.ToUnits(tsPrice, quotePrecision)
			tsPriceAsDecimal.SetString(conv.FromUnits(tsPriceInUnits, quotePrecision))
		} else {
			tsPriceInUnits = conv.ToUnits(tsPrice, 4)
			tsPriceAsDecimal.SetString(conv.FromUnits(tsPriceInUnits, 4))
		}
	}

	maxMarketPriceAsDecimal := conv.NewDecimalWithPrecision()
	maxMarketPriceAsDecimal.Copy(market.MaxMarketPrice.V)
	maxQuotePriceAsDecimal := conv.NewDecimalWithPrecision()
	maxQuotePriceAsDecimal.Copy(market.MaxQuotePrice.V)
	maxUsdtSpendLimit := conv.NewDecimalWithPrecision()
	maxUsdtSpendLimit.Copy(market.MaxUSDTSpendLimit.V)

	funds := conv.NewDecimalWithPrecision()
	lockedFunds := conv.NewDecimalWithPrecision()
	usedFunds := conv.NewDecimalWithPrecision()
	filledAmount := conv.NewDecimalWithPrecision()
	filledOppositeAmount := conv.NewDecimalWithPrecision()
	feeAmount := conv.NewDecimalWithPrecision()
	eventType := data.CommandType_NewOrder

	// create a new order with the user provided details
	order := model.NewOrder(userID, market.ID, side, orderType, stop, priceAsDecimal, amountAsDecimal, stopPriceAsDecimal, funds, lockedFunds, usedFunds, filledAmount, filledOppositeAmount, feeAmount, tpPriceAsDecimal, tpRelPriceAsDecimal, slPriceAsDecimal, slRelPriceAsDecimal, parentOrderId, otoType, account, ui, clientOrderID, tsActivationPriceAsDecimal, tsPriceAsDecimal, tsPriceType)

	if order.IsCustomOrderType() {
		if orderId != "" {
			uintOrderId, _ := strconv.ParseUint(orderId, 10, 64)
			order.ID = uintOrderId
			if previousOrder != nil && previousOrder.IsCustomOrderType() {
				if previousOrder.RootOrderId != nil {
					order.RootOrderId = previousOrder.RootOrderId
				} else {
					order.RootOrderId = &previousOrder.ID
				}
				eventType = data.CommandType_ReplaceOrder
				order.IsReplace = true
				order.PreviousLockedFunds = previousOrder.LockedFunds
				if order.IsStrangleOrStraddleOrderType() {
					order.PreviousOppositeLockedFunds = &postgres.Decimal{V: new(decimal.Big).Mul(previousOrder.TPPrice.V, previousOrder.SLAmount.V)}
				}
				order.IsInitOrderFilled = previousOrder.FilledAmount.V.Cmp(model.ZERO) > 0
			}
		}
	}

	// check if price is in limit of market max price
	if side == model.MarketSide_Buy && priceAsDecimal.Cmp(maxQuotePriceAsDecimal) != -1 {
		return order, errors.New("Maximum price allowed is " + utils.Fmt(market.MaxQuotePrice.V))
	}

	if side == model.MarketSide_Sell && priceAsDecimal.Cmp(maxMarketPriceAsDecimal) != -1 {
		return order, errors.New("Maximum price allowed is " + utils.Fmt(market.MaxMarketPrice.V))
	}

	// getting the email of the user from DB
	var orderEmail string
	err := service.repo.Conn.Table("users").Where("id = ?", order.OwnerID).Pluck("email", &orderEmail).Error
	if err != nil {
		log.Error().Err(err).Msg("error in querying DB")
		return nil, errors.New("error in querying DB")
	}

	// check if the order is from Market Maker
	if !isMMAccount(service.cfg.Server.MMAccounts, orderEmail) {
		// check if order is in limit of maximum usdt spend
		// for buy orders
		currentOrderUsdtRequirement := conv.NewDecimalWithPrecision().Mul(priceAsDecimal, amountAsDecimal)
		if side == model.MarketSide_Buy && currentOrderUsdtRequirement.Cmp(maxUsdtSpendLimit) == 1 {
			return order, errors.New("Maximum usdt spend limit allowed is " + utils.Fmt(market.MaxUSDTSpendLimit.V) + ", you are spending " + utils.Fmt(currentOrderUsdtRequirement) + "usdt for this order")
		}

		// sell orders
		currentOrderUsdtEquivalentAmount := conv.NewDecimalWithPrecision().Quo(maxUsdtSpendLimit, priceAsDecimal)
		if side == model.MarketSide_Sell && amountAsDecimal.Cmp(currentOrderUsdtEquivalentAmount) == 1 {
			return order, errors.New("Maximum usdt spend limit allowed is " + utils.Fmt(maxUsdtSpendLimit) + ", you are spending " + utils.Fmt(currentOrderUsdtRequirement) + "usdt for this order")
		}
	}
	marketPrice, err := last_prices.Get(order.MarketID)
	var marketPriceNullable *decimal.Big
	if err == nil {
		marketPriceNullable = &marketPrice
		conv.RoundToPrecision(marketPriceNullable)
	}

	logger.LogTimestamp(ctx, "pre_lock_funds", time.Now())

	// check balance, calculate locked funds and update user balance
	err = order_queues.CreateOrder(ctx, order, marketPriceNullable)
	if err != nil {
		return order, err
	}

	// add time log
	logger.LogTimestamp(ctx, "pre_kafka_send", time.Now())

	fundsInUnits := uint64(0)
	if order.Side == model.MarketSide_Buy {
		fundsInUnits = conv.ToUnits(fmt.Sprintf("%f", order.LockedFunds.V), quotePrecision)
	} else {
		fundsInUnits = conv.ToUnits(fmt.Sprintf("%f", order.LockedFunds.V), marketPrecision)
	}

	var tpPriceValue uint64
	var slPriceValue uint64
	if orderType == model.OrderType_OTO && stop == model.OrderStop_None && *otoType == model.OrderType_Market {
		tpPriceValue = tpRelPriceInUnits
		slPriceValue = slRelPriceInUnits
	} else {
		tpPriceValue = tpPriceInUnits
		slPriceValue = slPriceInUnits
	}

	// publish order on the registry
	orderEvent := data.Order{
		ID:              order.ID,
		EventType:       eventType,
		Side:            toDataSide(order.Side),
		Type:            toDataOrderType(order.Type),
		Stop:            toDataStop(order.Stop),
		Market:          order.MarketID,
		OwnerID:         order.OwnerID,
		Amount:          amountInUnits,
		Price:           priceInUnits,
		StopPrice:       stopPriceInUnits,
		Funds:           fundsInUnits,
		TakeProfitPrice: tpPriceValue,
		StopLossPrice:   slPriceValue,
		SubAccount:      order.SubAccount,
	}
	if otoType != nil {
		orderEvent.OtoType = toDataOrderType(*otoType)
	}
	if tsPriceType != nil {
		orderEvent.TrailingStopPriceType = toDataTSStop(*tsPriceType)
		orderEvent.TrailingStopActivationPrice = tsActivationPriceInUnits
		orderEvent.TrailingStopPrice = tsPriceInUnits
		//trt
	}
	bytes, err := orderEvent.ToBinary()
	if err != nil {
		return nil, err
	}

	if order.IsCustomOrderType() {
		err = service.dm.Publish("custom_orders", map[string]string{"market": market.ID}, kafkaGo.Message{Value: bytes})
	} else {
		// @done switch to internal matching engine
		// err = service.dm.Publish("orders", map[string]string{"market": market.ID}, kafkaGo.Message{Value: bytes})
		service.Markets[market.ID].Process(orderEvent)
	}

	return order, err
}

func GetDataOrderFromModel(order *model.Order, market model.Market) (data.Order, error) {
	// marketID := order.MarketID
	amount := fmt.Sprintf("%f", order.Amount.V)
	price := fmt.Sprintf("%f", order.Price.V)
	stopPrice := fmt.Sprintf("%f", order.StopPrice.V)
	tpPrice := fmt.Sprintf("%f", order.TPPrice.V)
	tpRelPrice := fmt.Sprintf("%f", order.TPRelPrice.V)
	slPrice := fmt.Sprintf("%f", order.SLPrice.V)
	slRelPrice := fmt.Sprintf("%f", order.SLRelPrice.V)
	tsActivationPrice := fmt.Sprintf("%f", order.TrailingStopActivationPrice.V)
	tsPriceType := order.TrailingStopPriceType
	tsPrice := fmt.Sprintf("%f", order.TrailingStopPrice.V)

	// market, err := marketCache.Get(marketID)
	// if err != nil {
	// 	return data.Order{}, err
	// }
	marketPrecision := uint8(market.MarketPrecision)
	quotePrecision := uint8(market.QuotePrecision)
	amountInUnits := conv.ToUnits(amount, marketPrecision)
	priceInUnits := conv.ToUnits(price, quotePrecision)
	stopPriceInUnits := conv.ToUnits(stopPrice, quotePrecision)
	tpPriceInUnits := conv.ToUnits(tpPrice, quotePrecision)
	tpRelPriceInUnits := conv.ToUnits(tpRelPrice, 4)
	slPriceInUnits := conv.ToUnits(slPrice, quotePrecision)
	slRelPriceInUnits := conv.ToUnits(slRelPrice, 4)
	tsActivationPriceInUnits := conv.ToUnits(tsActivationPrice, quotePrecision)

	var tsPriceInUnits uint64
	if tsPriceType != nil {
		if *tsPriceType == model.TrailingStopPriceType_Absolute {
			tsPriceInUnits = conv.ToUnits(tsPrice, quotePrecision)
		} else {
			tsPriceInUnits = conv.ToUnits(tsPrice, 4)
		}
	}

	filledAmountInUnits := conv.ToUnits(fmt.Sprintf("%f", order.FilledAmount.V), marketPrecision)
	eventType := data.CommandType_NewOrder

	fundPrecision := quotePrecision
	if order.Side == model.MarketSide_Buy {
		fundPrecision = quotePrecision
	} else {
		fundPrecision = marketPrecision
	}

	fundsInUnits := conv.ToUnits(fmt.Sprintf("%f", order.LockedFunds.V), fundPrecision)
	usedFundsInUnits := conv.ToUnits(fmt.Sprintf("%f", order.UsedFunds.V), fundPrecision)

	var tpPriceValue uint64
	var slPriceValue uint64
	if order.Type == model.OrderType_OTO && order.Stop == model.OrderStop_None && *order.OtoType == model.OrderType_Market {
		tpPriceValue = tpRelPriceInUnits
		slPriceValue = slRelPriceInUnits
	} else {
		tpPriceValue = tpPriceInUnits
		slPriceValue = slPriceInUnits
	}
	// publish order on the registry
	orderEvent := data.Order{
		ID:              order.ID,
		EventType:       eventType,
		Side:            toDataSide(order.Side),
		Type:            toDataOrderType(order.Type),
		Stop:            toDataStop(order.Stop),
		Market:          order.MarketID,
		OwnerID:         order.OwnerID,
		Amount:          amountInUnits,
		Price:           priceInUnits,
		StopPrice:       stopPriceInUnits,
		Funds:           fundsInUnits,
		TakeProfitPrice: tpPriceValue,
		StopLossPrice:   slPriceValue,
		SubAccount:      order.SubAccount,
		FilledAmount:    filledAmountInUnits,
		UsedFunds:       usedFundsInUnits,
		Status:          OrderStatus2DataStatus(order.Status),
	}
	if order.OtoType != nil {
		orderEvent.OtoType = toDataOrderType(*order.OtoType)
	}
	if order.TrailingStopPriceType != nil {
		orderEvent.TrailingStopPriceType = toDataTSStop(*order.TrailingStopPriceType)
		orderEvent.TrailingStopActivationPrice = tsActivationPriceInUnits
		orderEvent.TrailingStopPrice = tsPriceInUnits
	}
	return orderEvent, nil
}

func TSPriceType2ModelTSPriceType(tsPriceType model.TrailingStopPriceType) data.TrailingStopPriceTypeType {
	switch tsPriceType {
	case model.TrailingStopPriceType_Percentage:
		return data.TrailingStopPriceTypeType_Percentage
	default:
		return data.TrailingStopPriceTypeType_Absolute
	}
}

func OcoType2DataOcoType(ocoType model.OrderType) data.OrderType {
	switch ocoType {
	case model.OrderType_Limit:
		return data.OrderType_Limit
	case model.OrderType_Market:
		return data.OrderType_Market
	case model.OrderType_OCO:
		return data.OrderType_OCO
	case model.OrderType_OTO:
		return data.OrderType_OTO
	case model.OrderType_Straddle:
		return data.OrderType_Straddle
	case model.OrderType_Strangle:
		return data.OrderType_Strangle
	case model.OrderType_TrailingStop:
		return data.OrderType_TrailingStop
	}
	return data.OrderType_Limit
}

func OrderStatus2DataStatus(status model.OrderStatus) data.OrderStatus {
	switch status {
	case model.OrderStatus_PartiallyFilled:
		return data.OrderStatus_PartiallyFilled
	case model.OrderStatus_Filled:
		return data.OrderStatus_Filled
	case model.OrderStatus_Pending:
		return data.OrderStatus_Pending
	case model.OrderStatus_Untouched:
		return data.OrderStatus_Untouched
	default:
		return data.OrderStatus_Cancelled
	}
}

// GetOrderForUser for a specific user
func (service *Service) GetOrderForUser(orderID, userID uint64) (*model.Order, error) {
	order, err := service.repo.GetOrderForUser(orderID, userID)
	if err != nil {
		return nil, err
	}
	return order, nil
}

// GetOrderByID
func (service *Service) GetOrderByID(orderID uint64) (*model.Order, error) {
	return service.OMS.GetOrderByID(orderID)
}

// GetOrder - One order by marketID and orderID
func (service *Service) GetOrder(marketID string, orderID uint64) (*model.Order, error) {
	return service.OMS.GetOrder(marketID, orderID)
}

// ListMarketOrders - list all available orders for all user by selected market
func (service *Service) ListMarketOrders(marketID string, limit, page int, email, createdAt, side, status, orderType string) (*model.OrderListWithUser, error) {

	orders := make([]model.OrderWithUser, 0)
	var rowCount int64 = 0
	var usersBotID []uint64

	q := service.repo.ConnReaderAdmin

	if err := q.Table("users").
		Where("role_alias = ? OR email = ?", "bot", "marketmaker@paramountdax.com").
		Pluck("id", &usersBotID).Error; err != nil {
		return nil, err
	}

	o := q.Table("orders AS o")
	if len(usersBotID) > 0 {
		var noBotIndexCount int64
		/*
			NOTE: To have this query one needs to create it using the below command, this also needs to be run
			every time a new bot gets register or delete.
			IMPORTANT: Query will only use this index when the bot ids in the query perfectly matches the
			bots included in INDEX, e.g. 2 & 3 here in the example

			CREATE INDEX orders_backup_25052022_owner_id_idx2
			ON public.orders USING btree (owner_id)
			WHERE (owner_id <> ALL (ARRAY[(2)::bigint, (3)::bigint]));
		*/
		q.Table("pg_indexes").
			Where("tablename = 'orders' AND indexname = 'orders_bot_owner_id_exclude'").
			Count(&noBotIndexCount)

		if noBotIndexCount > 0 {
			noBots := fmt.Sprintf("(owner_id %s)", getNotAllCondition(sliceUint64ToInterface(usersBotID), "bigint"))
			o = o.Where(noBots)
		} else {
			o = o.Where("owner_id NOT IN (?)", usersBotID)
		}
	}

	q = o.Where("o.market_id = ?", marketID)

	if len(email) > 0 {
		qUserEmail := "%" + email + "%"
		var usersID []uint64

		if err := service.repo.ConnReaderAdmin.Table("users").
			Where("email LIKE ?", qUserEmail).
			Pluck("id", &usersID).Error; err != nil {
			return nil, err
		}

		q = q.Where("owner_id IN (?)", usersID)
	}

	if len(status) > 0 && model.OrderStatus(status).IsValid() {
		q = q.Where("o.status = ?", status)
	}

	if len(status) > 0 && status == "open" {
		q = q.Where("o.status in (?)", []model.OrderStatus{
			model.OrderStatus_Pending,
			model.OrderStatus_Untouched,
			model.OrderStatus_PartiallyFilled})
	}

	// search by side
	if len(side) > 0 && model.MarketSide(side).IsValid() {
		q = q.Where("o.side = ?", side)
	}
	// search by order type
	if len(orderType) > 0 && model.OrderType(orderType).IsValid() {
		q = q.Where("o.type = ?", orderType)
	}
	// search by created at date
	if len(createdAt) > 0 {
		q = q.Where("date_trunc('day', o.created_at) = ?", createdAt)
	}
	dbc := q.Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	q = q.Order("id DESC")
	if page <= 0 {
		page = 1
	}
	if limit > 100 || limit == 0 {
		limit = 100
	}

	q = q.Limit(limit).Offset((page - 1) * limit)
	db := q.Joins("LEFT JOIN users as u ON o.owner_id = u.id").
		Select("o.*, u.email as email").Find(&orders)

	if db.Error != nil {
		return nil, db.Error
	}

	orderList := model.OrderListWithUser{
		OrdersWithUser: orders,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: map[string]interface{}{},
		},
	}

	return &orderList, nil
}

func (service *Service) SetupOrderRootIds() error {
	orders := make([]*model.Order, 0)

	err := service.repo.ConnReaderAdmin.Table("orders").Find(&orders, "parent_order_id is not null").Error
	if err != nil {
		return err
	}
	orderMap := make(map[uint64]*model.Order, len(orders))

	for _, order := range orders {
		orderMap[order.ID] = order
	}
	tx := service.repo.Conn.Begin()
	for _, order := range orderMap {
		rootOrderId := service.setupRootOrderIds(order.ID, orderMap)
		if rootOrderId != order.RootOrderId {
			order.RootOrderId = rootOrderId

			err = tx.Table("orders").Where("id = ?", order.ID).Update("root_order_id", order.RootOrderId).Error
			if err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	err = tx.Commit().Error
	if err != nil {
		tx.Rollback()
		return err
	}

	return err
}

func (service *Service) setupRootOrderIds(orderId uint64, orderMap map[uint64]*model.Order) *uint64 {
	if orderMap[orderId] == nil {
		return &orderId
	} else {
		return service.setupRootOrderIds(*orderMap[orderId].ParentOrderId, orderMap)
	}
}

// ListUserOrders - list all available orders for the current user
func (service *Service) ListUserOrders(userID uint64, marketID string, status *model.OrderStatus, subAccount uint64, limit, page int) ([]model.Order, error) {
	orders := make([]model.Order, 0)
	var err error
	var db *gorm.DB

	if status != nil {
		db = service.repo.ConnReader.Where("owner_id = ? AND market_id = ? AND status = ? AND parent_order_id is null AND sub_account = ?", userID, marketID, status, subAccount)
	} else {
		db = service.repo.ConnReader.Where("owner_id = ? AND market_id = ? AND parent_order_id is null AND sub_account = ?", userID, marketID, subAccount)
	}
	db = db.Order("created_at DESC")
	if limit == 0 {
		db = db.Find(&orders)
	} else {
		db = db.Limit(limit).Offset((page - 1) * limit).Find(&orders)
	}
	err = db.Error
	return orders, err
}

// ListOrders - list all orders
func (service *Service) ListOrders(limit, page int, email, createdAt, side, status, orderType string) (*model.OrderListWithUser, error) {
	orders := make([]model.OrderWithUser, 0)
	var rowCount int64 = 0

	var botsID []uint64
	service.repo.ConnReaderAdmin.Table("users").
		Where("role_alias not in (?) OR email = ?", []string{model.Member.String(), model.Admin.String(), model.Business.String(), model.Broker.String()}, "marketmaker@paramountdax.com").
		Pluck("id", &botsID)

	q := service.repo.ConnReaderAdmin.Table("orders AS o").
		Where("owner_id NOT IN (?)", botsID)

	if len(email) > 0 {
		queryEmail := "%" + email + "%"
		var usersID []uint64

		if err := service.repo.ConnReaderAdmin.Table("users").Where("email LIKE ?", queryEmail).
			Pluck("id", &usersID).Error; err != nil {
			return nil, err
		}

		q = q.Where("owner_id IN (?)", usersID)
	}

	if len(status) > 0 && model.OrderStatus(status).IsValid() {
		q = q.Where("o.status = ?", status)
	}

	if len(status) > 0 && status == "open" {
		q = q.Where("o.status in (?)", []model.OrderStatus{model.OrderStatus_Pending, model.OrderStatus_Untouched, model.OrderStatus_PartiallyFilled})
	}

	// search by side
	if len(side) > 0 && model.MarketSide(side).IsValid() {
		q = q.Where("o.side = ?", side)
	}
	// search by order type
	if len(orderType) > 0 && model.OrderType(orderType).IsValid() {
		q = q.Where("o.type = ?", orderType)
	}
	// search by created at date
	if len(createdAt) > 0 {
		q = q.Where("date_trunc('day', o.created_at) = ?", createdAt)
	}

	dbc := q.Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	q = q.Order("created_at DESC")

	q = q.Order("id DESC")
	if page <= 0 {
		page = 1
	}
	if limit > 100 || limit == 0 {
		limit = 100
	}

	q = q.Limit(limit).Offset((page - 1) * limit)

	q = q.Joins("LEFT JOIN users as u ON o.owner_id = u.id")
	db := q.Select("o.*, u.email as email").Find(&orders)

	if db.Error != nil {
		return nil, q.Error
	}

	orderList := model.OrderListWithUser{
		OrdersWithUser: orders,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: map[string]interface{}{},
		},
	}

	return &orderList, q.Error
}

// ListUserOrdersByStatus - list all available orders for the current user
func (service *Service) ListUserOrdersByStatus(userID uint64, marketID string, status []model.OrderStatus, timestamp int64, subAccount uint64, limit, page int, orderAsc bool, timestampField string) ([]model.Order, model.PagingMeta, error) {
	var rowCount int64 = 0
	var db *gorm.DB
	var meta model.PagingMeta
	if limit > 1000 {
		limit = 1000
	}
	if page <= 0 {
		page = 1
	}

	db = service.repo.ConnReader.Table("orders").Where("owner_id = ? AND status IN (?) AND sub_account = ?", userID, status, subAccount)
	if marketID != "all" {
		db = db.Where("market_id = ?", marketID)
	}
	db = db.Where("parent_order_id is null")

	// create base query with time limit
	var since time.Time
	if timestamp != 0 {
		since = time.Unix(timestamp, 0)
		db = db.Where(fmt.Sprintf("%s > ?", timestampField), since)
	}

	// count records
	dbc := db.Count(&rowCount)

	if dbc.Error != nil {
		return nil, meta, dbc.Error
	}

	orders := make([]model.Order, 0, limit)
	if orderAsc {
		db = db.Order(fmt.Sprintf("%s ASC", timestampField))
	} else {
		db = db.Order(fmt.Sprintf("%s DESC", timestampField))
	}

	//limit = 5
	//records = 6
	//page = 2

	db = db.Limit(limit).Offset((page - 1) * limit).Find(&orders)
	meta = model.PagingMeta{
		Page:  int(page),
		Count: rowCount,
		Limit: int(limit),
		Filter: map[string]interface{}{
			"status": status,
			"since":  since,
		}}
	return orders, meta, db.Error
}

// GetUserOrders - list all [status] orders for the current user
func (service *Service) GetUserOrders(userID uint64, status string, limit, page, from, to int, markets []string, side string, subAccount uint64, ui string, clientOrderID string) (*model.OrderList, error) {
	orders := make([]model.Order, 0)
	var rowCount int64 = 0

	q := service.repo.ConnReader

	q = q.Table("orders").Where("owner_id = ? AND sub_account = ?", userID, subAccount)

	switch status {
	case "open":
		q = q.Where("status in (?)", []model.OrderStatus{model.OrderStatus_Pending, model.OrderStatus_Untouched, model.OrderStatus_PartiallyFilled})
	case "closed":
		q = q.Where("status in (?)", []model.OrderStatus{model.OrderStatus_Filled, model.OrderStatus_Cancelled})
	default:
		q = q.Where("status = ?", status)
	}

	if len(side) > 0 && (model.MarketSide(side).IsValid()) {
		q = q.Where("side = ?", side)
	}

	q = q.Where("market_id IN (?)", markets)

	if from > 0 {
		q = q.Where("created_at >= to_timestamp(?) ", from)
	}
	if to > 0 {
		q = q.Where("created_at <= to_timestamp(?) ", to)
	}

	if model.UIType(ui).IsValidUIType() {
		q = q.Where("ui = ?", ui)
	}
	if len(clientOrderID) > 0 {
		q = q.Where("client_order_id = ?", clientOrderID)
	}
	q = q.Where("parent_order_id is null")

	dbc := q.Table("orders").Count(&rowCount)

	if dbc.Error != nil {
		return nil, dbc.Error
	}

	db := q.Order("created_at DESC")
	if limit == 0 || limit > 1000 {
		limit = 1000
	}
	if page <= 0 {
		page = 1
	}

	db = db.Limit(limit).Offset((page - 1) * limit).Find(&orders)

	orderList := model.OrderList{
		Orders: orders,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: make(map[string]interface{})},
	}
	orderList.Meta.Filter["status"] = status

	return &orderList, db.Error
}

// CancelOrder  - cancels an existing order
func (service *Service) CancelOrder(ctx context.Context, market *model.Market, order *model.Order) (bool, occ.OrderCanceledStatus, error) {
	amountInUnits := conv.ToUnits(fmt.Sprintf("%f", order.Amount.V), uint8(market.MarketPrecision))
	priceInUnits := conv.ToUnits(fmt.Sprintf("%f", order.Price.V), uint8(market.QuotePrecision))
	stopPrice := fmt.Sprintf("%f", order.StopPrice.V)
	stopPriceInUnits := conv.ToUnits(stopPrice, uint8(market.QuotePrecision))

	if order.Status == model.OrderStatus_Cancelled {
		return true, occ.OrderCanceledStatus_AlreadyCancelled, nil
	}
	if order.Status == model.OrderStatus_Filled {
		return true, occ.OrderCanceledStatus_AlreadyFilled, nil
	}

	// publish order on the registry
	orderEvent := data.Order{
		ID:        order.ID,
		EventType: data.CommandType_CancelOrder,
		Side:      toDataSide(order.Side),
		Type:      toDataOrderType(order.Type),
		Stop:      toDataStop(order.Stop),
		Market:    order.MarketID,
		OwnerID:   order.OwnerID,
		Price:     priceInUnits,
		StopPrice: stopPriceInUnits,
	}
	bytes, err := orderEvent.ToBinary()
	if err != nil {
		return false, occ.OrderCanceledStatus_SendingFailed, err
	}

	if order.IsCustomOrderType() {
		err = service.dm.Publish("custom_orders", map[string]string{"market": market.ID}, kafkaGo.Message{Value: bytes})
	} else {
		// @done switch to internal matching engine
		// err = service.dm.Publish("orders", map[string]string{"market": market.ID}, kafkaGo.Message{Value: bytes})
		service.Markets[market.ID].Process(orderEvent)
		err = nil
	}
	logger.LogTimestamp(ctx, "post_send_cancel", time.Now())

	if err != nil {
		return false, occ.OrderCanceledStatus_SendingFailed, err
	}

	var isCancelled bool
	var respStatus occ.OrderCanceledStatus
	var respMsg error

	cancelManually := func(order *model.Order) error {
		fundsInUnits := uint64(0)
		if order.Side == model.MarketSide_Buy {
			fundsInUnits = conv.ToUnits(fmt.Sprintf("%f", order.LockedFunds.V), uint8(market.QuotePrecision))
		} else {
			fundsInUnits = conv.ToUnits(fmt.Sprintf("%f", order.LockedFunds.V), uint8(market.MarketPrecision))
		}

		cancelEvent := data.Event{
			Market:    order.MarketID,
			Type:      data.EventType_OrderStatusChange,
			CreatedAt: time.Now().UnixNano(),
			Payload: &data.Event_OrderStatus{
				OrderStatus: &data.OrderStatusMsg{
					ID:           order.ID,
					Type:         toDataOrderType(order.Type),
					Side:         toDataSide(order.Side),
					Price:        priceInUnits,
					Amount:       amountInUnits,
					Funds:        fundsInUnits,
					OwnerID:      order.OwnerID,
					Status:       data.OrderStatus_Cancelled,
					FilledAmount: 0,
					UsedFunds:    0,
				},
			},
		}

		cancelEventBytes, err := cancelEvent.ToBinary()
		if err != nil {
			return err
		}

		err = service.dm.Publish("events", map[string]string{"market": market.ID}, kafkaGo.Message{Value: cancelEventBytes})
		if err != nil {
			return err
		}

		return nil
	}

	internalCancelChan, err := cancelConfirmation.GetInstance().GetListenerMap(order.MarketID).GetListener(order.ID)
	if err != nil {
		return false, occ.OrderCanceledStatus_AlreadyCancelled, err
	}

	cancelChan, err := cancelConfirmation.WaitOrder(order.ID)
	if err != nil {
		return false, occ.OrderCanceledStatus_AlreadyCancelled, err
	}

	timeoutChan := time.After(10 * time.Second)

	for retriesCounter := 0; retriesCounter < 1; retriesCounter++ {
		select {
		case msg, ok := <-internalCancelChan:
			if !ok {
				break
			}
			logger.LogTimestamp(ctx, "post_wait_cancel", time.Now())

			if msg.Status == occ.OrderCanceledStatus_CancelFailedFromME {
				if err := cancelManually(order); err == nil {
					continue
				}
			} else {
				isCancelled = msg.IsCancelled
				respStatus = msg.Status
				if msg.ErrorMsg != "" {
					respMsg = errors.New(msg.ErrorMsg)
				} else {
					respMsg = nil
				}
			}
		case msg, ok := <-cancelChan:
			if !ok {
				break
			}
			logger.LogTimestamp(ctx, "post_wait_cancel", time.Now())

			if msg.Status == occ.OrderCanceledStatus_CancelFailedFromME {
				if err := cancelManually(order); err == nil {
					continue
				}
			} else {
				isCancelled = msg.IsCancelled
				respStatus = msg.Status
				if msg.ErrorMsg != "" {
					respMsg = errors.New(msg.ErrorMsg)
				} else {
					respMsg = nil
				}
			}
		case <-timeoutChan:
			if err := cancelManually(order); err == nil {
				continue
			}
		}

		if isCancelled {
			break
		}
	}

	if respMsg == nil && order.IsCustomOrderType() {
		_, err := service.ops.OMS.UpdateOrderStatus(order.MarketID, order.ID, model.OrderStatus_Cancelled, time.Now(), true)
		if err != nil {
			return false, occ.OrderCanceledStatus_CancelFailed, err
		}
	}

	cancelConfirmation.GetInstance().GetListenerMap(order.MarketID).RemoveListener(order.ID)
	cancelConfirmation.CancelWaiting(order.ID)
	monitor.OrdersCancelledCount.WithLabelValues(order.MarketID).Inc()

	return isCancelled, respStatus, respMsg
}

// CancelOrders  - cancels selected orders
func (service *Service) CancelOrders(orders []model.Order, localLogger zerolog.Logger) error {

	wg := sync.WaitGroup{}

	for _, o := range orders {
		wg.Add(1)
		go func(order model.Order) {
			defer wg.Done()

			market, err := marketCache.Get(order.MarketID)
			if err != nil {
				localLogger.Error().Err(err).Msg("Unable to get the market")
			}
			if isCancelled, statusCode, err := service.CancelOrder(context.Background(), market, &order); err != nil {
				localLogger.Error().Err(err).
					Str("status", statusCode.String()).
					Bool("isCancelled", isCancelled).
					Msg("Unable to cancel the order")
			}
		}(o)
	}
	wg.Wait()

	return nil
}

func (service *Service) CancelMarketOrders(market *model.Market, status []model.OrderStatus) error {

	orders, err := service.GetRepo().GetOrdersByMarketID(market.ID, status)

	if err != nil {
		return err
	}

	localLog := log.With().Str("market_id", market.ID).
		Int("count", len(orders)).Logger()

	localLog.Info().Msg("Start cancel orders for all markets")

	if len(orders) > 0 {
		_ = service.CancelOrders(orders, localLog)
	}

	return nil
}

// CancelUserOrders  - cancels all orders for selected user
func (service *Service) CancelUserOrders(marketID string, userID uint64, status string) error {
	orders, err := service.repo.GetOrdersByUser(marketID, userID, status)
	if err != nil {
		return err
	}

	localLog := log.With().
		Str("market_id", marketID).
		Uint64("user_id", userID).
		Str("status", status).
		Int("count", len(orders)).
		Logger()

	localLog.Info().Msg("Start user orders cancel orders for all markets")

	if len(orders) > 0 {
		_ = service.CancelOrders(orders, localLog)
	}

	return nil
}

func (service *Service) CancelUserOrdersBySubAccount(userID uint64, subAccount uint64) error {

	orders, err := service.repo.GetAllOpenedOrdersByUserSubAccount(userID, subAccount)
	if err != nil {
		return err
	}

	localLogger := log.With().
		Str("method", "CancelUserOrdersBySubAccount").
		Uint64("user_id", userID).
		Uint64("sub_Account", userID).
		Logger()

	localLogger.Info().Int("count", len(orders)).Str("stage", "Started").Msg("Trying cancel orders for all markets for user sub account")
	defer localLogger.Info().Int("count", len(orders)).Str("stage", "Finished").Msg("Trying cancel orders for all markets for user sub account")

	wg := sync.WaitGroup{}

	for _, o := range orders {
		wg.Add(1)
		go func(order model.Order) {
			defer wg.Done()

			market, err := marketCache.Get(order.MarketID)
			if err != nil {
				localLogger.Error().Err(err).Msg("Unable to get the market")
			}
			if isCancelled, statusCode, err := service.CancelOrder(context.Background(), market, &order); err != nil {
				localLogger.Error().Err(err).
					Str("status", statusCode.String()).
					Bool("isCancelled", isCancelled).
					Msg("Unable to cancel the order")
			}
		}(o)
	}
	wg.Wait()

	return nil
}

// ExportUserTrades  - gathers trades data to export
func (service *Service) ExportUserTrades(orderID uint64, format, orderType string, orderData []model.Trade) (*model.GeneratedFile, error) {
	data := [][]string{}
	data = append(data, []string{"ID", "Date & Time", "Type", "Pair", "Amount", "Price", "Fees", "Total", "Status"})
	widths := []int{45, 45, 20, 20, 45, 45, 45, 45, 25}

	for i := 0; i < len(orderData); i++ {
		o := orderData[i]
		market, err := service.GetMarketByID(o.MarketID)
		if err != nil {
			return nil, err
		}
		coinSymbol := ""
		fee := conv.NewDecimalWithPrecision()
		if o.TakerSide == "buy" {
			coinSymbol = market.MarketCoinSymbol
			fee = o.BidFeeAmount.V.Quantize(market.MarketPrecision)
		} else if o.TakerSide == "sell" {
			coinSymbol = market.QuoteCoinSymbol
			fee = o.AskFeeAmount.V.Quantize(market.MarketPrecision)
		}
		data = append(data, []string{
			fmt.Sprint(o.ID),
			o.CreatedAt.Format("2 Jan 2006 15:04:05"),
			fmt.Sprint(o.TakerSide),
			fmt.Sprint(o.MarketID),
			fmt.Sprintf("%f", o.Volume.V.Quantize(market.MarketPrecision)),
			fmt.Sprintf("%f", o.Price.V.Quantize(market.QuotePrecision)),
			fmt.Sprintf("%f %s", fee, coinSymbol),
			fmt.Sprintf("%f", o.QuoteVolume.V.Quantize(market.QuotePrecision)),
			fmt.Sprint(o.Status),
		})
	}

	var resp []byte
	var err error

	title := "Trade History Report"

	if format == "csv" {
		resp, err = CSVExport(data)
	} else {
		resp, err = PDFExport(data, widths, title)
	}

	generatedFile := model.GeneratedFile{
		Type:     format,
		DataType: orderType,
		Data:     resp,
	}
	return &generatedFile, err
}

// ExportUserOrders  - gathers data to export
func (service *Service) ExportUserOrders(orderID uint64, format, orderType string, orderData []model.Order) (*model.GeneratedFile, error) {
	data := [][]string{}
	data = append(data, []string{"ID", "Date & Time", "Type", "Side", "Pair", "Amount", "Price", "Filled", "Total", "Status"})
	widths := []int{45, 45, 15, 15, 20, 45, 45, 45, 45, 15}

	for i := 0; i < len(orderData); i++ {
		o := orderData[i]
		market, err := service.GetMarketByID(o.MarketID)
		if err != nil {
			return nil, err
		}
		data = append(data, []string{
			fmt.Sprint(o.ID),
			o.CreatedAt.Format("2 Jan 2006 15:04:05"),
			fmt.Sprint(o.Type),
			fmt.Sprint(o.Side),
			fmt.Sprint(o.MarketID),
			fmt.Sprintf("%f", o.Amount.V.Quantize(market.MarketPrecision)),
			fmt.Sprintf("%f", o.Price.V.Quantize(market.QuotePrecision)),
			fmt.Sprintf("%f", o.FilledAmount.V.Quantize(market.MarketPrecision)),
			fmt.Sprintf("%f", o.UsedFunds.V.Quantize(market.QuotePrecision)),
			fmt.Sprint(o.Status),
		})
	}

	var resp []byte
	var err error

	title := "Report"
	if orderType == "open" {
		title = "Open Orders Report"
	}
	if orderType == "closed" {
		title = "Order History Report"
	}

	if format == "csv" {
		resp, err = CSVExport(data)
	} else {
		resp, err = PDFExport(data, widths, title)
	}

	generatedFile := model.GeneratedFile{
		Type:     format,
		DataType: orderType,
		Data:     resp,
	}
	return &generatedFile, err
}

func toDataSide(side model.MarketSide) data.MarketSide {
	if side == model.MarketSide_Buy {
		return data.MarketSide_Buy
	}
	return data.MarketSide_Sell
}

func toDataStop(stop model.OrderStop) data.StopLoss {
	switch stop {
	case model.OrderStop_Loss:
		return data.StopLoss_Loss
	case model.OrderStop_Entry:
		return data.StopLoss_Entry
	default:
		return data.StopLoss_None
	}
}

func toDataTSStop(stop model.TrailingStopPriceType) data.TrailingStopPriceTypeType {
	switch stop {
	case model.TrailingStopPriceType_Percentage:
		return data.TrailingStopPriceTypeType_Percentage
	case model.TrailingStopPriceType_Absolute:
		return data.TrailingStopPriceTypeType_Absolute
	default:
		return data.TrailingStopPriceTypeType_Absolute
	}
}

func toDataOrderType(orderType model.OrderType) data.OrderType {
	switch orderType {
	case model.OrderType_Market:
		return data.OrderType_Market
	case model.OrderType_OCO:
		return data.OrderType_OCO
	case model.OrderType_OTO:
		return data.OrderType_OTO
	case model.OrderType_Strangle:
		return data.OrderType_Strangle
	case model.OrderType_Straddle:
		return data.OrderType_Straddle
	case model.OrderType_TrailingStop:
		return data.OrderType_TrailingStop
	default:
		return data.OrderType_Limit
	}
}

func (service *Service) ProcessOtoOcoOrderUpdateEvent(order *data.Order) error {
	var zero uint64
	market, err := marketCache.Get(order.Market)
	if err != nil {
		return err
	}
	oldOrder, err := service.OMS.GetOrder(order.Market, order.ID)
	if err != nil {
		oldOrder, err = service.repo.GetOrder(order.ID)
		if err != nil {
			return err
		}
	}
	amountInDecimal, _ := conv.NewDecimalWithPrecision().SetString(fmt.Sprintf("%d", order.Amount))
	slAmountInDecimal, _ := conv.NewDecimalWithPrecision().SetString(fmt.Sprintf("%d", order.SLAmount))
	tpAmountInDecimal, _ := conv.NewDecimalWithPrecision().SetString(fmt.Sprintf("%d", order.TPAmount))
	if order.FilledAmount != zero {
		filledAmountInDecimal, _ := conv.NewDecimalWithPrecision().SetString(fmt.Sprintf("%d", order.FilledAmount))
		oldOrder.FilledAmount = &postgres.Decimal{V: filledAmountInDecimal}
	}
	if order.FilledOppositeAmount != zero {
		filledOpAmountInDecimal, _ := conv.NewDecimalWithPrecision().SetString(fmt.Sprintf("%d", order.FilledOppositeAmount))
		oldOrder.FilledOppositeAmount = &postgres.Decimal{V: filledOpAmountInDecimal}
	}
	if order.FeeAmount != zero {
		feeAmountInDecimal, _ := conv.NewDecimalWithPrecision().SetString(fmt.Sprintf("%d", order.FeeAmount))
		oldOrder.FeeAmount = &postgres.Decimal{V: feeAmountInDecimal}
	}
	if order.UsedFunds != zero {
		usedFundsInDecimal, _ := conv.NewDecimalWithPrecision().SetString(fmt.Sprintf("%d", order.UsedFunds))
		oldOrder.UsedFunds = &postgres.Decimal{V: usedFundsInDecimal}
	}
	if order.LockedFunds != zero {
		lockedFundsAmountInDecimal, _ := conv.NewDecimalWithPrecision().SetString(fmt.Sprintf("%d", order.LockedFunds))
		oldOrder.LockedFunds = &postgres.Decimal{V: lockedFundsAmountInDecimal}
	}
	if order.TPFilledAmount != zero {
		tpFilledAmountInDecimal, _ := conv.NewDecimalWithPrecision().SetString(fmt.Sprintf("%d", order.TPFilledAmount))
		oldOrder.TPFilledAmount = &postgres.Decimal{V: tpFilledAmountInDecimal}
	}
	if order.SLFilledAmount != zero {
		slFilledAmountInDecimal, _ := conv.NewDecimalWithPrecision().SetString(fmt.Sprintf("%d", order.SLFilledAmount))
		oldOrder.SLFilledAmount = &postgres.Decimal{V: slFilledAmountInDecimal}
	}
	fundsInString := conv.FromUnits(order.Funds, uint8(market.MarketPrecision))
	fundsInDecimal, _ := conv.NewDecimalWithPrecision().SetString(fundsInString)

	oldOrder.TPOrderId = &order.TPOrderId
	oldOrder.SLOrderId = &order.SLOrderId
	oldOrder.RootOrderId = &order.RootOrderId
	oldOrder.Status = toModelOrderStatus(order.Status)
	oldOrder.Side = toModelSide(order.Side)
	oldOrder.Amount = &postgres.Decimal{V: amountInDecimal}
	oldOrder.SLAmount = &postgres.Decimal{V: slAmountInDecimal}
	oldOrder.TPAmount = &postgres.Decimal{V: tpAmountInDecimal}
	oldOrder.Funds = &postgres.Decimal{V: fundsInDecimal}
	tpStatus := toModelOrderStatus(order.TPStatus)
	oldOrder.TPStatus = &tpStatus
	slStatus := toModelOrderStatus(order.SLStatus)
	oldOrder.SLStatus = &slStatus
	oldOrder.UpdatedAt = time.Time{}
	err = service.ops.OMS.UpdateCustomOrder(oldOrder)
	if err != nil {
		return err
	}

	return nil
}

func toModelSide(side data.MarketSide) model.MarketSide {
	if side == data.MarketSide_Buy {
		return model.MarketSide_Buy
	}
	return model.MarketSide_Sell
}

func toModelStop(stop data.StopLoss) model.OrderStop {
	switch stop {
	case data.StopLoss_Loss:
		return model.OrderStop_Loss
	case data.StopLoss_Entry:
		return model.OrderStop_Entry
	default:
		return model.OrderStop_None
	}
}

//func toModelTSStop(stop data.TrailingStopPriceTypeType) model.TrailingStopPriceType {
//	switch stop {
//	case data.TrailingStopPriceTypeType_Percentage:
//		return model.TrailingStopPriceType_Percentage
//	case data.TrailingStopPriceTypeType_Absolute:
//		return model.TrailingStopPriceType_Absolute
//	default:
//		return model.TrailingStopPriceType_Absolute
//	}
//}

//func toModelOrderType(orderType data.OrderType) model.OrderType {
//	switch orderType {
//	case data.OrderType_Market:
//		return model.OrderType_Market
//	case data.OrderType_OCO:
//		return model.OrderType_OCO
//	case data.OrderType_OTO:
//		return model.OrderType_OTO
//	case data.OrderType_Strangle:
//		return model.OrderType_Strangle
//	case data.OrderType_Straddle:
//		return model.OrderType_Straddle
//	case data.OrderType_TrailingStop:
//		return model.OrderType_TrailingStop
//	default:
//		return model.OrderType_Limit
//	}
//}

func toModelOrderStatus(status data.OrderStatus) model.OrderStatus {
	switch status {
	case data.OrderStatus_Cancelled:
		return model.OrderStatus_Cancelled
	case data.OrderStatus_Filled:
		return model.OrderStatus_Filled
	case data.OrderStatus_Pending:
		return model.OrderStatus_Pending
	case data.OrderStatus_Untouched:
		return model.OrderStatus_Untouched
	case data.OrderStatus_PartiallyFilled:
		return model.OrderStatus_PartiallyFilled
	default:
		return model.OrderStatus_Cancelled
	}
}

func getNotAllCondition(items []interface{}, itemType string) string {
	itemsQuery := ""
	for _, item := range items {
		itemsQuery += fmt.Sprintf("(%v)::%s,", item, itemType)
	}
	if len(items) > 0 {
		itemsQuery = itemsQuery[:len(itemsQuery)-1]
	}

	return fmt.Sprintf("<> ALL (ARRAY[%s]::%s[])", itemsQuery, itemType)
}

func isMMAccount(MMAccounts []string, email string) bool {
	for _, data := range MMAccounts {
		if data == email {
			return true
		}
	}

	return false
}
