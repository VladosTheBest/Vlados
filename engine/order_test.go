package engine

import (
	"testing"

	model "gitlab.com/paramountdax-exchange/exchange_api_v2/data"

	. "github.com/smartystreets/goconvey/convey"
)

func TestOrderCreation(t *testing.T) {
	Convey("Should be able to create a new order", t, func() {
		order := model.NewOrder(1, 100000000, 12000000000, 1, 2, 1)
		So(order.Amount, ShouldEqual, 12000000000)
		So(order.Price, ShouldEqual, 100000000)
		So(order.Side, ShouldEqual, 1)
		So(order.Type, ShouldEqual, 2)
	})
}
