package model

/*
 * Copyright © 2018-2019 Around25 SRL <office@around25.com>
 *
 * Licensed under the Around25 Wallet License Agreement (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.around25.com/licenses/EXCHANGE_LICENSE
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * @author		Cosmin Harangus <cosmin@around25.com>
 * @copyright 2018-2019 Around25 SRL <office@around25.com>
 * @license 	EXCHANGE_LICENSE
 */

import (
	"errors"
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

var Zero = conv.NewDecimalWithPrecision()
var ErrOrder_InsufficientFunds = errors.New("Insufficient funds")
var ErrOrder_OrderTooSmall = errors.New("Minimum order threashold not reached - %s %s")
var ErrOrder_OrderStatusSame = errors.New("status the same with previous")
var ErrOrder_OrderStatusInvalid = errors.New("invalid order status")
var ErrOrder_OrderStopPriceInvalid = errors.New("invalid order stop price")
var ErrOrder_OrderPriceInvalid = errors.New("invalid order price")
var ErrOrder_OrderAmountInvalid = errors.New("invalid order amount")

type OrderStatus string

const (
	OrderStatus_Pending         OrderStatus = "pending"
	OrderStatus_Untouched       OrderStatus = "untouched"
	OrderStatus_PartiallyFilled OrderStatus = "partially_filled"
	OrderStatus_Cancelled       OrderStatus = "cancelled"
	OrderStatus_Filled          OrderStatus = "filled"
)

var (
	OrderStatusesToPending         []OrderStatus = []OrderStatus{}
	OrderStatusesToUntouched       []OrderStatus = []OrderStatus{OrderStatus_Pending}
	OrderStatusesToPartiallyFilled []OrderStatus = []OrderStatus{OrderStatus_Pending, OrderStatus_Untouched}
	OrderStatusesToCancelled       []OrderStatus = []OrderStatus{OrderStatus_Pending, OrderStatus_Untouched, OrderStatus_PartiallyFilled}
	OrderStatusesToFilled          []OrderStatus = []OrderStatus{OrderStatus_Pending, OrderStatus_Untouched, OrderStatus_PartiallyFilled}
)

var (
	orderStatusesTransition = map[OrderStatus]map[OrderStatus]bool{}
)

func init() {
	orderStatusesTransition[OrderStatus_Pending] = map[OrderStatus]bool{
		OrderStatus_Untouched:       true,
		OrderStatus_PartiallyFilled: true,
		OrderStatus_Filled:          true,
		OrderStatus_Cancelled:       true,
	}
	orderStatusesTransition[OrderStatus_Untouched] = map[OrderStatus]bool{
		OrderStatus_PartiallyFilled: true,
		OrderStatus_Filled:          true,
		OrderStatus_Cancelled:       true,
	}
	orderStatusesTransition[OrderStatus_PartiallyFilled] = map[OrderStatus]bool{
		OrderStatus_Filled:    true,
		OrderStatus_Cancelled: true,
	}
	orderStatusesTransition[OrderStatus_Filled] = map[OrderStatus]bool{}
	orderStatusesTransition[OrderStatus_Cancelled] = map[OrderStatus]bool{}
}

func (os OrderStatus) IsValid() bool {
	switch os {
	case OrderStatus_Pending,
		OrderStatus_Untouched,
		OrderStatus_PartiallyFilled,
		OrderStatus_Cancelled,
		OrderStatus_Filled:
		return true
	default:
		return false
	}
}

func (os OrderStatus) IsValidChange(ns OrderStatus) error {
	if os == ns {
		return ErrOrder_OrderStatusSame
	}
	ok := orderStatusesTransition[os][ns]

	if !ok {
		return ErrOrder_OrderStatusInvalid
	}

	return nil
}

func (os OrderStatus) In(statuses []OrderStatus) bool {
	for _, status := range statuses {
		if status == os {
			return true
		}
	}
	return false
}

//func (u *OrderStatus) Scan(value interface{}) error { *u = OrderStatus(value.([]byte)); return nil }

func (u OrderStatus) String() string {
	return string(u)
}

//func (u OrderStatus) Value() (driver.Value, error) { return string(u), nil }

type OrderType string

const (
	OrderType_Limit        OrderType = "limit"
	OrderType_Market       OrderType = "market"
	OrderType_OCO          OrderType = "oco"
	OrderType_OTO          OrderType = "oto"
	OrderType_Strangle     OrderType = "strangle"
	OrderType_Straddle     OrderType = "straddle"
	OrderType_TrailingStop OrderType = "trailing_stop"
)

func (ot OrderType) String() string {
	return string(ot)
}

func (ot OrderType) IsValid() bool {
	switch ot {
	case OrderType_Limit,
		OrderType_Market,
		OrderType_OCO,
		OrderType_OTO,
		OrderType_Strangle,
		OrderType_Straddle,
		OrderType_TrailingStop:
		return true
	default:
		return false
	}
}

type MarketSide string

const (
	MarketSide_Buy  MarketSide = "buy"
	MarketSide_Sell MarketSide = "sell"
)

func (ms MarketSide) IsValid() bool {
	switch ms {
	case MarketSide_Buy,
		MarketSide_Sell:
		return true
	default:
		return false
	}
}

type OrderStop string

const (
	OrderStop_None  OrderStop = "none"
	OrderStop_Loss  OrderStop = "loss"
	OrderStop_Entry OrderStop = "entry"
)

type OtoOrderType string

const (
	OtoOrderType_Limit  OtoOrderType = "limit"
	OtoOrderType_Market OtoOrderType = "market"
	OtoOrderType_Stop   OtoOrderType = "stop"
)

type TrailingStopPriceType string

const (
	TrailingStopPriceType_Percentage TrailingStopPriceType = "percentage"
	TrailingStopPriceType_Absolute   TrailingStopPriceType = "absolute"
)

// Order structure
type Order struct {
	ID        uint64            `sql:"type:bigint" gorm:"PRIMARY_KEY" json:"id" wire:"id" example:"31312"`
	Type      OrderType         `sql:"not null;type:order_type_t;default:limit" json:"type" wire:"type" example:"limit"`
	Status    OrderStatus       `sql:"not null;type:order_status_t;default:'pending'" json:"status" wire:"status" example:"pending"`
	Side      MarketSide        `sql:"not null;type:market_side_t;default:'buy'" json:"side" wire:"side" example:"buy"`
	Amount    *postgres.Decimal `sql:"type:decimal(36,18)" json:"amount" wire:"amount" example:"0.3414"`
	Price     *postgres.Decimal `sql:"type:decimal(36,18)" json:"price" wire:"price" example:"1.24"`
	Stop      OrderStop         `sql:"not null;type:order_stoploss_t;default:'none'" json:"stop" wire:"stop" example:"none"`
	StopPrice *postgres.Decimal `sql:"type:decimal(36,18)" json:"stop_price" wire:"stop_price" example:"0"`
	Funds     *postgres.Decimal `sql:"type:decimal(36,18)" json:"funds" wire:"funds" example:"94.214"`
	MarketID  string            `sql:"type:varchar(10) REFERENCES markets(id)" json:"market_id" wire:"market_id" example:"ethbtc"`
	OwnerID   uint64            `sql:"type:bigint REFERENCES users(id)" json:"owner_id" wire:"owner_id" example:"142"`

	LockedFunds          *postgres.Decimal `sql:"type:decimal(36,18)" json:"locked_funds" wire:"locked_funds" example:"1.1224"`
	UsedFunds            *postgres.Decimal `sql:"type:decimal(36,18)" json:"used_funds" wire:"used_funds" example:"0.412"`
	FilledAmount         *postgres.Decimal `sql:"type:decimal(36,18)" json:"filled_amount" wire:"filled_amount" example:"0.123"`
	FilledOppositeAmount *postgres.Decimal `sql:"type:decimal(36,18)" json:"filled_opposite_amount" wire:"filled_opposite_amount" example:"0.123"`
	FeeAmount            *postgres.Decimal `sql:"type:decimal(36,18)" json:"fee_amount" wire:"fee_amount" example:"0.00031"`

	CreatedAt time.Time `json:"created_at" wire:"created_at" example:"2019-08-28T22:12:34"`
	UpdatedAt time.Time `json:"updated_at" wire:"updated_at" example:"2019-08-28T22:12:34"`

	ParentOrderId *uint64 `sql:"type:bigint" json:"parent_order_id" wire:"parent_order_id" example:"313121234"`
	InitOrderId   *uint64 `sql:"type:bigint" json:"init_order_id" wire:"init_order_id" example:"313121234"`

	TPPrice        *postgres.Decimal `gorm:"column:tp_price" sql:"type:decimal(36,18)" json:"tp_price" wire:"tp_price" example:"1.24"`
	TPRelPrice     *postgres.Decimal `gorm:"column:tp_rel_price" sql:"type:decimal(36,18)" json:"tp_rel_price" wire:"tp_rel_price" example:"1.24"`
	TPAmount       *postgres.Decimal `gorm:"column:tp_amount" sql:"type:decimal(36,18)" json:"tp_amount" wire:"tp_amount" example:"1.24"`
	TPFilledAmount *postgres.Decimal `gorm:"column:tp_filled_amount" sql:"type:decimal(36,18)" json:"tp_filled_amount" wire:"tp_filled_amount" example:"1.24"`
	TPStatus       *OrderStatus      `gorm:"column:tp_status" sql:"type:order_status_t" json:"tp_status" wire:"tp_status" example:"pending"`
	TPOrderId      *uint64           `gorm:"column:tp_order_id" sql:"type:bigint" json:"tp_order_id" wire:"tp_order_id" example:"313121234"`

	SLPrice        *postgres.Decimal `gorm:"column:sl_price" sql:"type:decimal(36,18)" json:"sl_price" wire:"sl_price" example:"1.24"`
	SLRelPrice     *postgres.Decimal `gorm:"column:sl_rel_price" sql:"type:decimal(36,18)" json:"sl_rel_price" wire:"sl_rel_price" example:"1.24"`
	SLAmount       *postgres.Decimal `gorm:"column:sl_amount" sql:"column:sl_amount;type:decimal(36,18)" json:"sl_amount" wire:"sl_amount" example:"1.24"`
	SLFilledAmount *postgres.Decimal `gorm:"column:sl_filled_amount" sql:"type:decimal(36,18)" json:"sl_filled_amount" wire:"sl_filled_amount" example:"1.24"`
	SLStatus       *OrderStatus      `gorm:"column:sl_status" sql:"type:order_status_t" json:"sl_status" wire:"sl_status" example:"pending"`
	SLOrderId      *uint64           `gorm:"column:sl_order_id" sql:"type:bigint" json:"sl_order_id" wire:"sl_order_id" example:"313121234"`

	TrailingStopActivationPrice *postgres.Decimal      `sql:"column:trailing_stop_activation_price;null;type:decimal(36,18)" json:"ts_activation_price" wire:"trailing_stop_activation_price" example:"1.24"`
	TrailingStopPrice           *postgres.Decimal      `sql:"column:trailing_stop_price;null;type:decimal(36,18)" json:"ts_price" wire:"trailing_stop_price" example:"1.24"`
	TrailingStopPriceType       *TrailingStopPriceType `sql:"column:trailing_stop_price_type;null;type:trailing_stop_price_type_t" json:"ts_price_type" wire:"trailing_stop_price_type" example:"percentage"`

	OtoType                     *OrderType        `sql:"type:order_type_t;default:limit" json:"oto_type" wire:"oto_type" example:"limit"`
	IsReplace                   bool              `gorm:"-" json:"-"`
	IsInitOrderFilled           bool              `gorm:"-" json:"-"`
	OppositeFunds               *postgres.Decimal `gorm:"-" json:"-"`
	OppositeLockedFunds         *postgres.Decimal `gorm:"-" json:"-"`
	PreviousOppositeLockedFunds *postgres.Decimal `gorm:"-" json:"-"`
	PreviousLockedFunds         *postgres.Decimal `gorm:"-" json:"-"`
	SubAccount                  uint64            `sql:"sub_account" json:"sub_account" wire:"sub_account"`
	RefID                       string            `gorm:"column:ref_id" json:"ref_id" wire:"ref_id"`
	UI                          UIType            `sql:"not null;type:ui_type_t;default:'web-basic'" json:"ui" wire:"ui"`
	ClientOrderID               string            `gorm:"column:client_order_id" json:"client_order_id" wire:"client_order_id"`
	RootOrderId                 *uint64           `gorm:"column:root_order_id" json:"root_order_id" wire:"root_order_id"`
}

func (o *Order) IsValidForME(isCancel bool) bool {
	if o.ID == 0 {
		return false
	}

	if !isCancel {
		if o.Stop != OrderStop_None {
			if o.StopPrice.V == Zero {
				return false
			}
		}
		switch o.Type {
		case OrderType_Limit:
			return o.Price.V != Zero && o.Amount.V != Zero
		case OrderType_Market:
			return o.LockedFunds.V != Zero && o.Amount.V != Zero
		}
	}

	if o.Stop != OrderStop_None {
		if o.StopPrice.V == Zero {
			return false
		}
		if o.Type == OrderType_Limit {
			return o.Price.V != Zero
		}
	}

	return true
}
func (o *Order) IsActive() bool {
	return o.Status != OrderStatus_Filled && o.Status != OrderStatus_Cancelled
}

// OrderList structure
type OrderList struct {
	Orders []Order
	Meta   PagingMeta
}

// OrderListWithTrades structure
type OrderListWithTrades struct {
	Trades TradesForOrders
	Orders []Order
	Meta   PagingMeta
}

// OrderWithUser structure
type OrderWithUser struct {
	Order
	UserEmail string `gorm:"column:email" json:"user_email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// OrderListWithUser structure
type OrderListWithUser struct {
	OrdersWithUser []OrderWithUser `json:"orders_with_user"`
	Meta           PagingMeta      `json:"meta"`
}

func NewOrder(userID uint64, marketID string, side MarketSide, orderType OrderType, stop OrderStop, price, amount, stopPrice, funds, lockedFunds, usedFunds, filledAmount, filledOppositeAmount, feeAmount, tpPriceAsDecimal, tpRelPriceAsDecimal, slPriceAsDecimal, slRelPriceAsDecimal *decimal.Big, parentOrderId *uint64, otoType *OrderType, subAccount uint64, ui UIType, clientOrderID string, tsActivationPriceAsDecimal, tsPriceAsDecimal *decimal.Big, tsPriceType *TrailingStopPriceType) *Order {
	return &Order{
		Status:                      OrderStatus_Pending,
		Side:                        side,
		Type:                        orderType,
		MarketID:                    marketID,
		OwnerID:                     userID,
		Amount:                      &postgres.Decimal{V: amount},
		Price:                       &postgres.Decimal{V: price},
		StopPrice:                   &postgres.Decimal{V: stopPrice},
		Stop:                        stop,
		Funds:                       &postgres.Decimal{V: funds}, // @todo add user funds from the database
		LockedFunds:                 &postgres.Decimal{V: lockedFunds},
		UsedFunds:                   &postgres.Decimal{V: usedFunds},
		FilledAmount:                &postgres.Decimal{V: filledAmount},
		FilledOppositeAmount:        &postgres.Decimal{V: filledOppositeAmount},
		FeeAmount:                   &postgres.Decimal{V: feeAmount},
		ParentOrderId:               parentOrderId,
		TPPrice:                     &postgres.Decimal{V: tpPriceAsDecimal},
		TPRelPrice:                  &postgres.Decimal{V: tpRelPriceAsDecimal},
		TPAmount:                    &postgres.Decimal{V: nil},
		TPFilledAmount:              &postgres.Decimal{V: nil},
		TPStatus:                    nil,
		SLPrice:                     &postgres.Decimal{V: slPriceAsDecimal},
		SLRelPrice:                  &postgres.Decimal{V: slRelPriceAsDecimal},
		SLAmount:                    &postgres.Decimal{V: nil},
		SLFilledAmount:              &postgres.Decimal{V: nil},
		SLStatus:                    nil,
		OtoType:                     otoType,
		SubAccount:                  subAccount,
		OppositeFunds:               &postgres.Decimal{V: nil},
		OppositeLockedFunds:         &postgres.Decimal{V: nil},
		UI:                          ui,
		ClientOrderID:               clientOrderID,
		TrailingStopActivationPrice: &postgres.Decimal{V: tsActivationPriceAsDecimal},
		TrailingStopPrice:           &postgres.Decimal{V: tsPriceAsDecimal},
		TrailingStopPriceType:       tsPriceType,
	}
}

// Model Methods

// CalculateFunds - Update funds and locked funds based on the available balance for the required coin
func (order *Order) CalculateFunds(balance *decimal.Big, marketPrice *decimal.Big) {
	order.Funds.V = conv.NewDecimalWithPrecision().Copy(balance)

	if order.Side == MarketSide_Sell {
		order.LockedFunds.V = conv.NewDecimalWithPrecision().Copy(order.Amount.V)
		if order.IsStrangleOrStraddleOrderType() {
			order.OppositeLockedFunds.V = conv.NewDecimalWithPrecision().Mul(order.TPPrice.V, order.Amount.V)
			conv.RoundToPrecision(order.OppositeLockedFunds.V)
		}
		conv.RoundToPrecision(order.LockedFunds.V)
		return
	}

	switch order.Type {
	case OrderType_Limit,
		OrderType_OCO:
		order.LockedFunds.V = conv.NewDecimalWithPrecision().Mul(order.Price.V, order.Amount.V)
	case OrderType_Market:
		if order.Stop == OrderStop_None {
			order.LockedFunds.V = order.CalculateLockedFundsForMarketOrder(balance, order.Amount.V, marketPrice)
		} else {
			order.LockedFunds.V = order.CalculateLockedFundsForMarketOrder(balance, order.Amount.V, order.StopPrice.V)
		}
	case OrderType_OTO:
		switch *order.OtoType {
		case OrderType_Limit:
			order.LockedFunds.V = conv.NewDecimalWithPrecision().Mul(order.Price.V, order.Amount.V)
		case OrderType_Market:
			if order.Stop == OrderStop_None {
				order.LockedFunds.V = conv.NewDecimalWithPrecision().Copy(order.Funds.V)
			} else {
				order.LockedFunds.V = conv.NewDecimalWithPrecision().Mul(order.StopPrice.V, order.Amount.V)
			}
		}
	}

	conv.RoundToPrecision(order.LockedFunds.V)
}

func (order *Order) CanCalculateFundsWithoutBalance() bool {
	if order.Side == MarketSide_Sell {
		return true
	}
	if order.Type == OrderType_Limit || order.Type == OrderType_OCO {
		return true
	}
	if order.Type == OrderType_OTO && *order.OtoType == OrderType_Limit {
		return true
	}
	if order.Type == OrderType_OTO && *order.OtoType == OrderType_Market && order.Stop != OrderStop_None {
		return true
	}
	return false
}

func (order *Order) CalculateFundsForSellOrLimit() {
	order.Funds.V = conv.NewDecimalWithPrecision()

	if order.Side == MarketSide_Sell {
		order.LockedFunds.V = conv.NewDecimalWithPrecision().Copy(order.Amount.V)
		if order.IsStrangleOrStraddleOrderType() {
			order.OppositeLockedFunds.V = conv.NewDecimalWithPrecision().Mul(order.TPPrice.V, order.Amount.V)
			conv.RoundToPrecision(order.OppositeLockedFunds.V)
		}
		conv.RoundToPrecision(order.LockedFunds.V)
		return
	}

	switch order.Type {
	case OrderType_Limit,
		OrderType_OCO:
		order.LockedFunds.V = conv.NewDecimalWithPrecision().Mul(order.Price.V, order.Amount.V)
	case OrderType_OTO:
		switch *order.OtoType {
		case OrderType_Limit:
			order.LockedFunds.V = conv.NewDecimalWithPrecision().Mul(order.Price.V, order.Amount.V)
		case OrderType_Market:
			if order.Stop != OrderStop_None {
				order.LockedFunds.V = conv.NewDecimalWithPrecision().Mul(order.StopPrice.V, order.Amount.V)
			}
		}
	}

	conv.RoundToPrecision(order.LockedFunds.V)
}

func (order *Order) CalculateFundsForBuyOrMarket(balance *decimal.Big, marketPrice *decimal.Big) {
	order.Funds.V = conv.NewDecimalWithPrecision().Copy(balance)

	switch order.Type {
	case OrderType_Market:
		{
			if order.Stop == OrderStop_None {
				order.LockedFunds.V = order.CalculateLockedFundsForMarketOrder(balance, order.Amount.V, marketPrice)
			} else {
				order.LockedFunds.V = order.CalculateLockedFundsForMarketOrder(balance, order.Amount.V, order.StopPrice.V)
			}
		}
	case OrderType_OTO:
		{
			if *order.OtoType == OrderType_Market {
				if order.Stop == OrderStop_None {
					order.LockedFunds.V = conv.NewDecimalWithPrecision().Copy(order.Funds.V)
				} else {
					order.LockedFunds.V = conv.NewDecimalWithPrecision().Mul(order.StopPrice.V, order.Amount.V)
				}
			}
		}
	default:
		return
	}
	conv.RoundToPrecision(order.LockedFunds.V)
}

func (order *Order) CalculateLockedFundsForMarketOrder(balance, amount, price *decimal.Big) *decimal.Big {
	if price != nil {
		totalValue := conv.NewDecimalWithPrecision().Mul(price, amount)
		totalValueWithMultiplier := conv.NewDecimalWithPrecision().Mul(totalValue, conv.NewDecimalWithPrecision().SetFloat64(1.1))
		if totalValueWithMultiplier.Cmp(balance) > 0 {
			if totalValue.Cmp(balance) <= 0 {
				return totalValue
			} else {
				return conv.NewDecimalWithPrecision().Copy(balance)
			}
		} else {
			return totalValueWithMultiplier
		}
	} else {
		return conv.NewDecimalWithPrecision().Copy(balance)
	}
}

// IsValidTransition checks if the order can move to the given status
func (order *Order) IsValidTransition(status OrderStatus) bool {
	if order.Status == status {
		return false
	}
	switch order.Status {
	case OrderStatus_Pending:
		return true
	case OrderStatus_Untouched:
		return status == OrderStatus_PartiallyFilled || status == OrderStatus_Filled || status == OrderStatus_Cancelled
	case OrderStatus_PartiallyFilled:
		return status == OrderStatus_Filled || status == OrderStatus_Cancelled
	}
	return false
}

func (order *Order) IsValidAgainstBalance(available *decimal.Big, market *Market) error {
	minMarketVolume := market.MinMarketVolume.V
	// minQuoteVolume := market.MinQuoteVolume.V
	// Check if the user has the necessary funds to execute the order
	if order.IsCustomOrderType() && order.IsReplace {
		if order.IsInitOrderFilled {
			return nil
		}
		if conv.NewDecimalWithPrecision().Add(available, order.PreviousLockedFunds.V).Cmp(order.LockedFunds.V) == -1 {
			return ErrOrder_InsufficientFunds
		}
		if order.IsStrangleOrStraddleOrderType() {
			if conv.NewDecimalWithPrecision().Add(order.OppositeFunds.V, order.PreviousOppositeLockedFunds.V).Cmp(order.OppositeLockedFunds.V) == -1 {
				return ErrOrder_InsufficientFunds
			}
		}
	} else if available.Cmp(order.LockedFunds.V) == -1 || (!order.IsCustomOrderType() && order.LockedFunds.V.Cmp(Zero) != 1) {
		return ErrOrder_InsufficientFunds
	}

	if order.Type == OrderType_Limit || order.Type == OrderType_Market {
		if order.Stop != OrderStop_None {
			if order.StopPrice.V.Cmp(Zero) == 0 {
				return ErrOrder_OrderStopPriceInvalid
			}
			if order.Amount.V.Cmp(Zero) <= 0 {
				return ErrOrder_OrderAmountInvalid
			}
		}

		if order.Amount.V.Cmp(Zero) <= 0 {
			return ErrOrder_OrderAmountInvalid
		}

		switch order.Type {
		case OrderType_Limit:
			if order.Price.V.Cmp(Zero) <= 0 {
				return ErrOrder_OrderPriceInvalid
			}
		case OrderType_Market:
			if available.Cmp(Zero) <= 0 {
				return ErrOrder_OrderPriceInvalid
			}
		}
	}

	if order.IsStrangleOrStraddleOrderType() && order.OppositeFunds.V.Cmp(order.OppositeLockedFunds.V) == -1 {
		return ErrOrder_InsufficientFunds
	}

	// fmt.Printf("Check Order: %s > %s \n", order.LockedFunds.V.String(), minQuoteVolume.String())
	// fmt.Printf("Check Order2: %s > %s \n", order.Amount.V.String(), minMarketVolume.String())

	// for sell orders check that the minimum market volume is met
	if order.Side == MarketSide_Sell && order.LockedFunds.V.Cmp(minMarketVolume) == -1 {
		return fmt.Errorf(ErrOrder_OrderTooSmall.Error(), minMarketVolume.Quantize(market.MarketPrecision).String(), market.MarketCoinSymbol)
	}

	// for buy orders check that the minimum quote market volume is met
	if order.Side == MarketSide_Buy && order.Amount.V.Cmp(minMarketVolume) == -1 {
		return fmt.Errorf(ErrOrder_OrderTooSmall.Error(), minMarketVolume.Quantize(market.MarketPrecision).String(), market.MarketCoinSymbol)
	}
	// all checks successful
	return nil
}

// IsValid - check if the order is valid for a given market
func (order *Order) IsValid(minMarketVolume, minQuoteVolume *decimal.Big, market *Market) error {
	// Check if the user has the necessary funds to execute the order
	if order.IsCustomOrderType() && order.IsReplace {
		if order.IsInitOrderFilled {
			return nil
		}
		if conv.NewDecimalWithPrecision().Add(order.Funds.V, order.PreviousLockedFunds.V).Cmp(order.LockedFunds.V) == -1 {
			return ErrOrder_InsufficientFunds
		}
		if order.IsStrangleOrStraddleOrderType() {
			if conv.NewDecimalWithPrecision().Add(order.OppositeFunds.V, order.PreviousOppositeLockedFunds.V).Cmp(order.OppositeLockedFunds.V) == -1 {
				return ErrOrder_InsufficientFunds
			}
		}
	} else if order.Funds.V.Cmp(order.LockedFunds.V) == -1 || (!order.IsCustomOrderType() && order.LockedFunds.V.Cmp(Zero) != 1) {
		return ErrOrder_InsufficientFunds
	}

	if order.Type == OrderType_Limit || order.Type == OrderType_Market {
		if order.Stop != OrderStop_None {
			if order.StopPrice.V.Cmp(Zero) == 0 {
				return ErrOrder_OrderStopPriceInvalid
			}
			if order.Amount.V.Cmp(Zero) <= 0 {
				return ErrOrder_OrderAmountInvalid
			}
		}

		if order.Amount.V.Cmp(Zero) <= 0 {
			return ErrOrder_OrderAmountInvalid
		}

		switch order.Type {
		case OrderType_Limit:
			if order.Price.V.Cmp(Zero) <= 0 {
				return ErrOrder_OrderPriceInvalid
			}
		case OrderType_Market:
			if order.Funds.V.Cmp(Zero) <= 0 {
				return ErrOrder_OrderPriceInvalid
			}
		}
	}

	if order.IsStrangleOrStraddleOrderType() && order.OppositeFunds.V.Cmp(order.OppositeLockedFunds.V) == -1 {
		return ErrOrder_InsufficientFunds
	}

	// for sell orders check that the minimum market volume is met
	if order.Side == MarketSide_Sell && order.LockedFunds.V.Cmp(minQuoteVolume) == -1 {
		return fmt.Errorf(ErrOrder_OrderTooSmall.Error(), minQuoteVolume.Quantize(market.MarketPrecision).String(), market.QuoteCoinSymbol)
	}

	// for buy orders check that the minimum quote market volume is met
	if order.Side == MarketSide_Buy && order.Amount.V.Cmp(minMarketVolume) == -1 {
		return fmt.Errorf(ErrOrder_OrderTooSmall.Error(), minMarketVolume.Quantize(market.MarketPrecision).String(), market.MarketCoinSymbol)
	}
	// all checks successful
	return nil
}

// IsCustomOrderType - check if order should be processed by custom order processor
func (order *Order) IsCustomOrderType() bool {
	return order.Type == OrderType_OCO || order.Type == OrderType_OTO || order.Type == OrderType_Strangle || order.Type == OrderType_Straddle || order.Type == OrderType_TrailingStop
}

// IsCustomOrderType - check if order should be processed by custom order processor
func (order *Order) IsStrangleOrStraddleOrderType() bool {
	return order.Type == OrderType_Strangle || order.Type == OrderType_Straddle
}

func (order *Order) GetOppositeLockedFunds() *decimal.Big {
	if order.IsStrangleOrStraddleOrderType() {
		return conv.NewDecimalWithPrecision().Mul(order.SLPrice.V, order.Amount.V)
	} else {
		return Zero
	}
}

func clonePostgresDecimal(dec *postgres.Decimal) *postgres.Decimal {
	if dec == nil {
		return nil
	}
	if dec.V == nil {
		return &postgres.Decimal{V: nil}
	}
	return &postgres.Decimal{V: (&decimal.Big{}).Copy(dec.V)}
}

func cloneUint64(id *uint64) *uint64 {
	if id == nil {
		return nil
	}
	aux := *id
	return &aux
}

func cloneStatus(status *OrderStatus) *OrderStatus {
	if status == nil {
		return nil
	}
	stat := *status
	return &stat
}

func cloneTrailingStopPriceType(status *TrailingStopPriceType) *TrailingStopPriceType {
	if status == nil {
		return nil
	}
	stat := *status
	return &stat
}

func cloneOrderType(status *OrderType) *OrderType {
	if status == nil {
		return nil
	}
	stat := *status
	return &stat
}

func (order *Order) Clone() *Order {
	tpStatus := cloneStatus(order.TPStatus)
	slStatus := cloneStatus(order.SLStatus)
	trailingStopPriceType := cloneTrailingStopPriceType(order.TrailingStopPriceType)
	otoType := cloneOrderType(order.OtoType)
	return &Order{
		ID:                          order.ID,
		Status:                      order.Status,
		Side:                        order.Side,
		Type:                        order.Type,
		MarketID:                    order.MarketID,
		OwnerID:                     order.OwnerID,
		Amount:                      clonePostgresDecimal(order.Amount),
		Price:                       clonePostgresDecimal(order.Price),
		StopPrice:                   clonePostgresDecimal(order.StopPrice),
		Stop:                        order.Stop,
		Funds:                       clonePostgresDecimal(order.Funds), // @todo add user funds from the database
		LockedFunds:                 clonePostgresDecimal(order.LockedFunds),
		UsedFunds:                   clonePostgresDecimal(order.UsedFunds),
		FilledAmount:                clonePostgresDecimal(order.FilledAmount),
		FilledOppositeAmount:        clonePostgresDecimal(order.FilledOppositeAmount),
		FeeAmount:                   clonePostgresDecimal(order.FeeAmount),
		ParentOrderId:               cloneUint64(order.ParentOrderId),
		InitOrderId:                 cloneUint64(order.InitOrderId),
		TPOrderId:                   cloneUint64(order.TPOrderId),
		TPPrice:                     clonePostgresDecimal(order.TPPrice),
		TPRelPrice:                  clonePostgresDecimal(order.TPRelPrice),
		TPAmount:                    clonePostgresDecimal(order.TPAmount),
		TPFilledAmount:              clonePostgresDecimal(order.TPFilledAmount),
		TPStatus:                    tpStatus,
		SLStatus:                    slStatus,
		SLAmount:                    clonePostgresDecimal(order.SLAmount),
		SLFilledAmount:              clonePostgresDecimal(order.SLFilledAmount),
		SLPrice:                     clonePostgresDecimal(order.SLPrice),
		SLOrderId:                   cloneUint64(order.SLOrderId),
		SLRelPrice:                  clonePostgresDecimal(order.SLRelPrice),
		ClientOrderID:               order.ClientOrderID,
		TrailingStopActivationPrice: clonePostgresDecimal(order.TrailingStopActivationPrice),
		TrailingStopPrice:           clonePostgresDecimal(order.TrailingStopPrice),
		TrailingStopPriceType:       trailingStopPriceType,
		IsReplace:                   order.IsReplace,
		IsInitOrderFilled:           order.IsInitOrderFilled,
		SubAccount:                  order.SubAccount,
		RefID:                       order.RefID,
		UI:                          order.UI,
		OtoType:                     otoType,
		CreatedAt:                   order.CreatedAt,
		UpdatedAt:                   order.UpdatedAt,
		OppositeFunds:               clonePostgresDecimal(order.OppositeFunds),
		OppositeLockedFunds:         clonePostgresDecimal(order.OppositeLockedFunds),
		PreviousOppositeLockedFunds: clonePostgresDecimal(order.PreviousOppositeLockedFunds),
		PreviousLockedFunds:         clonePostgresDecimal(order.PreviousLockedFunds),
	}
}

// MarshalJSON convert the order into a json string
func (order *Order) MarshalJSON() ([]byte, error) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(map[string]interface{}{
		"id":                     order.ID,
		"type":                   order.Type,
		"status":                 order.Status,
		"side":                   order.Side,
		"amount":                 utils.FmtP(order.Amount),
		"price":                  utils.FmtP(order.Price),
		"stop":                   order.Stop,
		"stop_price":             utils.FmtP(order.StopPrice),
		"funds":                  utils.FmtP(order.Funds),
		"market_id":              order.MarketID,
		"owner_id":               order.OwnerID,
		"locked_funds":           utils.FmtP(order.LockedFunds),
		"used_funds":             utils.FmtP(order.UsedFunds),
		"filled_amount":          utils.FmtP(order.FilledAmount),
		"filled_opposite_amount": utils.FmtP(order.FilledOppositeAmount),
		"fee_amount":             utils.FmtP(order.FeeAmount),
		"created_at":             order.CreatedAt,
		"updated_at":             order.UpdatedAt,
		"init_order_id":          order.InitOrderId,
		"tp_price":               utils.FmtDecimal(order.TPPrice),
		"tp_rel_price":           utils.FmtDecimal(order.TPRelPrice),
		"tp_amount":              utils.FmtDecimal(order.TPAmount),
		"tp_filled_amount":       utils.FmtDecimal(order.TPFilledAmount),
		"tp_status":              order.TPStatus,
		"tp_order_id":            order.TPOrderId,
		"sl_price":               utils.FmtDecimal(order.SLPrice),
		"sl_rel_price":           utils.FmtDecimal(order.SLRelPrice),
		"sl_amount":              utils.FmtDecimal(order.SLAmount),
		"sl_filled_amount":       utils.FmtDecimal(order.SLFilledAmount),
		"sl_status":              order.SLStatus,
		"sl_order_id":            order.SLOrderId,
		"oto_type":               order.OtoType,
		"account":                order.SubAccount,
		"ui":                     order.UI,
		"client_order_id":        order.ClientOrderID,
		"ts_activation_price":    utils.FmtDecimal(order.TrailingStopActivationPrice),
		"ts_price":               utils.FmtDecimal(order.TrailingStopPrice),
		"ts_price_type":          order.TrailingStopPriceType,
	})
}

// MarshalJSON convert the order into a json string
func (order *OrderWithUser) MarshalJSON() ([]byte, error) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(map[string]interface{}{
		"id":                     order.ID,
		"email":                  order.UserEmail,
		"type":                   order.Type,
		"status":                 order.Status,
		"side":                   order.Side,
		"amount":                 utils.Fmt(order.Amount.V),
		"price":                  utils.Fmt(order.Price.V),
		"stop":                   order.Stop,
		"stop_price":             utils.Fmt(order.StopPrice.V),
		"funds":                  utils.Fmt(order.Funds.V),
		"market_id":              order.MarketID,
		"owner_id":               order.OwnerID,
		"locked_funds":           utils.Fmt(order.LockedFunds.V),
		"used_funds":             utils.Fmt(order.UsedFunds.V),
		"filled_amount":          utils.Fmt(order.FilledAmount.V),
		"filled_opposite_amount": utils.Fmt(order.FilledOppositeAmount.V),
		"fee_amount":             utils.Fmt(order.FeeAmount.V),
		"created_at":             order.CreatedAt,
		"updated_at":             order.UpdatedAt,
		"first_name":             order.FirstName,
		"last_name":              order.LastName,
		"init_order_id":          order.InitOrderId,
		"tp_price":               utils.Fmt(order.TPPrice.V),
		"tp_rel_price":           utils.Fmt(order.TPRelPrice.V),
		"tp_amount":              utils.FmtDecimal(order.TPAmount),
		"tp_filled_amount":       utils.FmtDecimal(order.TPFilledAmount),
		"tp_status":              order.TPStatus,
		"tp_order_id":            order.TPOrderId,
		"sl_price":               utils.Fmt(order.SLPrice.V),
		"sl_rel_price":           utils.Fmt(order.SLRelPrice.V),
		"sl_amount":              utils.FmtDecimal(order.SLAmount),
		"sl_filled_amount":       utils.FmtDecimal(order.SLFilledAmount),
		"sl_status":              order.SLStatus,
		"sl_order_id":            order.SLOrderId,
		"oto_type":               order.OtoType,
		"account":                order.SubAccount,
		"ui":                     order.UI,
		"client_order_id":        order.ClientOrderID,
		"ts_activation_price":    utils.FmtDecimal(order.TrailingStopActivationPrice),
		"ts_price":               utils.FmtDecimal(order.TrailingStopPrice),
		"ts_price_type":          order.TrailingStopPriceType,
	})
}

// GenerateUpdatesFromTrade - generate order status and amounts from trade information
func (order *Order) GenerateUpdatesFromTrade(trade *Trade, updatedAt time.Time) *Order {
	// add to the filled amount of the order
	var filledAmount, feeAmount, usedFunds, lockedFunds *decimal.Big
	filledAmount = (&decimal.Big{}).Add(order.FilledAmount.V, trade.Volume.V)
	lockedFunds = (&decimal.Big{}).Copy(order.LockedFunds.V)

	// add to fee_amount for the ask order
	if order.ID == trade.AskID {
		feeAmount = (&decimal.Big{}).Add(order.FeeAmount.V, trade.AskFeeAmount.V)
		usedFunds = (&decimal.Big{}).Add(order.UsedFunds.V, trade.Volume.V)
	} else {
		feeAmount = (&decimal.Big{}).Add(order.FeeAmount.V, trade.BidFeeAmount.V)
		usedFunds = (&decimal.Big{}).Add(order.UsedFunds.V, trade.QuoteVolume.V)
	}

	updates := Order{
		ID:           order.ID,
		MarketID:     order.MarketID,
		FilledAmount: &postgres.Decimal{V: filledAmount},
		FeeAmount:    &postgres.Decimal{V: feeAmount},
		UsedFunds:    &postgres.Decimal{V: usedFunds},
		LockedFunds:  &postgres.Decimal{V: lockedFunds},
		RefID:        order.RefID,
		SubAccount:   order.SubAccount,
		UpdatedAt:    updatedAt,
	}

	if order.Amount.V.Cmp(filledAmount) != 1 || order.LockedFunds.V.Cmp(usedFunds) != 1 {
		order.Status = OrderStatus_Filled
		updates.Status = OrderStatus_Filled
	} else {
		order.Status = OrderStatus_PartiallyFilled
		updates.Status = OrderStatus_PartiallyFilled
	}

	return &updates
}

// GetValidTransitionsToStatus godoc
// Get a list of valid statuses that can be used to transition to the given status
func GetValidTransitionsToStatus(status OrderStatus) []OrderStatus {
	switch status {
	case OrderStatus_Untouched:
		return OrderStatusesToUntouched
	case OrderStatus_PartiallyFilled:
		return OrderStatusesToPartiallyFilled
	case OrderStatus_Filled:
		return OrderStatusesToFilled
	case OrderStatus_Cancelled:
		return OrderStatusesToCancelled
	}
	return OrderStatusesToPending
}

// Orders is a slice of *Order, which can be sorted
type Orders []*Order

func (os Orders) Len() int           { return len(os) }
func (os Orders) Swap(i, j int)      { os[i], os[j] = os[j], os[i] }
func (os Orders) Less(i, j int) bool { return os[i].Price.V.Cmp(os[j].Price.V) == -1 }

// OrdersDesc is a slice of *Order, which can be sorted in descending order
type OrdersDesc []*Order

func (os OrdersDesc) Len() int           { return len(os) }
func (os OrdersDesc) Swap(i, j int)      { os[i], os[j] = os[j], os[i] }
func (os OrdersDesc) Less(i, j int) bool { return os[i].Price.V.Cmp(os[j].Price.V) == 1 }
