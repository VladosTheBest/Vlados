package oms

import (
	"context"

	"github.com/ericlagergren/decimal"

	"math/rand"
	"sort"
	"strconv"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

func TestOMS_GetMarketDepthLevel2ByID(t *testing.T) {
	r, _ := setupRepo()
	oInstance = nil
	Init(r, nil, nil, context.TODO())

	rand.Seed(time.Now().UnixNano())

	Convey("OMS: Should be able to generate market depth", t, func() {
		const marketId = "btcusdt"
		market := getTestMarket(marketId)
		orders := getTestMarketDepthOrders(marketId)

		oInstance.ordersActive[marketId] = make(map[uint64]*model.Order)
		for _, order := range orders {
			order := order
			oInstance.ordersActive[marketId][order.ID] = order
		}
		oInstance.updateMarketDepthCache(market)
		depth, err := oInstance.GetMarketDepthLevel2ByID(marketId, 50)

		So(err, ShouldBeNil)
		bidsPrice := make([]string, 0)
		asksPrice := make([]string, 0)
		for _, price := range depth.Bids {
			bidsPrice = append(bidsPrice, price[0])
		}
		for _, price := range depth.Asks {
			asksPrice = append(asksPrice, price[0])
		}

		So(sort.SliceIsSorted(bidsPrice, func(i, j int) bool {
			first, _ := strconv.ParseFloat(bidsPrice[i], 64)
			second, _ := strconv.ParseFloat(bidsPrice[j], 64)
			return first >= second
		}), ShouldBeTrue)
		So(sort.SliceIsSorted(asksPrice, func(i, j int) bool {
			first, _ := strconv.ParseFloat(asksPrice[i], 64)
			second, _ := strconv.ParseFloat(asksPrice[j], 64)
			return first <= second
		}), ShouldBeTrue)
	})
}

func TestOMS_GetMarketDepthLevel1(t *testing.T) {
	r, _ := setupRepo()
	oInstance = nil
	Init(r, nil, nil, context.TODO())

	Convey("should be able to get market depth", t, func() {
		market := getTestMarket(marketId)
		orders := getTestMarketDepthOrders(marketId)

		oInstance.ordersActive[marketId] = make(map[uint64]*model.Order)
		for _, order := range orders {
			oInstance.ordersActive[marketId][order.ID] = order
		}

		oInstance.updateMarketDepthCache(market)
		marketDepth, err := oInstance.GetMarketDepthLevel1(market)
		So(err, ShouldBeNil)

		depthLvl2, err := oInstance.GetMarketDepthLevel2(market, 5)
		So(err, ShouldBeNil)

		So(err, ShouldBeNil)
		So(marketDepth.Timestamp, ShouldEqual, depthLvl2.Timestamp)
		So(marketDepth.LastVolume, ShouldEqual, depthLvl2.LastVolume)
		So(marketDepth.LastPrice, ShouldEqual, depthLvl2.LastPrice)
	})
}

func getTestMarketDepthOrders(marketId string) []*model.Order {
	const n = 40
	const min = 10000000
	const max = 99999999
	const precision = 7

	orders := make([]*model.Order, 0, 40)

	for i := 0; i < n; i++ {
		price := getRandPrice(min, max, precision)
		order := getTestOrderWithPrice(uint64(i), marketId, 0, price)
		order.Status = model.OrderStatus_Untouched
		order.Type = model.OrderType_Limit
		if i%2 == 0 {
			order.Side = model.MarketSide_Buy
		} else {
			order.Side = model.MarketSide_Sell
		}
		orders = append(orders, order)
	}

	return orders
}

func getRandInt(min, max int) uint64 {
	return uint64(rand.Intn(max-min+1) + min)
}

func getRandPrice(min, max, precision int) *decimal.Big {
	price := conv.NewDecimalWithPrecision()
	price.SetMantScale(int64(getRandInt(min, max)), rand.Intn(precision))
	return price
}
