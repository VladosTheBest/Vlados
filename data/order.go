package data

import (
	// proto "github.com/golang/protobuf/proto"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"google.golang.org/protobuf/proto"
)

// NewOrder create a new order
func NewOrder(id, price, amount uint64, side MarketSide, category OrderType, eventType CommandType) Order {
	return Order{ID: id, Price: price, Amount: amount, Side: side, Type: category, EventType: eventType}
}

// Valid checks if the order is valid based on the type of the order and the price/amount/funds
func (order *Order) Valid() bool {
	if order.ID == 0 {
		return false
	}
	switch order.EventType {
	case CommandType_NewOrder:
		{
			if order.Stop != StopLoss_None {
				if order.StopPrice == 0 {
					return false
				}
			}
			switch order.Type {
			case OrderType_Limit:
				return order.Price != 0 && order.Amount != 0
			case OrderType_Market:
				return order.Funds != 0 && order.Amount != 0
			}
		}
	case CommandType_CancelOrder:
		{
			if order.Stop != StopLoss_None {
				if order.StopPrice == 0 {
					return false
				}
			}
			if order.Type == OrderType_Limit {
				return order.Price != 0
			}
		}
	}
	return true
}

// Filled checks if the order can be considered filled
func (order *Order) Filled() bool {
	if order.EventType != CommandType_NewOrder {
		return false
	}
	if order.Type == OrderType_Market && (order.Amount == 0 || order.Funds == 0) {
		return true
	}
	return false
}

// GetUnfilledAmount - get the amount of units left to be filled
func (order *Order) GetUnfilledAmount() uint64 {
	return order.Amount - order.FilledAmount
}

// GetUnusedFunds - get the remaining funds available for trading
func (order *Order) GetUnusedFunds() uint64 {
	return order.Funds - order.UsedFunds
}

//***************************
// Interface Implementations
//***************************

// SetStatus changes the status of an order if the new status has not already been set
// and it's not lower than the current set status
func (order *Order) SetStatus(status OrderStatus) {
	if order.Status < status {
		order.Status = status
	}
}

// ***************************
// Interface Implementations
// ***************************

// // LessThan implementes the skiplist interface
// func (order *Order) LessThan(other *Order) bool {
// 	return order.Price < other.Price
// }

// FromBinary loads an order from a byte array
func (order *Order) FromBinary(msg []byte) error {
	return proto.UnmarshalOptions{Merge: true}.Unmarshal(msg, order)
}

// ToBinary converts an order to a byte string
func (order *Order) ToBinary() ([]byte, error) {
	return proto.MarshalOptions{UseCachedSize: true, Deterministic: true}.Marshal(order)
}

// ToModel return the model representation of the data market side
func (side MarketSide) ToModel() model.MarketSide {
	switch side {
	case MarketSide_Buy:
		return model.MarketSide_Buy
	case MarketSide_Sell:
		return model.MarketSide_Sell
	}
	return model.MarketSide_Buy
}

// ToModel return the model representation of the order type
func (orderType OrderType) ToModel() model.OrderType {
	switch orderType {
	case OrderType_Limit:
		return model.OrderType_Limit
	case OrderType_Market:
		return model.OrderType_Market
	case OrderType_OCO:
		return model.OrderType_OCO
	case OrderType_OTO:
		return model.OrderType_OTO
	case OrderType_Strangle:
		return model.OrderType_Strangle
	case OrderType_Straddle:
		return model.OrderType_Straddle
	case OrderType_TrailingStop:
		return model.OrderType_TrailingStop
	}
	return model.OrderType_Limit
}

// ToModel return the model representation of the order status
func (status OrderStatus) ToModel() model.OrderStatus {
	switch status {
	case OrderStatus_Pending:
		return model.OrderStatus_Pending
	case OrderStatus_Untouched:
		return model.OrderStatus_Untouched
	case OrderStatus_PartiallyFilled:
		return model.OrderStatus_PartiallyFilled
	case OrderStatus_Cancelled:
		return model.OrderStatus_Cancelled
	case OrderStatus_Filled:
		return model.OrderStatus_Filled
	}
	return model.OrderStatus_Pending
}

// ToModel return the model representation of the stop loss
func (stop StopLoss) ToModel() model.OrderStop {
	switch stop {
	case StopLoss_None:
		return model.OrderStop_None
	case StopLoss_Entry:
		return model.OrderStop_Entry
	case StopLoss_Loss:
		return model.OrderStop_Loss
	}
	return model.OrderStop_None
}

func (trailingStopPriceTypeType TrailingStopPriceTypeType) ToModel() *model.TrailingStopPriceType {
	switch trailingStopPriceTypeType {
	case TrailingStopPriceTypeType_Absolute:
		model := model.TrailingStopPriceType_Absolute
		return &model
	case TrailingStopPriceTypeType_Percentage:
		model := model.TrailingStopPriceType_Percentage
		return &model
	}
	return nil
}
