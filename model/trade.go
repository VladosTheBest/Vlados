package model

import (
	"encoding/json"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

type TradeStatus string

const (
	TradeStatus_Created   TradeStatus = "created"
	TradeStatus_FeesAdded TradeStatus = "fees_added"
)

// Trade holds information about a completed trade event
type Trade struct {
	ID                 uint64            `sql:"type:bigint" gorm:"PRIMARY_KEY" json:"id" wire:"id"`
	MarketID           string            `sql:"type:varchar(10) REFERENCES markets(id)" json:"market_id" wire:"market_id"`
	Volume             *postgres.Decimal `sql:"type:decimal(36,18)" json:"volume" wire:"volume"`
	QuoteVolume        *postgres.Decimal `sql:"type:decimal(36,18)" json:"quote_volume" wire:"quote_volume"`
	Price              *postgres.Decimal `sql:"type:decimal(36,18)" json:"price" wire:"price"`
	AskFeeAmount       *postgres.Decimal `sql:"type:decimal(36,18)" json:"ask_fee_amount" wire:"ask_fee_amount"`
	BidFeeAmount       *postgres.Decimal `sql:"type:decimal(36,18)" json:"bid_fee_amount" wire:"bid_fee_amount"`
	MarketAskFeeAmount *postgres.Decimal `sql:"type:decimal(36,18)" json:"market_ask_fee_amount" wire:"market_ask_fee_amount"`
	QuoteBidFeeAmount  *postgres.Decimal `sql:"type:decimal(36,18)" json:"quote_bid_fee_amount" wire:"quote_bid_fee_amount"`
	AskID              uint64            `sql:"type:bigint references orders(id)" json:"ask_id" wire:"ask_id"`
	AskOwnerID         uint64            `sql:"type:bigint references users(id)" json:"ask_owner_id" wire:"ask_owner_id"`
	BidID              uint64            `sql:"type:bigint references orders(id)" json:"bid_id" wire:"bid_id"`
	EventSeqID         int64             `gorm:"column:event_seqid" sql:"type:bigint" json:"-" wire:"event_seqid"`
	SeqID              int64             `gorm:"column:seqid" sql:"type:bigint" json:"seqid" wire:"seqid"`
	BidOwnerID         uint64            `sql:"type:bigint references users(id)" json:"bid_owner_id" wire:"bid_owner_id"`
	TakerSide          MarketSide        `sql:"not null;type:market_side_t;default:'buy'" json:"taker_side" wire:"taker_side"`
	Status             TradeStatus       `sql:"not null;type:trade_status_t;default:'created'" json:"status" wire:"status"`
	Timestamp          int64             `json:"timestamp" wire:"timestamp"`
	CreatedAt          time.Time         `json:"-" wire:"created_at"`
	UpdatedAt          time.Time         `json:"-" wire:"updated_at"`
	RefID              string            `gorm:"column:ref_id" json:"ref_id" wire:"ref_id"`
	AskSubAccount      uint64            `sql:"ask_sub_account" json:"ask_sub_account" wire:"ask_sub_account"`
	BidSubAccount      uint64            `sql:"bid_sub_account" json:"bid_sub_account" wire:"bid_sub_account"`
}

func (t *Trade) IsSelfTrading() bool {
	return t.AskOwnerID == t.BidOwnerID && t.AskSubAccount == t.BidSubAccount
}

type PublicTrade Trade

type UserTrades struct {
	UserID        uint64
	Trades        []Trade
	OrdersRootIds map[uint64]*Order
}

type UserTrade struct {
	UserID uint64
	Trade  Trade
}

type TradesForOrders struct {
	OrderIDs []uint64
	Trades   []Trade
}

// TradeList structure
type TradeList struct {
	Trades *UserTrades `json:"trades"`
	Meta   PagingMeta  `json:"meta"`
}

// TradeOrderList structure
type TradeOrderList struct {
	Trades *UserTrades `json:"trades"`
	Orders []Order     `json:"orders"`
	Meta   PagingMeta  `json:"meta"`
}

// NewTrade creates a new trade to save in the database
func NewTrade(eventSeqID uint64, market string, seqID uint64, side MarketSide, price, volume, quoteVolume *decimal.Big, askID, bidID, askOwnerID, bidOwnerID uint64, timestamp int64) *Trade {
	return &Trade{
		MarketID:    market,
		Volume:      &postgres.Decimal{V: volume},
		QuoteVolume: &postgres.Decimal{V: quoteVolume},
		Price:       &postgres.Decimal{V: price},
		AskID:       askID,
		BidID:       bidID,
		AskOwnerID:  askOwnerID,
		BidOwnerID:  bidOwnerID,
		TakerSide:   side,
		Timestamp:   timestamp,
		EventSeqID:  int64(eventSeqID),
		SeqID:       int64(seqID),
		Status:      TradeStatus_Created,
		CreatedAt:   time.Unix(0, timestamp),
		UpdatedAt:   time.Unix(0, timestamp),
	}
}

// Model Methods

func (trade *Trade) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":                    trade.ID,
		"market_id":             trade.MarketID,
		"volume":                utils.Fmt(trade.Volume.V),
		"quote_volume":          utils.Fmt(trade.QuoteVolume.V),
		"price":                 utils.Fmt(trade.Price.V),
		"ask_fee_amount":        utils.Fmt(trade.AskFeeAmount.V),
		"bid_fee_amount":        utils.Fmt(trade.BidFeeAmount.V),
		"market_ask_fee_amount": utils.Fmt(trade.MarketAskFeeAmount.V),
		"quote_bid_fee_amount":  utils.Fmt(trade.QuoteBidFeeAmount.V),
		//"ask_id":                trade.AskID,
		//"bid_id":                trade.BidID,
		//"ask_owner_id":          trade.AskOwnerID,
		//"bid_owner_id":          trade.BidOwnerID,
		"taker_side": trade.TakerSide,
		//"event_seqid":           trade.EventSeqID,
		"seqid":      trade.SeqID,
		"status":     trade.Status,
		"timestamp":  trade.Timestamp / 1000000000,
		"self_trade": trade.AskOwnerID == trade.BidOwnerID,
	})
}

func (trade PublicTrade) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":           trade.ID,
		"market_id":    trade.MarketID,
		"volume":       utils.Fmt(trade.Volume.V),
		"quote_volume": utils.Fmt(trade.QuoteVolume.V),
		"price":        utils.Fmt(trade.Price.V),
		"taker_side":   trade.TakerSide,
		"seqid":        trade.SeqID,
		"timestamp":    trade.Timestamp / 1000000000,
	})
}

func (trades UserTrades) MarshalJSON() ([]byte, error) {
	var orderId uint64
	list := make([]map[string]interface{}, 0, len(trades.Trades))
	for _, trade := range trades.Trades {
		if trades.UserID == trade.AskOwnerID {
			order := trades.OrdersRootIds[trade.AskID]
			if order != nil {
				orderId = *order.RootOrderId

				list = append(list, map[string]interface{}{
					"id":        trade.ID,
					"market_id": trade.MarketID,
					"order_id":  orderId,
					//"counter_order_id": trade.BidID,
					"volume":       utils.Fmt(trade.Volume.V),
					"quote_volume": utils.Fmt(trade.QuoteVolume.V),
					"price":        utils.Fmt(trade.Price.V),
					"fee_amount":   utils.Fmt(trade.AskFeeAmount.V),
					"side":         "sell",
					"taker_side":   trade.TakerSide,
					"seqid":        trade.SeqID,
					"timestamp":    trade.Timestamp / 1000000000,
					"self_trade":   trade.AskOwnerID == trade.BidOwnerID,
				})
			} else {
				list = append(list, map[string]interface{}{
					"id":        trade.ID,
					"market_id": trade.MarketID,
					"order_id":  trade.AskID,
					//"counter_order_id": trade.BidID,
					"volume":       utils.Fmt(trade.Volume.V),
					"quote_volume": utils.Fmt(trade.QuoteVolume.V),
					"price":        utils.Fmt(trade.Price.V),
					"fee_amount":   utils.Fmt(trade.AskFeeAmount.V),
					"side":         "sell",
					"taker_side":   trade.TakerSide,
					"seqid":        trade.SeqID,
					"timestamp":    trade.Timestamp / 1000000000,
					"self_trade":   trade.AskOwnerID == trade.BidOwnerID,
				})
			}
		} else {
			order := trades.OrdersRootIds[trade.BidID]
			if order != nil {
				orderId = *order.RootOrderId

				list = append(list, map[string]interface{}{
					"id":        trade.ID,
					"market_id": trade.MarketID,
					"order_id":  orderId,
					//"counter_order_id": trade.BidID,
					"volume":       utils.Fmt(trade.Volume.V),
					"quote_volume": utils.Fmt(trade.QuoteVolume.V),
					"price":        utils.Fmt(trade.Price.V),
					"fee_amount":   utils.Fmt(trade.BidFeeAmount.V),
					"side":         "buy",
					"taker_side":   trade.TakerSide,
					"seqid":        trade.SeqID,
					"timestamp":    trade.Timestamp / 1000000000,
					"self_trade":   trade.AskOwnerID == trade.BidOwnerID,
				})
			} else {
				list = append(list, map[string]interface{}{
					"id":        trade.ID,
					"market_id": trade.MarketID,
					"order_id":  trade.BidID,
					//"counter_order_id": trade.BidID,
					"volume":       utils.Fmt(trade.Volume.V),
					"quote_volume": utils.Fmt(trade.QuoteVolume.V),
					"price":        utils.Fmt(trade.Price.V),
					"fee_amount":   utils.Fmt(trade.BidFeeAmount.V),
					"side":         "buy",
					"taker_side":   trade.TakerSide,
					"seqid":        trade.SeqID,
					"timestamp":    trade.Timestamp / 1000000000,
					"self_trade":   trade.AskOwnerID == trade.BidOwnerID,
				})
			}
		}
	}
	return json.Marshal(list)
}

func (t TradesForOrders) MarshalJSON() ([]byte, error) {
	// create a map of orders that we should generate the trades for
	orderMap := map[uint64]struct{}{}
	for _, orderID := range t.OrderIDs {
		orderMap[orderID] = struct{}{}
	}

	list := make([]map[string]interface{}, 0, len(t.Trades))
	for _, trade := range t.Trades {
		if _, ok := orderMap[trade.AskID]; ok {
			list = append(list, map[string]interface{}{
				"id":               trade.ID,
				"market_id":        trade.MarketID,
				"order_id":         trade.AskID,
				"counter_order_id": trade.BidID,
				"volume":           utils.Fmt(trade.Volume.V),
				"quote_volume":     utils.Fmt(trade.QuoteVolume.V),
				"price":            utils.Fmt(trade.Price.V),
				"fee_amount":       utils.Fmt(trade.AskFeeAmount.V),
				"side":             "sell",
				"taker_side":       trade.TakerSide,
				"seqid":            trade.SeqID,
				"timestamp":        trade.Timestamp / 1000000000,
				"self_trade":       trade.AskOwnerID == trade.BidOwnerID,
			})
		}
		if _, ok := orderMap[trade.BidID]; ok {
			list = append(list, map[string]interface{}{
				"id":               trade.ID,
				"market_id":        trade.MarketID,
				"order_id":         trade.BidID,
				"counter_order_id": trade.AskID,
				"volume":           utils.Fmt(trade.Volume.V),
				"quote_volume":     utils.Fmt(trade.QuoteVolume.V),
				"price":            utils.Fmt(trade.Price.V),
				"fee_amount":       utils.Fmt(trade.BidFeeAmount.V),
				"side":             "buy",
				"taker_side":       trade.TakerSide,
				"seqid":            trade.SeqID,
				"timestamp":        trade.Timestamp / 1000000000,
				"self_trade":       trade.AskOwnerID == trade.BidOwnerID,
			})
		}
	}
	return json.Marshal(list)
}

// MarshalJSON godoc
func (userTrade UserTrade) MarshalJSON() ([]byte, error) {
	trade := userTrade.Trade
	userID := userTrade.UserID
	feeAmount := ""
	var side string
	orderID := uint64(0)
	counterOrderID := uint64(0)
	if userID == trade.AskOwnerID {
		feeAmount = utils.Fmt(trade.AskFeeAmount.V)
		side = "sell"
		orderID = trade.AskID
		counterOrderID = trade.BidID
	} else {
		feeAmount = utils.Fmt(trade.BidFeeAmount.V)
		side = "buy"
		orderID = trade.BidID
		counterOrderID = trade.AskID
	}
	return json.Marshal(map[string]interface{}{
		"id":               trade.ID,
		"market_id":        trade.MarketID,
		"order_id":         orderID,
		"counter_order_id": counterOrderID,
		"volume":           utils.Fmt(trade.Volume.V),
		"quote_volume":     utils.Fmt(trade.QuoteVolume.V),
		"price":            utils.Fmt(trade.Price.V),
		"fee_amount":       feeAmount,
		"side":             side,
		"seqid":            trade.SeqID,
		"timestamp":        trade.Timestamp / 1000000000,
		"self_trade":       trade.AskOwnerID == trade.BidOwnerID,
	})
}

// SetFees - calculate bid fee amount based on general platform fees
func (trade *Trade) SetFees(takerFee, makerFee *decimal.Big) *Trade {
	askFeeAmount := conv.NewDecimalWithPrecision()
	bidFeeAmount := conv.NewDecimalWithPrecision()
	marketAskFeeAmount := conv.NewDecimalWithPrecision()
	quoteBidFeeAmount := conv.NewDecimalWithPrecision()

	if trade.TakerSide == MarketSide_Sell {
		askFeeAmount.Mul(trade.QuoteVolume.V, takerFee)
		bidFeeAmount.Mul(trade.Volume.V, makerFee)
	} else {
		askFeeAmount.Mul(trade.QuoteVolume.V, makerFee)
		bidFeeAmount.Mul(trade.Volume.V, takerFee)
	}

	trade.AskFeeAmount = &postgres.Decimal{V: askFeeAmount}
	trade.BidFeeAmount = &postgres.Decimal{V: bidFeeAmount}

	marketAskFeeAmount.Mul(askFeeAmount, trade.Volume.V)
	quoteBidFeeAmount.Mul(bidFeeAmount, trade.Price.V)
	trade.MarketAskFeeAmount = &postgres.Decimal{V: marketAskFeeAmount}
	trade.QuoteBidFeeAmount = &postgres.Decimal{V: quoteBidFeeAmount}
	return trade
}

// GORM Event Handlers
