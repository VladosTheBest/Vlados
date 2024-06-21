package oms

import (
	"github.com/DATA-DOG/go-sqlmock"
	. "github.com/smartystreets/goconvey/convey"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"sync"
	"testing"
)

func TestLastPrice_Get(t *testing.T) {
	r, mock := setupRepo()
	prices := make(map[string]LastPriceItem)
	lock := &sync.RWMutex{}

	testLastPrice := lastPrice{
		repo:   r,
		prices: prices,
		lock:   lock,
	}
	trade := getTestTrade(testOrderId, marketId, testUserId, testUserId1, model.MarketSide_Buy)

	Convey("last price should get set", t, func() {
		testLastPrice.Set(trade)
		priceItem := testLastPrice.Get(marketId)

		So(trade.Price.V, ShouldEqual, priceItem.Price)
	})

	Convey("get a market last price which has not yet assigned should set an empty trade", t, func() {
		price := "28563.77000000"
		mock.ExpectQuery("SELECT * FROM \"trades\" WHERE market_id = $1 ORDER BY seqid DESC,\"trades\".\"id\" LIMIT 1").
			WithArgs("bchusdt").
			WillReturnRows(sqlmock.NewRows([]string{"id", "bid_sub_account", "timestamp", "bid_owner_id", "status", "ask_sub_account", "ask_fee_amount", "market_id", "price", "market_ask_fee_amount", "bid_id", "quote_volume", "ask_id", "ask_owner_id", "ref_id", "bid_fee_amount", "taker_side", "seqid", "volume", "quote_bid_fee_amount", "event_seqid"}).
				AddRow("7118", "0", "1683134707800959831", "6", "created", "0", "20.34062980", "bchusdt", price, "5.793930154349755159882500000000000", "819660", "8136.25192149", "210974", "6", "", "0.00142422", "buy", "2466", "0.28484517", "40.68125960745450000000000000000000", "1203674"))
		testLastPrice := testLastPrice.Get("bchusdt")

		So(testLastPrice.Price.String(), ShouldEqual, price)
	})
}
