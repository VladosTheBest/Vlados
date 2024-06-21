package oms

import (
	"context"
	"errors"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/data"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/net/manager"
	"math"
	"runtime"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

var (
	OrderNotFoundErr        = errors.New("order not found")
	marketId                = "btcusdt"
	testUserId       uint64 = 6
	testUserId1      uint64 = 7
	testOrderId      uint64 = 1
	testOrderId1     uint64 = 2
	expectedOrder           = getTestOrder(testOrderId, marketId, testUserId)
	expectedOrder1          = getTestOrder(testOrderId1, marketId, testUserId1)
	expectedTrade           = getTestTrade(1, marketId, testUserId, testUserId1, model.MarketSide_Buy)
	expectedTrade1          = getTestTrade(2, marketId, testUserId, testUserId1, model.MarketSide_Sell)
)

func TestSendOrder(t *testing.T) {
	dm_ := setupDM()
	r, _ := setupRepo()
	ctx := context.TODO()
	enc := data.NewWireEncoder("wire")
	oInstance = nil
	Init(r, dm_, enc, ctx)

	//dm := oInstance.dm
	//dm.SetMarkets([]string{})
	////NOTE: commented because it needs kafka server (change the kafka configs in util_test.go)
	//dm.Start(ctx)

	//topic := "orders"
	//
	//// NOTE: uncomment if want to test consumer as well
	////dm.Subscribe(topic, func(msg kafkaGo.Message) error {
	////	ev := data.DataEvent{}
	////	err := ev.FromBinary(msg.Value)
	////	assert.Nil(t, err)
	////	fmt.Println("got event in consumer, ", ev.Command, ev.Model, ev.GetOrder().ID)
	////	return nil
	////})
	//for i := 0; i < 11; i++ {
	//	order := getTestOrder(uint64(i), "btcusdt", 7)
	//	event := data.NewSaveDataEvent(oInstance.encoder, "orders", *order)
	//	bytes, err := event.ToBinary()
	//	assert.Nil(t, err)
	//	msg := kafkaGo.Message{
	//		Value: bytes,
	//	}
	//	err = dm.Publish(topic, nil, msg)
	//	time.Sleep(time.Second * 5)
	//}
}

func TestOMSCache(t *testing.T) {
	r, _ := setupRepo()
	oInstance = nil
	Init(r, nil, nil, context.TODO())

	Convey("OMS: Should not increase memory after save order", t, func() {
		memoryBefore := getAllocatedMemoryInMB()
		testSaveOrder(t)
		oInstance.cleanupCache()
		memoryAfter := getAllocatedMemoryInMB()

		// memory (in MB) before and after should not be greater than 1.
		// 1 is used as grace unit
		So(math.Abs(float64(memoryBefore-memoryAfter)), ShouldBeLessThanOrEqualTo, 1)
	})
}

func testSaveOrder(t *testing.T) {
	marketIds := []string{"btcusdt", "ethusdt", "eosusdt", "prdxusdt", "solusdt", "piusdt", "btceth", "btcbusd"}
	var totalUsers uint64 = 125
	var userOrders uint64 = 1000
	populateOrders(totalUsers, userOrders, marketIds)
	deleteOrders(totalUsers, userOrders, marketIds)
}

func populateOrders(users, orders uint64, markets []string) {
	var user uint64 = 0
	for ; user < users; user++ {
		var orderID uint64 = 0
		for ; orderID < orders; orderID++ {
			for _, markerID := range markets {
				order := getTestOrder(orderID, markerID, user)
				oInstance.addOrder(*order, true)
			}
		}
	}
}

func deleteOrders(users, orders uint64, markets []string) {
	var user uint64 = 0
	for ; user < users; user++ {
		var orderID uint64 = 0
		for ; orderID < orders; orderID++ {
			for _, markerID := range markets {
				oInstance.removeCompletedOrder(markerID, orderID, user)
			}
		}
	}
}

func getAllocatedMemoryInMB() int {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int(m.Alloc / 1024 / 1024)
}

func TestOMS_GetOrder(t *testing.T) {
	r, _ := setupRepo()
	oInstance = nil
	Init(r, nil, nil, context.TODO())

	Convey("returned order should be equal to expected", t, func() {
		oInstance.addOrder(*expectedOrder, true)
		order, err := oInstance.GetOrder(marketId, testOrderId)

		So(err, ShouldBeNil)
		So(order, ShouldResemble, expectedOrder)
	})

	Convey("get error with invalid orderId", t, func() {
		order, err := oInstance.GetOrder(marketId, 11)

		So(err, ShouldResemble, OrderNotFoundErr)
		So(order, ShouldBeNil)
	})
}

func TestOMS_GetTradeOrders(t *testing.T) {
	r, _ := setupRepo()
	oInstance = nil
	Init(r, nil, nil, context.TODO())

	Convey("it should return expected ask and bid Order", t, func() {
		ord1 := expectedOrder.Clone()
		ord2 := expectedOrder1.Clone()
		oInstance.addOrder(*ord1, false)
		oInstance.addOrder(*ord2, false)
		order1, order2, err1, err2 := oInstance.GetTradeOrders(marketId, testOrderId, testOrderId1)

		So(err1, ShouldBeNil)
		So(err2, ShouldBeNil)
		So(order1, ShouldResemble, ord1)
		So(order2, ShouldResemble, ord2)
	})

	Convey("it should return err on invalid askId", t, func() {
		ord2 := expectedOrder1.Clone()
		order1, order2, err1, err2 := oInstance.GetTradeOrders(marketId, 3, testOrderId1)

		So(err1.Error(), ShouldEqual, ErrTradeOrderNotFound.Error())
		So(err2, ShouldBeNil)
		So(order1, ShouldBeNil)
		So(order2, ShouldResemble, ord2)
	})

	Convey("it should return err on invalid bidId", t, func() {
		ord1 := expectedOrder.Clone()
		order1, order2, err1, err2 := oInstance.GetTradeOrders(marketId, testOrderId, 3)

		So(err1, ShouldBeNil)
		So(err2.Error(), ShouldEqual, ErrTradeOrderNotFound.Error())
		So(order1, ShouldResemble, ord1)
		So(order2, ShouldBeNil)
	})
}

func TestOMS_GetOrderByID(t *testing.T) {
	r, _ := setupRepo()
	oInstance = nil
	Init(r, nil, nil, context.TODO())

	Convey("it should return expected order by id", t, func() {
		oInstance.addOrder(*expectedOrder, false)
		order, err := oInstance.GetOrderByID(testOrderId)

		So(err, ShouldBeNil)
		So(order, ShouldResemble, expectedOrder)
	})

	Convey("it should return error on invalid id", t, func() {
		order, err := oInstance.GetOrderByID(testOrderId1)

		So(err, ShouldResemble, OrderNotFoundErr)
		So(order, ShouldBeNil)
	})
}

func TestOMS_GetOrdersByUser(t *testing.T) {
	const market = "btcusdt"
	r, _ := setupRepo()
	oInstance = nil
	Init(r, nil, nil, context.TODO())
	var i uint64 = 0
	expectedOrders := make(map[string]map[uint64]model.Order)

	Convey("get all orders from user by userId", t, func() {
		expectedOrders[market] = map[uint64]model.Order{}
		for ; i < 5; i++ {
			o := getTestOrder(i, market, 6)
			oInstance.addOrder(*o, true)
			expectedOrders[market][i] = *o.Clone()
		}
		orders := oInstance.GetOrdersByUser(6)

		So(orders, ShouldResemble, expectedOrders)
	})
}

func TestOMS_GetOrdersByMarketID(t *testing.T) {
	r, _ := setupRepo()
	oInstance = nil
	Init(r, nil, nil, context.TODO())
	var i uint64 = 0
	var expectedOrders []*model.Order

	Convey("get orders by marketId", t, func() {
		for ; i < 5; i++ {
			o := getTestOrder(i, marketId, 6)
			oInstance.addOrder(*o, true)
			expectedOrders = append(expectedOrders, o)
		}
		orders, err := oInstance.GetOrdersByMarketID(marketId)

		So(err, ShouldBeNil)
		So(orders, ShouldResemble, expectedOrders)
	})

	Convey("should return error on invalid marketId", t, func() {
		orders, err := oInstance.GetOrdersByMarketID("bchusdt")

		So(err, ShouldResemble, OrderNotFoundErr)
		So(orders, ShouldResemble, []*model.Order{})
	})
}

func TestOMS_SaveOrder(t *testing.T) {
	r, _ := setupRepo()
	ctx := context.TODO()
	oInstance = nil
	enc := data.NewWireEncoder("wire")
	dm := manager.NewMockDataManager([]string{"sync_data"})

	Init(r, dm, enc, ctx)

	Convey("order should get saved", t, func() {
		err := oInstance.SaveOrder(expectedOrder)
		msg := getEventFromDataManager("sync_data", dm.(*manager.MockDataManager))

		event := data.SyncEvent{}
		err = event.FromBinary(msg.Value)

		So(err, ShouldBeNil)
		So(event.Payload["id"].GetUint64Value(), ShouldEqual, expectedOrder.ID)
	})
}

func TestOMS_UpdateOrder(t *testing.T) {
	r, _ := setupRepo()
	ctx := context.TODO()
	oInstance = nil
	enc := data.NewWireEncoder("wire")
	dm := manager.NewMockDataManager([]string{"sync_data"})
	Init(r, dm, enc, ctx)

	Convey("order update event should get send via kafka", t, func() {
		o := oInstance.addOrder(*expectedOrder, true).Clone()
		o.Status = model.OrderStatus_PartiallyFilled
		Err := oInstance.UpdateOrder(o)
		msg := getEventFromDataManager("sync_data", dm.(*manager.MockDataManager))

		event := data.SyncEvent{}
		err := event.FromBinary(msg.Value)

		So(Err, ShouldBeNil)
		So(err, ShouldBeNil)
		So(event.Payload["status"].GetStrValue(), ShouldEqual, o.Status)
	})

	Convey("update order which is not yet created", t, func() {
		err := oInstance.UpdateOrder(expectedOrder1)
		So(err, ShouldResemble, OrderNotFoundErr)
	})

	Convey("update order status with same order status", t, func() {
		testOrder := getTestOrder(3, marketId, testUserId)
		o := oInstance.addOrder(*testOrder, true)
		err := oInstance.UpdateOrder(o)
		So(err, ShouldBeNil)
	})

	Convey("update order with invalid order status", t, func() {
		expectedOrder1.Status = model.OrderStatus_PartiallyFilled
		o := oInstance.addOrder(*expectedOrder1, true).Clone()
		o.Status = model.OrderStatus_Pending
		err := oInstance.UpdateOrder(o)

		So(err, ShouldBeNil)
	})

	Convey("update order with status filled or cancelled", t, func() {
		testOrder := getTestOrder(3, marketId, testUserId)
		o := oInstance.addOrder(*testOrder, true).Clone()
		o.Status = model.OrderStatus_Filled
		err := oInstance.UpdateOrder(o)

		So(err, ShouldBeNil)
	})
}

func TestOMS_UpdateOrderStatus(t *testing.T) {
	r, _ := setupRepo()
	ctx := context.TODO()
	oInstance = nil
	enc := data.NewWireEncoder("wire")
	dm := manager.NewMockDataManager([]string{"sync_data"})
	Init(r, dm, enc, ctx)

	Convey("order update event should get send via kafka", t, func() {
		_ = oInstance.addOrder(*expectedOrder, true).Clone()
		ok, Err := oInstance.UpdateOrderStatus(marketId, testOrderId, model.OrderStatus_PartiallyFilled, time.Now(), true)
		msg := getEventFromDataManager("sync_data", dm.(*manager.MockDataManager))

		event := data.SyncEvent{}
		err := event.FromBinary(msg.Value)

		So(Err, ShouldBeNil)
		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)
		So(event.Payload["status"].GetStrValue(), ShouldEqual, model.OrderStatus_PartiallyFilled)
	})

	Convey("update order which is not yet created", t, func() {
		ok, err := oInstance.UpdateOrderStatus(marketId, 4, model.OrderStatus_PartiallyFilled, time.Now(), true)
		So(err, ShouldResemble, OrderNotFoundErr)
		So(ok, ShouldBeFalse)
	})

	Convey("update order status with same order status", t, func() {
		testOrder := getTestOrder(3, marketId, testUserId)
		_ = oInstance.addOrder(*testOrder, true)
		ok, err := oInstance.UpdateOrderStatus(marketId, testOrderId, model.OrderStatus_Pending, time.Now(), true)
		So(err, ShouldBeNil)
		So(ok, ShouldBeFalse)
	})

	Convey("update order with invalid order status", t, func() {
		expectedOrder1.Status = model.OrderStatus_PartiallyFilled
		_ = oInstance.addOrder(*expectedOrder1, true).Clone()
		ok, err := oInstance.UpdateOrderStatus(marketId, testOrderId, model.OrderStatus_PartiallyFilled, time.Now(), true)

		So(err, ShouldBeNil)
		So(ok, ShouldBeFalse)
	})

	Convey("update order with status filled or cancelled", t, func() {
		testOrder := getTestOrder(3, marketId, testUserId)
		_ = oInstance.addOrder(*testOrder, true).Clone()
		ok, err := oInstance.UpdateOrderStatus(marketId, testOrderId, model.OrderStatus_Cancelled, time.Now(), true)

		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)
	})
}

func TestOMS_SaveTrade(t *testing.T) {
	r, _ := setupRepo()
	oInstance = nil
	enc := data.NewWireEncoder("wire")
	dm := manager.NewMockDataManager([]string{"sync_data"})
	Init(r, dm, enc, context.TODO())

	Convey("trade event should be send to kafka", t, func() {
		err := oInstance.SaveTrade(expectedTrade)
		msg := getEventFromDataManager("sync_data", dm.(*manager.MockDataManager))
		event := data.SyncEvent{}
		Err := event.FromBinary(msg.Value)

		So(Err, ShouldBeNil)
		So(err, ShouldBeNil)
		So(event.Payload["id"].GetUint64Value(), ShouldEqual, expectedTrade.ID)
	})
}

func TestOMS_SaveRevenues(t *testing.T) {
	r, _ := setupRepo()
	oInstance = nil
	enc := data.NewWireEncoder("wire")
	dm := manager.NewMockDataManager([]string{"sync_data"})
	Init(r, dm, enc, context.TODO())
	testRevenue := getTestRevenue(1, model.OperationType_Deposit, testOrderId)

	Convey("revenues obj should be sent over kafka to save into DB", t, func() {
		err := oInstance.SaveRevenues(testRevenue)
		So(err, ShouldBeNil)

		msg := getEventFromDataManager("sync_data", dm.(*manager.MockDataManager))
		event := data.SyncEvent{}
		err = event.FromBinary(msg.Value)

		So(err, ShouldBeNil)
		So(event.Payload["id"].GetUint64Value(), ShouldEqual, 1)
	})
}

func TestOMS_SaveLiabilities(t *testing.T) {
	r, _ := setupRepo()
	oInstance = nil
	enc := data.NewWireEncoder("wire")
	dm := manager.NewMockDataManager([]string{"sync_data"})
	Init(r, dm, enc, context.TODO())
	testLiability := getTestLiability(1, model.OperationType_Withdraw, testOrderId)

	Convey("liabilities obj should be sent over kafka to save into DB", t, func() {
		err := oInstance.SaveLiabilities(testLiability)
		So(err, ShouldBeNil)

		msg := getEventFromDataManager("sync_data", dm.(*manager.MockDataManager))
		event := data.SyncEvent{}
		err = event.FromBinary(msg.Value)

		So(err, ShouldBeNil)
		So(event.Payload["id"].GetUint64Value(), ShouldEqual, 1)
	})
}

func TestOMS_SaveReferralEarnings(t *testing.T) {
	r, _ := setupRepo()
	oInstance = nil
	enc := data.NewWireEncoder("wire")
	dm := manager.NewMockDataManager([]string{"sync_data"})
	Init(r, dm, enc, context.TODO())
	testReferralEarning := getReferralEarning(1, model.OperationType_Withdraw, testOrderId)

	Convey("liabilities obj should be sent over kafka to save into DB", t, func() {
		err := oInstance.SaveReferralEarnings(testReferralEarning)
		So(err, ShouldBeNil)

		msg := getEventFromDataManager("sync_data", dm.(*manager.MockDataManager))
		event := data.SyncEvent{}
		err = event.FromBinary(msg.Value)

		So(err, ShouldBeNil)
		So(event.Payload["id"].GetUint64Value(), ShouldEqual, 1)
	})
}

func TestOMS_SaveOperations(t *testing.T) {
	r, _ := setupRepo()
	oInstance = nil
	enc := data.NewWireEncoder("wire")
	dm := manager.NewMockDataManager([]string{"sync_data"})
	Init(r, dm, enc, context.TODO())
	testOperation := getTestOperation(1, model.OperationType_Withdraw, model.OperationStatus_Accepted)

	Convey("liabilities obj should be sent over kafka to save into DB", t, func() {
		err := oInstance.SaveOperations(testOperation)
		So(err, ShouldBeNil)

		msg := getEventFromDataManager("sync_data", dm.(*manager.MockDataManager))
		event := data.SyncEvent{}
		err = event.FromBinary(msg.Value)

		So(err, ShouldBeNil)
		So(event.Payload["id"].GetUint64Value(), ShouldEqual, 1)
	})
}
