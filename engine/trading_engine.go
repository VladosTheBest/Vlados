package engine

import (
	model "gitlab.com/paramountdax-exchange/exchange_api_v2/data"
)

// TradingEngine contains the current order book and information about the service since it was created
type TradingEngine interface {
	Process(order model.Order, events *[]model.Event)
	GetOrderBook() OrderBook
	LoadMarket(marketID string, orders []model.Order, eventSeqID, tradeSeqID uint64) error
	CancelOrder(order model.Order, events *[]model.Event)
	ProcessEvent(order model.Order, events *[]model.Event) interface{}
	AppendInvalidOrder(order model.Order, events *[]model.Event)
}

type tradingEngine struct {
	OrderBook OrderBook
	Symbol    string
}

// NewTradingEngine creates a new trading engine that contains an empty order book and can start receving requests
func NewTradingEngine(marketID string, pricePrecision, volumePrecision int) TradingEngine {
	orderBook := NewOrderBook(marketID, pricePrecision, volumePrecision)
	return &tradingEngine{
		OrderBook: orderBook,
	}
}

// Process a single order and returned all the events that can be satisfied instantly
func (ngin *tradingEngine) Process(order model.Order, events *[]model.Event) {
	ngin.OrderBook.Process(order, events)
}

func (ngin *tradingEngine) CancelOrder(order model.Order, events *[]model.Event) {
	ngin.OrderBook.Cancel(order, events)
}

func (ngin *tradingEngine) LoadMarket(marketID string, orders []model.Order, eventSeqID, tradeSeqID uint64) error {
	return ngin.GetOrderBook().LoadFromOrders(marketID, orders, eventSeqID, tradeSeqID)
}

func (ngin *tradingEngine) AppendInvalidOrder(order model.Order, events *[]model.Event) {
	ngin.OrderBook.AppendErrorEvent(events, model.ErrorCode_InvalidOrder, order)
}

func (ngin *tradingEngine) ProcessEvent(order model.Order, events *[]model.Event) interface{} {
	switch order.EventType {
	case model.CommandType_NewOrder:
		ngin.Process(order, events)
	case model.CommandType_CancelOrder:
		ngin.CancelOrder(order, events)
	default:
		return nil
	}
	return nil
}

func (ngin tradingEngine) GetOrderBook() OrderBook {
	return ngin.OrderBook
}
