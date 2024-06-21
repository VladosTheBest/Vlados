package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	marketCache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/markets"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/crons"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/data"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// ErrInvalidFrozenOrderStatus godoc
var ErrInvalidFrozenOrderStatus = errors.New("Invalid status for frozen order")

// ErrInvalidFrozenOrderMarket godoc
var ErrInvalidFrozenOrderMarket = errors.New("Invalid market for frozen order")

// RevertOrder - revert a stuck pending order
func (service *Service) RevertOrder(market *model.Market, order *model.Order) error {
	// pre execution checks
	if order.Status != model.OrderStatus_Pending {
		return ErrInvalidFrozenOrderStatus
	}
	if order.MarketID != market.ID {
		return ErrInvalidFrozenOrderMarket
	}

	// format order cancel event for publishing
	price := fmt.Sprintf("%f", order.Price.V)
	priceInUnits := conv.ToUnits(price, uint8(market.QuotePrecision))
	stopPrice := fmt.Sprintf("%f", order.StopPrice.V)
	stopPriceInUnits := conv.ToUnits(stopPrice, uint8(market.QuotePrecision))

	// publish cancel order event to kafka
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

	// bytes, err := orderEvent.ToBinary()
	// if err != nil {
	// 	return err
	// }

	// start transaction
	tx := service.repo.Conn.Begin()

	// add order in the revert queue
	revertOrder := model.NewSystemRevertOrderQueue(order.ID, 30) // add revert order with a 30 minutes delay

	// create revert order in queue
	db := tx.Create(revertOrder)
	if db.Error != nil {
		tx.Rollback()
		return db.Error
	}

	// @todo switch to in memory matching engine
	// err = service.dm.Publish("orders", map[string]string{"market": market.ID}, kafkaGo.Message{Value: bytes})
	var err error = nil
	service.Markets[market.ID].Process(orderEvent)

	if err != nil {
		tx.Rollback()
		return err
	}

	// commit transaction
	db = tx.Commit()
	if db.Error != nil {
		return db.Error
	}

	return nil
}

// StartCronSystemRevertFrozenOrder godoc
func (service *Service) StartCronSystemRevertFrozenOrder() {
	log.Info().Str("cron", "system_revert_frozen_order").Str("action", "start").Msg("Cron revert frozen order - started")
	for range crons.SystemRevertOrderChan {
		err := service.ExecuteRevertOrder()
		if err != nil {
			log.Error().Err(err).Msg("Unable to execute order reversal cron")
		}
	}
	log.Info().Str("cron", "system_revert_frozen_order").Str("action", "stop").Msg("2 => Cron revert frozen order - stopped")
}

// ExecuteRevertOrder godoc
func (service *Service) ExecuteRevertOrder() error {
	revertOrders, err := service.repo.GetScheduledRevertedOrdersFromQueue(100)
	if err != nil {
		return err
	}

	for i := range revertOrders {
		revertOrder := revertOrders[i]
		order := revertOrder.Order
		market, err := marketCache.Get(order.MarketID)
		if err != nil {
			continue
		}
		err = service.repo.Delete(&revertOrder, uint(revertOrder.ID))
		if err != nil {
			log.Error().Err(err).Str("market_id", market.ID).Uint64("order_id", order.ID).Msg("Unable to revert frozen order (delete from queue)")
			continue
		}
		err = service.ops.CancelOrder(&order, market.MarketCoinSymbol, market.QuoteCoinSymbol, model.OrderStatus_Cancelled, time.Now())
		if err != nil {
			log.Error().Err(err).Str("market_id", market.ID).Uint64("order_id", order.ID).Msg("Unable to revert frozen order (revert funds)")
			continue
		}
	}

	return nil
}
