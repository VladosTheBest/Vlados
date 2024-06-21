package oms

import (
	"context"
	"sort"
	"sync"
	"time"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"

	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/markets"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"

	"github.com/ericlagergren/decimal"
)

func (o *OMS) GetMarketDepthLevel2(market *model.Market, limit int) (*model.MarketDepthLevel2, error) {
	return o.GetMarketDepthLevel2ByID(market.ID, limit)
}

func (o *OMS) GetMarketDepthLevel2ByID(market_id string, limit int) (*model.MarketDepthLevel2, error) {
	depth, err := o.marketDepthCache.GetDepthLevel2(market_id)
	if err != nil {
		return nil, err
	}

	lenAsks := len(depth.Asks)
	lenBids := len(depth.Bids)

	if limit > 0 {
		if len(depth.Asks) > limit {
			lenAsks = limit
		}
		if len(depth.Bids) > limit {
			lenBids = limit
		}
	}

	return &model.MarketDepthLevel2{
		Timestamp:  depth.Timestamp,
		LastPrice:  depth.LastPrice,
		LastVolume: depth.LastVolume,
		Bids:       depth.Bids[:lenBids],
		Asks:       depth.Asks[:lenAsks],
	}, nil
}

func (o *OMS) GetMarketDepthLevel1(market *model.Market) (*model.MarketDepthLevel1, error) {
	return o.marketDepthCache.GetDepthLevel1(market.ID)
}

func (o *OMS) getActiveOrders(marketID string) map[uint64]*model.Order {
	o.ordersLock.RLock()
	defer o.ordersLock.RUnlock()
	clones := map[uint64]*model.Order{}
	orders, ok := o.ordersActive[marketID]
	if !ok {
		return clones
	}
	for idx, ord := range orders {
		clones[idx] = ord.Clone()
	}
	return clones
}

type PriceLevel struct {
	Price  *decimal.Big
	Volume *decimal.Big
}
type DepthLevel map[string]*PriceLevel

type SortablePriceLevels []*PriceLevel
type SortablePriceLevelsDesc []*PriceLevel

func (os SortablePriceLevels) Len() int           { return len(os) }
func (os SortablePriceLevels) Swap(i, j int)      { os[i], os[j] = os[j], os[i] }
func (os SortablePriceLevels) Less(i, j int) bool { return os[i].Price.Cmp(os[j].Price) == -1 }

func (os SortablePriceLevelsDesc) Len() int           { return len(os) }
func (os SortablePriceLevelsDesc) Swap(i, j int)      { os[i], os[j] = os[j], os[i] }
func (os SortablePriceLevelsDesc) Less(i, j int) bool { return os[i].Price.Cmp(os[j].Price) == 1 }

func getDepthLevel(market *model.Market, orders map[uint64]*model.Order) model.MarketDepthLevel2 {
	bids, asks := DepthLevel{}, DepthLevel{}
	for _, order := range orders {
		if order.Status == model.OrderStatus_Filled ||
			order.Status == model.OrderStatus_Cancelled ||
			order.Status == model.OrderStatus_Pending {
			continue
		}
		if order.Type == model.OrderType_Market {
			continue
		}
		price := order.Price.V.Quantize(market.MarketPrecision).String()
		if order.Side == model.MarketSide_Sell {
			if _, ok := asks[price]; !ok {
				dec := decimal.Big{}
				dec.Context = decimal.Context128
				dec.Context.RoundingMode = decimal.ToZero
				dec.Quantize(8)
				asks[price] = new(PriceLevel)
				asks[price].Price = order.Price.V
				asks[price].Volume = &dec
			}
			unfilledAmount := conv.NewDecimalWithPrecision().Sub(order.Amount.V, order.FilledAmount.V)
			asks[price].Volume.Add(asks[price].Volume, unfilledAmount)
		} else {
			if _, ok := bids[price]; !ok {
				dec := decimal.Big{}
				dec.Context = decimal.Context128
				dec.Context.RoundingMode = decimal.ToZero
				dec.Quantize(8)
				bids[price] = new(PriceLevel)
				bids[price].Price = order.Price.V
				bids[price].Volume = &dec
			}
			unfilledAmount := conv.NewDecimalWithPrecision().Sub(order.Amount.V, order.FilledAmount.V)
			bids[price].Volume.Add(bids[price].Volume, unfilledAmount)
		}
	}

	sortedAsks := make(SortablePriceLevels, len(asks))
	sortedBids := make(SortablePriceLevelsDesc, len(bids))

	i := 0
	for _, pl := range asks {
		sortedAsks[i] = pl
		i++
	}
	i = 0
	for _, pl := range bids {
		sortedBids[i] = pl
		i++
	}
	sort.Sort(sortedAsks)
	sort.Sort(sortedBids)

	lvl2 := model.MarketDepthLevel2{
		Asks: [][2]string{},
		Bids: [][2]string{},
	}

	for _, pl := range sortedAsks {
		lvl2.Asks = append(lvl2.Asks, [2]string{
			pl.Price.Quantize(market.MarketPrecision).String(),
			pl.Volume.Quantize(market.QuotePrecision).String(),
		})
	}
	for _, pl := range sortedBids {
		lvl2.Bids = append(lvl2.Bids, [2]string{
			pl.Price.Quantize(market.MarketPrecision).String(),
			pl.Volume.Quantize(market.QuotePrecision).String(),
		})
	}
	return lvl2
}

func (o *OMS) updateMarketDepthCache(market *model.Market) {
	orders := o.getActiveOrders(market.ID)
	lvl1 := model.MarketDepthLevel1{}
	lvl2 := getDepthLevel(market, orders)

	lp := o.LastPrice.Get(market.ID)
	lvl2.Timestamp = lp.Time
	lvl2.LastPrice = lp.Price.String()
	lvl2.LastVolume = lp.Volume.String()

	lvl1.Timestamp = lvl2.Timestamp
	lvl1.LastPrice = lvl2.LastPrice
	lvl1.LastVolume = lvl2.LastVolume

	if len(lvl2.Bids) > 0 {
		lvl1.BidPrice = lvl2.Bids[0][0]
		lvl1.BidVolume = lvl2.Bids[0][1]
	}

	if len(lvl2.Asks) > 0 {
		lvl1.AskPrice = lvl2.Asks[0][0]
		lvl1.AskVolume = lvl2.Asks[0][1]
	}

	o.marketDepthCache.SetDepth(market.ID, marketDepthCacheItem{
		level2: lvl2,
		level1: lvl1,
	})
}

func (o *OMS) marketDepthCacheProcessor(ctx context.Context, wait *sync.WaitGroup) {
	log.Info().Str("cron", "oms_market_depth_cache_processor").Str("action", "start").Msg("OMS Market depth cache processor - started")

	ticker := time.NewTicker(250 * time.Millisecond)
	tickerUpdateMarkets := time.NewTicker(15 * time.Second)
	marketsCache := markets.GetAll()

	for {
		select {
		case <-tickerUpdateMarkets.C:
			marketsCache = markets.GetAll()
		case <-ticker.C:
			for _, market := range marketsCache {
				o.updateMarketDepthCache(market)
			}
		case <-ctx.Done():
			tickerUpdateMarkets.Stop()
			ticker.Stop()
			log.Info().Str("cron", "oms_market_depth_cache_processor").Str("action", "stop").Msg("10 => OMS Market depth cache processor - stopped")
			wait.Done()
			return
		}
	}
}
