package oms

import (
	"errors"
	. "github.com/smartystreets/goconvey/convey"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestMarketDepthCache_SetDepth(t *testing.T) {
	testMarketDepthCache := marketDepthCache{
		ob:   make(map[string]marketDepthCacheItem),
		lock: &sync.RWMutex{},
	}

	Convey("it should set the depth in the market depth cache", t, func() {
		l1 := getTestMarketDepthLevel1()
		l2 := getTestMarketDepthLevel2()
		testMarketDepthCacheItem := marketDepthCacheItem{
			level2: *l2,
			level1: *l1,
		}

		testMarketDepthCache.SetDepth(marketId, testMarketDepthCacheItem)
		testL1, err := testMarketDepthCache.GetDepthLevel1(marketId)
		So(err, ShouldBeNil)
		testL2, err := testMarketDepthCache.GetDepthLevel2(marketId)
		So(err, ShouldBeNil)

		So(testL1, ShouldResemble, l1)
		So(testL2, ShouldResemble, l2)
	})

	Convey("it should return err when depth not found", t, func() {
		const market = "solusdt"
		expectedErr := errors.New("depth not found")
		_, err := testMarketDepthCache.GetDepthLevel1(market)
		So(err, ShouldResemble, expectedErr)
		_, err = testMarketDepthCache.GetDepthLevel2(market)
		So(err, ShouldResemble, expectedErr)
	})
}

func getTestMarketDepthLevel1() *model.MarketDepthLevel1 {
	price := conv.NewDecimalWithPrecision()
	price.SetString(conv.FromUnits(1000, 8))
	endPrice := conv.NewDecimalWithPrecision()
	endPrice.SetString(conv.FromUnits(8000, 8))
	marketDepthLevel1 := model.MarketDepthLevel1{
		Timestamp:  time.Now().UnixNano(),
		BidPrice:   price.String(),
		BidVolume:  price.String(),
		AskPrice:   price.String(),
		AskVolume:  price.String(),
		LastPrice:  endPrice.String(),
		LastVolume: endPrice.String(),
	}

	return &marketDepthLevel1
}

func getTestMarketDepthLevel2() *model.MarketDepthLevel2 {
	price := conv.NewDecimalWithPrecision()
	price.SetString(conv.FromUnits(1000, 8))
	endPrice := conv.NewDecimalWithPrecision()
	endPrice.SetString(conv.FromUnits(8000, 8))
	var bids [][2]string
	var asks [][2]string

	var i int
	for ; i < 5; i++ {
		p := rand.Int()
		pr := strconv.Itoa(p)
		bids = append(bids, [2]string{pr, pr})
		asks = append(asks, [2]string{pr, pr})
	}

	marketDepthLevel2 := model.MarketDepthLevel2{
		Timestamp:  time.Now().UnixNano(),
		LastPrice:  endPrice.String(),
		LastVolume: price.String(),
		Bids:       bids,
		Asks:       asks,
	}

	return &marketDepthLevel2
}
