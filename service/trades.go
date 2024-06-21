package service

import (
	"fmt"
	"github.com/ericlagergren/decimal"
	"strings"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
	"gorm.io/gorm"
)

// GetTrades for a given market
func (s *Service) GetTrades(market string, limit, page int) ([]model.PublicTrade, error) {
	trades := []model.PublicTrade{}
	db := s.repo.ConnReader.
		Table("trades").
		Where("market_id = ?", market).
		Order("seqid DESC"). // seqid fetch faster (select correct index)
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&trades)

	if db.Error != nil {
		return nil, db.Error
	}

	return trades, nil
}

// GetUserTrades for a given market
func (s *Service) GetUserTrades(market string, userID uint64, timestamp int64, sortAsc bool, limit, page int, subAccount uint64) (model.UserTrades, model.PagingMeta, error) {
	var meta model.PagingMeta
	var q *gorm.DB
	var rowCount int64 = 0
	if limit > 100 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}
	order := "id DESC"
	if sortAsc {
		order = "id ASC"
	}

	q = s.repo.ConnReader.Table("trades").Where("market_id = ?", market)
	q = q.Where("(ask_owner_id = ? AND ask_sub_account = ?) OR (bid_owner_id = ? AND bid_sub_account = ?)", userID, subAccount, userID, subAccount)

	if timestamp != 0 {
		q = q.Where("timestamp > ?", timestamp*1000000000)
	}

	// get total items
	dbc := q.Count(&rowCount)

	if dbc.Error != nil {
		return model.UserTrades{}, meta, dbc.Error
	}
	// get data
	trades := make([]model.Trade, 0, limit)
	db := q.Order(order).
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&trades)

	if db.Error != nil {
		return model.UserTrades{}, meta, db.Error
	}
	meta = model.PagingMeta{
		Page:  int(page),
		Count: rowCount,
		Limit: int(limit),
		Filter: map[string]interface{}{
			"since": timestamp,
		}}
	return model.UserTrades{Trades: trades, UserID: userID}, meta, nil
}

// GetTradesByOrders - all trades for given orders
func (service *Service) GetTradesByOrders(orders *model.OrderList, excludeSelfTrades bool) (*model.TradesForOrders, error) {
	trades := []model.Trade{}
	if len(orders.Orders) == 0 {
		return &model.TradesForOrders{Trades: trades}, nil
	}

	orderIds := make([]uint64, 0)
	for i := 0; i < len(orders.Orders); i++ {
		o := orders.Orders[i]
		orderIds = append(orderIds, o.ID)
		if o.IsCustomOrderType() {
			if o.InitOrderId != nil {
				orderIds = append(orderIds, *o.InitOrderId)
			}
			if o.TPOrderId != nil {
				orderIds = append(orderIds, *o.TPOrderId)
			}
			if o.SLOrderId != nil {
				orderIds = append(orderIds, *o.SLOrderId)
			}
		}
	}

	db := service.repo.ConnReader.Where("ask_id IN (?) OR bid_id IN (?)", orderIds, orderIds)

	if excludeSelfTrades {
		db.Where("ask_owner_id != bid_owner_id")
	}

	db = db.Order("id DESC").Find(&trades)

	if db.Error != nil {
		return nil, db.Error
	}

	return &model.TradesForOrders{OrderIDs: orderIds, Trades: trades}, nil
}

func (service *Service) GetUserTradesWithOrders(userID uint64, from, to int, subAccount uint64, excludeSelfTrade bool, limit, page int) (*model.TradeOrderList, error) {
	trades := make([]model.Trade, 0)
	var rowCount int64 = 0
	q := service.repo.ConnReader.Where("(ask_owner_id = ? AND ask_sub_account = ?) OR (bid_owner_id = ? AND bid_sub_account = ?)", userID, subAccount, userID, subAccount)
	if excludeSelfTrade {
		q = q.Where("ask_owner_id <> bid_owner_id")
	}
	if from > 0 {
		q = q.Where("created_at >= to_timestamp(?) ", from)
	}
	if to > 0 {
		q = q.Where("created_at <= to_timestamp(?) ", to)
	}
	dbc := q.Table("trades").Count(&rowCount)
	if dbc.Error != nil {
		return nil, dbc.Error
	}
	ord := "created_at DESC"
	db := q.Order(ord)
	db = db.Limit(limit).Offset((page - 1) * limit).Find(&trades)

	orderIds := make([]uint64, 0)
	for _, trade := range trades {
		orderIds = append(orderIds, trade.AskID, trade.BidID)
	}
	orders := make([]model.Order, 0)
	err := service.repo.ConnReader.Table("orders").
		Where("id in (?)", orderIds).
		Where("owner_id = ?", userID).
		Find(&orders).Error
	if err != nil {
		return nil, err
	}
	orderRootIdsMap := make(map[uint64]*model.Order, 0)
	for _, orderRootId := range orders {
		if orderRootId.RootOrderId != nil {
			orderRootIdsMap[orderRootId.ID] = &orderRootId
		}
	}

	tradeList := model.TradeOrderList{
		Trades: &model.UserTrades{Trades: trades, UserID: userID, OrdersRootIds: orderRootIdsMap},
		Orders: orders,
		Meta: model.PagingMeta{
			Page:   page,
			Count:  rowCount,
			Limit:  limit,
			Filter: make(map[string]interface{})},
	}

	return &tradeList, db.Error
}

// GetUserTradeHistory - list all [status] trades for the current user
func (service *Service) GetUserTradeHistory(userID uint64, status string, limit, page int, from, to int, side string, markets []string, query, sort string, subAccount uint64) (*model.TradeList, error) {
	trades := make([]model.Trade, 0)
	var rowCount int64 = 0
	q := service.repo.ConnReader.Where("(ask_owner_id = ? AND ask_sub_account = ?) OR (bid_owner_id = ? AND bid_sub_account = ?)", userID, subAccount, userID, subAccount)

	if from > 0 {
		q = q.Where("created_at >= to_timestamp(?) ", from)
	}
	if to > 0 {
		q = q.Where("created_at <= to_timestamp(?) ", to)
	}

	if len(side) > 0 && model.MarketSide(side) == model.MarketSide_Buy {
		q = q.Where("bid_owner_id = ?", userID)
	}

	if len(side) > 0 && model.MarketSide(side) == model.MarketSide_Sell {
		q = q.Where("ask_owner_id = ?", userID)
	}

	if len(markets) > 0 {
		q = q.Where("market_id IN (?) ", markets)
	}

	if len(status) > 0 && status == "fees" {
		q = q.Where("(ask_fee_amount > 0 OR bid_fee_amount > 0)")
	}

	if len(query) > 0 {
		queryp := "%" + query + "%"
		q = q.Where("market_id ILIKE ?", queryp)
	}

	dbc := q.Table("trades").Count(&rowCount)
	if dbc.Error != nil {
		return nil, dbc.Error
	}
	upperSort := strings.ToUpper(sort)
	if (len(sort) == 0) || ((upperSort != "ASC") && (upperSort != "DESC")) {
		upperSort = "DESC"
	}
	ord := "created_at " + upperSort
	db := q.Order(ord)
	if limit == 0 {
		db = db.Find(&trades)
	} else {
		db = db.Limit(limit).Offset((page - 1) * limit).Find(&trades)
	}

	orderIds := make([]uint64, 0)
	for _, trade := range trades {
		orderIds = append(orderIds, trade.AskID, trade.BidID)
	}
	orderRootIds := make([]model.Order, 0)

	err := service.repo.ConnReader.Table("orders").
		Select("id, type, root_order_id").
		Where("root_order_id is not null").
		Where("id in (?)", orderIds).
		Find(&orderRootIds).Error
	if err != nil {
		return nil, err
	}
	orderRootIdsMap := make(map[uint64]*model.Order, len(orderRootIds))
	for _, orderRootId := range orderRootIds {
		orderRootIdsMap[orderRootId.ID] = &orderRootId
	}

	tradeList := model.TradeList{
		Trades: &model.UserTrades{Trades: trades, UserID: userID, OrdersRootIds: orderRootIdsMap},
		Meta: model.PagingMeta{
			Page:   page,
			Count:  rowCount,
			Limit:  limit,
			Filter: make(map[string]interface{})},
	}

	tradeList.Meta.Filter["status"] = "trades"
	if len(status) > 0 && status == "fees" {
		tradeList.Meta.Filter["status"] = "fees"
	}

	return &tradeList, db.Error
}

func (service *Service) ExportUserTardes(format string, tradeData []model.Trade) (*model.GeneratedFile, error) {
	data := [][]string{}
	data = append(data, []string{"OrderID", "Date & Time", "Pair", "Side", "Fees", "Price", "Total"})
	widths := []int{45, 45, 20, 20, 45, 55, 45}

	for i := 0; i < len(tradeData); i++ {
		o := tradeData[i]
		market, err := service.GetMarketByID(o.MarketID)
		if err != nil {
			return nil, err
		}
		data = append(data, []string{
			fmt.Sprint(o.ID),
			o.CreatedAt.Format("2 Jan 2006 15:04:05"),
			fmt.Sprint(o.MarketID),
			fmt.Sprint(o.TakerSide),
			fmt.Sprint(o.AskFeeAmount.V.Quantize(market.QuotePrecision)),
			fmt.Sprint(utils.Fmt(o.Price.V)),
			fmt.Sprint(utils.Fmt(o.QuoteVolume.V))})
	}

	var resp []byte
	var err error
	title := "Trade Fees Report"

	if format == "csv" {
		resp, err = CSVExport(data)
	} else {
		resp, err = PDFExport(data, widths, title)
	}

	generatedFile := model.GeneratedFile{
		Type:     format,
		DataType: "trade",
		Data:     resp,
	}
	return &generatedFile, err
}

// ExportUserFees  - gathers trade fees data to export
func (service *Service) ExportUserFees(format, tradeType string, tradeData []model.Trade) (*model.GeneratedFile, error) {
	data := [][]string{}
	data = append(data, []string{"ID", "Date & Time", "Side", "Pair", "Amount", "Fees"})
	widths := []int{45, 45, 20, 20, 45, 55}

	for i := 0; i < len(tradeData); i++ {
		o := tradeData[i]
		var fee *decimal.Big
		coinSymbol := ""
		market, err := service.GetMarketByID(o.MarketID)
		if err != nil {
			return nil, err
		}
		if o.TakerSide == "buy" {
			fee = o.BidFeeAmount.V.Quantize(market.QuotePrecision)
			coinSymbol = market.MarketCoinSymbol
		}
		if o.TakerSide == "sell" {
			fee = o.AskFeeAmount.V.Quantize(market.QuotePrecision)
			coinSymbol = market.QuoteCoinSymbol
		}
		data = append(data, []string{
			fmt.Sprint(o.ID),
			o.CreatedAt.Format("2 Jan 2006 15:04:05"),
			fmt.Sprint(o.TakerSide),
			fmt.Sprint(o.MarketID),
			fmt.Sprint(utils.Fmt(o.Volume.V.Quantize(market.MarketPrecision))),
			fmt.Sprintf("%f %s", fee, coinSymbol)})
	}

	var resp []byte
	var err error

	title := "Trade Fees Report"

	if format == "csv" {
		resp, err = CSVExport(data)
	} else {
		resp, err = PDFExport(data, widths, title)
	}

	generatedFile := model.GeneratedFile{
		Type:     format,
		DataType: tradeType,
		Data:     resp,
	}
	return &generatedFile, err
}

// GetUserTradeHistoryByContractID - list all [status] trades by contract ID for the current user
func (service *Service) GetUserTradeHistoryByContractID(userID uint64, status string, limit, page int, from, to int, side string, markets []string, query, sort string, subAccount uint64) (*model.TradeList, error) {
	trades := make([]model.Trade, 0)
	var rowCount int64 = 0
	q := service.repo.ConnReader.Where("(ask_owner_id = ? AND ask_sub_account = ?) OR (bid_owner_id = ? AND bid_sub_account = ?)", userID, subAccount, userID, subAccount)

	if from > 0 {
		q = q.Where("created_at >= to_timestamp(?) ", from)
	}
	if to > 0 {
		q = q.Where("created_at <= to_timestamp(?) ", to)
	}

	if len(side) > 0 && model.MarketSide(side) == model.MarketSide_Buy {
		q = q.Where("bid_owner_id = ?", userID)
	}

	if len(side) > 0 && model.MarketSide(side) == model.MarketSide_Sell {
		q = q.Where("ask_owner_id = ?", userID)
	}

	if len(markets) > 0 {
		q = q.Where("market_id IN (?) ", markets)
	}

	if len(status) > 0 && status == "fees" {
		q = q.Where("(ask_fee_amount > 0 OR bid_fee_amount > 0)")
	}

	if len(query) > 0 {
		queryp := "%" + query + "%"
		q = q.Where("market_id ILIKE ?", queryp)
	}

	dbc := q.Table("trades").Count(&rowCount)
	if dbc.Error != nil {
		return nil, dbc.Error
	}
	upperSort := strings.ToUpper(sort)
	if (len(sort) == 0) || ((upperSort != "ASC") && (upperSort != "DESC")) {
		upperSort = "DESC"
	}
	ord := "created_at " + upperSort
	db := q.Order(ord)
	if limit == 0 {
		db = db.Find(&trades)
	} else {
		db = db.Limit(limit).Offset((page - 1) * limit).Find(&trades)
	}

	tradeList := model.TradeList{
		Trades: &model.UserTrades{Trades: trades, UserID: userID},
		Meta: model.PagingMeta{
			Page:   page,
			Count:  rowCount,
			Limit:  limit,
			Filter: make(map[string]interface{})},
	}

	tradeList.Meta.Filter["status"] = "trades"
	if len(status) > 0 && status == "fees" {
		tradeList.Meta.Filter["status"] = "fees"
	}

	return &tradeList, db.Error
}
