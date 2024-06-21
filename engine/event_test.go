package engine_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	model "gitlab.com/paramountdax-exchange/exchange_api_v2/data"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/engine"
)

func TestEventsUsage(t *testing.T) {
	Convey("Create a new event", t, func() {
		order := model.Order{ID: 1, Price: 1200000000, Amount: 121300000000}
		encoded, _ := order.ToBinary()
		event := engine.Event{}
		Convey("I should be able to decode the message as an order", func() {
			event.DecodeFromBinary(encoded)
			So(event.Order.ID, ShouldEqual, 1)
			So(event.Order.Price, ShouldEqual, 1200000000)
			So(event.Order.Amount, ShouldEqual, 121300000000)
			So(event.HasEvents(), ShouldEqual, false)
			events := make([]model.Event, 2)
			event.SetEvents(events)
			So(event.HasEvents(), ShouldEqual, true)
		})
	})
}
