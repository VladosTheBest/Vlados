package engine

import (
	model "gitlab.com/paramountdax-exchange/exchange_api_v2/data"
)

// Load the full order book from the backup object
func (book *orderBook) LoadFromOrders(marketID string, orders []model.Order, eventSeqID, tradeSeqID uint64) error {
	book.MarketID = marketID
	book.LastEventSeqID = eventSeqID
	book.LastTradeSeqID = tradeSeqID
	// book.PricePrecision = int(market.PricePrecision)
	// book.VolumePrecision = int(market.VolumePrecision)
	book.LowestAsk = 0
	book.HighestBid = 0
	book.LowestEntryPrice = 0
	book.HighestLossPrice = 0

	buyOrders := []*model.Order{}
	sellOrders := []*model.Order{}
	stopEntryOrders := []*model.Order{}
	stopLossOrders := []*model.Order{}

	for index := range orders {
		order := orders[index]
		if order.Stop == model.StopLoss_None || (order.Status != model.OrderStatus_Pending) {
			order.Stop = model.StopLoss_None
			if order.Side == model.MarketSide_Buy {
				buyOrders = append(buyOrders, &order)
				if book.HighestBid == 0 || book.HighestBid < order.Price {
					book.HighestBid = order.Price
				}
			} else {
				sellOrders = append(sellOrders, &order)
				if book.LowestAsk == 0 || book.LowestAsk > order.Price {
					book.LowestAsk = order.Price
				}
			}
		} else if order.Stop == model.StopLoss_Entry {
			stopEntryOrders = append(stopEntryOrders, &order)
			if book.LowestEntryPrice == 0 || book.LowestEntryPrice > order.StopPrice {
				book.LowestEntryPrice = order.StopPrice
			}
		} else {
			stopLossOrders = append(stopLossOrders, &order)
			if book.HighestLossPrice == 0 || book.HighestLossPrice < order.StopPrice {
				book.HighestLossPrice = order.StopPrice
			}
		}
	}

	// load limit orders
	for _, buyBookEntry := range buyOrders {
		book.addBuyBookEntry(*buyBookEntry)
	}
	for _, sellBookEntry := range sellOrders {
		book.addSellBookEntry(*sellBookEntry)
	}

	// load stop orders
	for _, order := range stopEntryOrders {
		book.StopEntryOrders.addOrder(order.StopPrice, *order)
	}
	for _, order := range stopLossOrders {
		book.StopLossOrders.addOrder(order.StopPrice, *order)
	}

	return nil
}
