package oms

import (
	"context"
	. "github.com/smartystreets/goconvey/convey"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSeq_NextOrderAndTradeID(t *testing.T) {
	r, mock := setupRepo()
	ctx := context.TODO()

	testSeq := seq{
		repo: r,
		ctx:  ctx,
	}

	Convey("it should return nextOrderID", t, func() {
		testSeq.init()
		orderID := testSeq.NextOrderID()
		tradeID := testSeq.NextTradeID()

		So(orderID, ShouldEqual, 1)
		So(tradeID, ShouldEqual, 1)
	})

	Convey("it should get last id from database and assign to seq obj", t, func() {
		mock.ExpectQuery("select last_value from trades_id_seq").
			WillReturnRows(mock.NewRows([]string{"last_value"}).AddRow(20000000))
		mock.ExpectQuery("select last_value from orders_id_seq").
			WillReturnRows(mock.NewRows([]string{"last_value"}).AddRow(20000000))

		testSeq.init()
		orderID := testSeq.NextOrderID()
		tradeID := testSeq.NextTradeID()

		So(orderID, ShouldEqual, 20000001)
		So(tradeID, ShouldEqual, 20000001)
	})

	Convey("if currentSeqId greater then previous", t, func() {
		atomic.StoreUint64(testSeq.lastOrderSeq, 200001)
		atomic.StoreUint64(testSeq.lastTradeSeq, 200001)
		atomic.StoreUint64(testSeq.prevTradeSeq, 200000)
		atomic.StoreUint64(testSeq.prevOrderSeq, 200000)

		mock.ExpectQuery("SELECT setval('orders_id_seq', $1, true);").
			WithArgs(200001).
			RowsWillBeClosed()
		mock.ExpectQuery("SELECT setval('trades_id_seq', $1, true);").
			WithArgs(200001).
			RowsWillBeClosed()

		ctx, cancel := context.WithCancel(context.Background())
		wait := &sync.WaitGroup{}
		wait.Add(1)
		go testSeq.refresher(ctx, wait)
		time.Sleep(2 * time.Second)

		orderID := testSeq.NextOrderID()
		tradeID := testSeq.NextTradeID()
		cancel()
		wait.Wait()

		So(orderID, ShouldEqual, 200002)
		So(tradeID, ShouldEqual, 200002)
	})
}
