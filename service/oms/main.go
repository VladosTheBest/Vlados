package oms

import (
	"context"
	"errors"
	"fmt"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/gostop"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/data"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/net/manager"

	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/rs/zerolog/log"
	orderCache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/order"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/monitor"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

const chunkSize = 1000

var ErrTradeOrderNotFound = errors.New("Unable to get trade order")
var ErrOrderNotFound = errors.New("order not found")

type OMS struct {
	rootCtx              context.Context
	ordersActiveByID     map[uint64]model.Order                        // orderID:order
	ordersActiveByClient map[uint64]map[string]map[uint64]*model.Order // userID:marketID:orderID:order
	ordersActive         map[string]map[uint64]*model.Order            // market:orderID:order
	ordersLock           *sync.RWMutex
	Seq                  *seq
	repo                 *queries.Repo
	LastPrice            *lastPrice
	marketDepthCache     *marketDepthCache
	dm                   manager.DataManagerInterface
	encoder              data.Encoder
}

func (o *OMS) GetOrder(marketID string, id uint64) (*model.Order, error) {
	o.ordersLock.RLock()
	defer o.ordersLock.RUnlock()
	ord, ok := o.ordersActive[marketID][id]
	if !ok {
		return nil, ErrOrderNotFound
	}
	order := *ord
	return &order, nil
}

func (o *OMS) GetTradeOrders(marketID string, askID, bidID uint64) (*model.Order, *model.Order, error, error) {
	o.ordersLock.RLock()
	defer o.ordersLock.RUnlock()
	askOrd, askOK := o.ordersActive[marketID][askID]
	bidOrd, bidOK := o.ordersActive[marketID][bidID]
	var askOrder, bidOrder *model.Order
	if !askOK && !bidOK {
		return askOrder, bidOrder, ErrTradeOrderNotFound, ErrTradeOrderNotFound
	}
	if askOK {
		askOrder = askOrd.Clone()
		//askOrder := &(*askOrd)
	}
	if bidOK {
		bidOrder = bidOrd.Clone()
		//bidOrder := &(*bidOrd)
	}

	if !askOK {
		return askOrder, bidOrder, ErrTradeOrderNotFound, nil
	} else if !bidOK {
		return askOrder, bidOrder, nil, ErrTradeOrderNotFound
	}

	return askOrder, bidOrder, nil, nil
}

func (o *OMS) GetOrderByID(id uint64) (*model.Order, error) {
	o.ordersLock.RLock()
	defer o.ordersLock.RUnlock()
	ord, ok := o.ordersActiveByID[id]
	if !ok {
		return nil, ErrOrderNotFound
	}
	olink := &ord
	order := *olink
	return &order, nil
}

func (o *OMS) GetOrdersByUser(userID uint64) map[string]map[uint64]model.Order {
	o.ordersLock.RLock()
	defer o.ordersLock.RUnlock()

	ordersMap, ok := o.ordersActiveByClient[userID]

	ordersMapOut := map[string]map[uint64]model.Order{}
	if ok {
		for marketID, orders := range ordersMap {
			ordersMapOut[marketID] = map[uint64]model.Order{}
			for id, order := range orders {
				ordersMapOut[marketID][id] = *order.Clone()
			}

		}
	}

	return ordersMapOut
}

func (o *OMS) GetOrdersByMarketID(marketID string) ([]*model.Order, error) {
	o.ordersLock.RLock()
	defer o.ordersLock.RUnlock()
	orderMap, ok := o.ordersActive[marketID]
	if !ok {
		return []*model.Order{}, ErrOrderNotFound
	}
	// get sorted keys from map
	var keyList []uint64
	for key := range orderMap {
		keyList = append(keyList, key)
	}
	sort.Slice(keyList, func(i, j int) bool {
		return keyList[i] < keyList[j]
	})
	orderList := make([]*model.Order, 0, len(keyList))
	for k := range keyList {
		orderList = append(orderList, orderMap[keyList[k]])
	}

	return orderList, nil
}

func (o *OMS) addOrder(order model.Order, skipLock bool) *model.Order {
	if !skipLock {
		o.ordersLock.Lock()
		defer o.ordersLock.Unlock()
	}

	o.ordersActiveByID[order.ID] = order

	if o.ordersActive[order.MarketID] == nil {
		o.ordersActive[order.MarketID] = make(map[uint64]*model.Order)
	}

	if o.ordersActiveByClient[order.OwnerID] == nil {
		o.ordersActiveByClient[order.OwnerID] = make(map[string]map[uint64]*model.Order)
	}

	if o.ordersActiveByClient[order.OwnerID][order.MarketID] == nil {
		o.ordersActiveByClient[order.OwnerID][order.MarketID] = make(map[uint64]*model.Order)
	}

	ord := o.ordersActiveByID[order.ID]
	o.ordersActive[order.MarketID][order.ID] = &ord
	o.ordersActiveByClient[order.OwnerID][order.MarketID][order.ID] = &ord

	return &ord
}

func (o *OMS) SaveOrder(order *model.Order) error {
	ord := o.addOrder(*order, false)
	event := data.NewSaveDataEvent(o.encoder, "orders", ord)
	return o.sendDataEvent(event)
}

func (o *OMS) UpdateCustomOrder(order *model.Order) error {
	marketID := order.MarketID
	id := order.ID
	status := order.Status

	o.ordersLock.Lock()
	ord, ok := o.ordersActive[marketID][id]
	if !ok {
		o.ordersLock.Unlock()
		return ErrOrderNotFound
	}

	if err := ord.Status.IsValidChange(status); err != nil {
		if err == model.ErrOrder_OrderStatusInvalid {
			log.Debug().
				Uint64("orderID", ord.ID).
				Str("orderType", ord.Type.String()).
				Str("prevStatus", ord.Status.String()).
				Str("nextStatus", status.String()).
				Str("method", "IsValidChange").
				Msg(err.Error())
			o.ordersLock.Unlock()
			return nil
		}

		if err == model.ErrOrder_OrderStatusSame { // && status == model.OrderStatus_PartiallyFilled
			// ignore same status order if the order has the status partially filled
			err = nil
		} else {
			o.ordersLock.Unlock()
			return err
		}
	}

	o.ordersActiveByID[order.ID] = *order
	o.ordersActive[order.MarketID][order.ID] = order
	o.ordersActiveByClient[order.OwnerID][order.MarketID][order.ID] = order

	if status == model.OrderStatus_Cancelled || status == model.OrderStatus_Filled {
		o.removeCompletedOrder(order.MarketID, order.ID, order.OwnerID)
	}
	o.ordersLock.Unlock()

	return nil
}

func (o *OMS) UpdateOrder(newOrder *model.Order) error {
	marketID := newOrder.MarketID
	id := newOrder.ID
	status := newOrder.Status
	updatedAt := newOrder.UpdatedAt
	filledAmount := newOrder.FilledAmount
	feeAmount := newOrder.FeeAmount
	usedFunds := newOrder.UsedFunds

	o.ordersLock.Lock()
	order, ok := o.ordersActive[marketID][id]
	if !ok {
		o.ordersLock.Unlock()
		return ErrOrderNotFound
	}

	if err := order.Status.IsValidChange(status); err != nil {
		if err == model.ErrOrder_OrderStatusInvalid {
			log.Debug().
				Uint64("orderID", order.ID).
				Str("orderType", order.Type.String()).
				Str("prevStatus", order.Status.String()).
				Str("nextStatus", status.String()).
				Str("method", "IsValidChange").
				Msg(err.Error())
			o.ordersLock.Unlock()
			return nil
		}

		if err == model.ErrOrder_OrderStatusSame { // && status == model.OrderStatus_PartiallyFilled
			// ignore same status order if the order has the status partially filled
			err = nil
		} else {
			o.ordersLock.Unlock()
			return err
		}
	}

	order.Status = status
	order.UpdatedAt = updatedAt
	order.FilledAmount = filledAmount
	order.FeeAmount = feeAmount
	order.UsedFunds = usedFunds

	o.ordersActiveByID[order.ID] = *order
	o.ordersActive[order.MarketID][order.ID] = order
	o.ordersActiveByClient[order.OwnerID][order.MarketID][order.ID] = order

	if status == model.OrderStatus_Cancelled || status == model.OrderStatus_Filled {
		o.removeCompletedOrder(order.MarketID, order.ID, order.OwnerID)
	}
	o.ordersLock.Unlock()
	clone := order.Clone()

	err := o.updateOrder(clone)

	cachedOrder := clone.Clone()
	if orderCache.IsSubscribed(cachedOrder.OwnerID, cachedOrder.SubAccount) {
		orderCache.Set(cachedOrder)
	}

	return err
}

func (o *OMS) updateOrder(order *model.Order) error {
	event := data.NewUpdateOrderDataEvent(o.encoder, order)
	return o.sendDataEvent(event)
}

func (o *OMS) removeCompletedOrder(marketID string, orderID, ownerID uint64) {
	delete(o.ordersActive[marketID], orderID)
	delete(o.ordersActiveByClient[ownerID][marketID], orderID)
	delete(o.ordersActiveByID, orderID)
}

func (o *OMS) UpdateOrderStatus(marketID string, id uint64, status model.OrderStatus, updatedAt time.Time, withSaving bool) (bool, error) {
	o.ordersLock.Lock()
	defer o.ordersLock.Unlock()
	// o.ordersLock.RLock()
	order, ok := o.ordersActive[marketID][id]
	// o.ordersLock.RUnlock()
	if !ok {
		return false, ErrOrderNotFound
	}

	if err := order.Status.IsValidChange(status); err != nil {
		if err == model.ErrOrder_OrderStatusSame {
			return false, nil
		}
		if err == model.ErrOrder_OrderStatusInvalid {
			log.Debug().
				Uint64("orderID", order.ID).
				Str("orderType", order.Type.String()).
				Str("prevStatus", order.Status.String()).
				Str("nextStatus", status.String()).
				Str("method", "IsValidChange").
				Msg(err.Error())
			return false, nil
		}
		return false, err
	}
	order.Status = status
	order.UpdatedAt = updatedAt

	o.ordersActiveByID[order.ID] = *order
	o.ordersActive[order.MarketID][order.ID] = order
	o.ordersActiveByClient[order.OwnerID][order.MarketID][order.ID] = order

	if status == model.OrderStatus_Cancelled || status == model.OrderStatus_Filled {
		o.removeCompletedOrder(order.MarketID, order.ID, order.OwnerID)
	}

	if withSaving {
		err := o.updateOrder(order.Clone())

		cachedOrder := order.Clone()
		if orderCache.IsSubscribed(cachedOrder.OwnerID, cachedOrder.SubAccount) {
			orderCache.Set(cachedOrder)
		}

		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func (o *OMS) SaveTrade(trade *model.Trade) error {
	o.LastPrice.Set(trade)
	event := data.NewSaveDataEvent(o.encoder, "trades", trade)
	return o.sendDataEvent(event)
}

func (o *OMS) SaveRevenues(revenues ...*model.Revenue) (err error) {
	for _, r := range revenues {
		event := data.NewSaveDataEvent(o.encoder, "revenues", r)
		if e := o.sendDataEvent(event); e != nil {
			err = e
		}
	}

	return err
}

func (o *OMS) SaveLiabilities(liabilities ...*model.Liability) (err error) {
	for _, l := range liabilities {
		event := data.NewSaveDataEvent(o.encoder, "liabilities", l)
		if e := o.sendDataEvent(event); e != nil {
			err = e
		}
	}
	return err
}

func (o *OMS) SaveReferralEarnings(items ...*model.ReferralEarning) (err error) {
	for _, re := range items {
		event := data.NewSaveDataEvent(o.encoder, "referral_earnings", re)
		if e := o.sendDataEvent(event); e != nil {
			err = e
		}
	}

	return err
}

func (o *OMS) SaveOperations(operations ...*model.Operation) (err error) {
	for _, op := range operations {
		event := data.NewSaveDataEvent(o.encoder, "operations", op)
		if e := o.sendDataEvent(event); e != nil {
			err = e
		}
	}

	return err
}

// func (o *OMS) newTx() (*gorm.DB, error) {
// 	tx := o.repo.Conn.Begin()
// 	tx.Debug()

// 	//if err := tx.Exec(`set transaction isolation level READ COMMITTED`).Error; err != nil {
// 	//	log.Error().Str("service", "OMS").
// 	//		Str("method", "processorOrders").
// 	//		Err(err).Msg("Unable to set isolation level")
// 	//	tx.Rollback()
// 	//	return nil, err
// 	//}

// 	return tx, nil
// }

// func (o *OMS) newTxWithConflictsSkipping() (*gorm.DB, error) {
// 	tx, err := o.newTx()
// 	if err != nil {
// 		return nil, err
// 	}

// 	return tx.Clauses(clause.OnConflict{DoNothing: true}), nil
// 	//.Set("gorm:insert_option", "ON CONFLICT DO NOTHING")
// }

// Monitor the number of active orders in the OMS map
func (o *OMS) cronActiveMonitor(ctx context.Context, wait *sync.WaitGroup) {
	log.Info().Str("cron", "oms_active_order_monitor").Str("action", "start").Msg("OMS Active order monitor - started")
	ticker := time.NewTicker(time.Second)

	for {
		select {
		case <-ticker.C:
			o.ordersLock.Lock()
			for marketID := range o.ordersActive {
				monitor.ActiveOrders.WithLabelValues(marketID).Set(float64(len(o.ordersActive[marketID])))
			}
			o.ordersLock.Unlock()
		case <-ctx.Done():
			ticker.Stop()
			log.Info().Str("cron", "oms_active_order_monitor").Str("action", "stop").Msg("11 => OMS Active order monitor - stopped")
			wait.Done()
			return
		}
	}
}

// Clean up the OMS memory cache
func (o *OMS) cronCacheCleanup(ctx context.Context, wait *sync.WaitGroup) {
	log.Info().Str("cron", "oms_cache_cleanup").Str("action", "start").Msg("OMS Cache cleanup - started")
	ticker := time.NewTicker(5 * time.Minute)

	for {
		select {
		case <-ticker.C:
			o.cleanupCache()
		case <-ctx.Done():
			ticker.Stop()
			log.Info().Str("cron", "oms_cache_cleanup").Str("action", "stop").Msg("12 => OMS Cache cleanup - stopped")
			wait.Done()
			return
		}
	}
}

// cleanupCache cleans the unwanted memory from orders cache
func (o *OMS) cleanupCache() {
	log.Info().Str("service", "OMS").Str("method", "cleanupCache").Msg("cleaning up orders cache")
	o.ordersLock.Lock()
	defer o.ordersLock.Unlock()

	uniqueMarkets := make(map[string]bool)
	uniqueUsers := make(map[uint64]bool)
	copyOrdersActiveByID := make(map[uint64]model.Order)
	for id, order := range o.ordersActiveByID {
		copyOrdersActiveByID[id] = order
		uniqueMarkets[order.MarketID] = true
		uniqueUsers[order.OwnerID] = true
	}

	o.ordersActive = make(map[string]map[uint64]*model.Order, len(uniqueMarkets))
	o.ordersActiveByClient = make(map[uint64]map[string]map[uint64]*model.Order, len(uniqueUsers)+chunkSize)
	o.ordersActiveByID = make(map[uint64]model.Order, len(o.ordersActiveByID)+chunkSize)

	for _, order := range copyOrdersActiveByID {
		o.addOrder(order, true)
	}

	// running GC will collect the deleted maps/cache which we have just re-initialized
	runtime.GC()
}

func (o *OMS) sendDataEvent(event *data.SyncEvent) error {
	bytes, err := event.ToBinary()
	if err != nil {
		log.Error().
			Str("section", "OMS").
			Str("method", "sendDataEvent").
			Str("payload", fmt.Sprintf("%+v", event.Payload)).
			Err(err).Msg("Unable to serialize event")
		return err
	}

	msg := kafka.Message{Value: bytes}
	if err := o.dm.Publish("sync_data", map[string]string{}, msg); err != nil {
		log.Error().Str("section", "OMS").Str("topic", "sync_data").Str("method", "sendDataEvent").Err(err).Msg("Unable to send event")
		return err
	}

	return nil
}

var oInstance *OMS

func Init(repo *queries.Repo, dm manager.DataManagerInterface, enc data.Encoder, rootCtx context.Context) *OMS {
	if oInstance != nil {
		panic("OMS already inited")
	}

	oInstance = &OMS{
		ordersActive:         make(map[string]map[uint64]*model.Order),
		ordersActiveByClient: make(map[uint64]map[string]map[uint64]*model.Order),
		ordersActiveByID:     make(map[uint64]model.Order),
		ordersLock:           &sync.RWMutex{},
		rootCtx:              rootCtx,
		repo:                 repo,
		LastPrice: &lastPrice{
			repo:   repo,
			prices: map[string]LastPriceItem{},
			lock:   &sync.RWMutex{},
		},
		Seq: &seq{
			repo: repo,
			ctx:  rootCtx,
		},
		marketDepthCache: &marketDepthCache{
			ob:   map[string]marketDepthCacheItem{},
			lock: &sync.RWMutex{},
		},
		dm:      dm,
		encoder: enc,
	}

	oInstance.Seq.init()

	var orders []model.Order
	if err := repo.Conn.Table("orders").
		Where("status in (?)", []model.OrderStatus{model.OrderStatus_Pending, model.OrderStatus_Untouched, model.OrderStatus_PartiallyFilled}).
		Find(&orders).Error; err != nil {
		log.Error().Err(err).
			Str("service", "OMS").
			Str("method", "Init").
			Msg("Unable to load active orders")
	}

	oInstance.ordersLock.Lock()
	for _, order := range orders {
		oInstance.addOrder(order, true)
	}
	oInstance.ordersLock.Unlock()

	gostop.GetInstance().Go("market_depth_cache_processor", oInstance.marketDepthCacheProcessor, true)
	gostop.GetInstance().Go("cron_active_monitor", oInstance.cronActiveMonitor, true)
	gostop.GetInstance().Go("cron_cache_cleanup", oInstance.cronCacheCleanup, true)

	return oInstance
}
