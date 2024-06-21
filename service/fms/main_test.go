package fms

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	. "github.com/smartystreets/goconvey/convey"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"testing"
)

func TestFundsEngine_InitAccountsAndGetAccountBalances(t *testing.T) {
	r, mock := setupRepo()
	ctx := context.TODO()
	fm := Init(r, ctx)

	bMap := make(map[string]float64)
	coins := []string{"bch", "btc", "eos", "eth", "prdx", "usdt", "xrp"}
	for _, coin := range coins {
		bMap[coin] = 139331.8883
	}

	Convey("it should initiate all user accounts balances into fms", t, func() {
		userRows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email", "role", "account_type", "status"}).
			AddRow(1, "user1", "shah1", "user1@gmail.com", model.Admin, "private", model.UserStatusActive).
			AddRow(2, "user2", "shah2", "user2@gmail.com", model.Member, "private", model.UserStatusActive).
			AddRow(3, "user3", "shah3", "user3@gmail.com", model.Member, "private", model.UserStatusActive)
		subAccountRows := sqlmock.NewRows([]string{"id", "user_id", "account_group", "market_type", "status"}).
			AddRow(1, 1, model.AccountGroupMain, model.MarketTypeMargin, model.SubAccountStatusActive).
			AddRow(2, 2, model.AccountGroupMain, model.MarketTypeMargin, model.SubAccountStatusActive).
			AddRow(3, 3, model.AccountGroupMain, model.MarketTypeMargin, model.SubAccountStatusActive)
		liabilitiesRows := sqlmock.NewRows([]string{"coin_symbol", "available", "locked", "in_orders"})
		for coin, price := range bMap {
			liabilitiesRows.AddRow(coin, price, 0.000, 0.000)
		}

		mock.
			ExpectQuery("SELECT * FROM \"users\" WHERE status = $1").
			WithArgs(model.UserStatusActive).
			WillReturnRows(userRows)
		mock.
			ExpectQuery("SELECT * FROM \"sub_accounts\" WHERE status = $1").
			WithArgs(model.SubAccountStatusActive).
			WillReturnRows(subAccountRows)
		for i := 1; i <= 3; i++ {
			mock.
				ExpectQuery("SELECT * FROM get_balances($1, $2)").
				WithArgs(i, 0).
				WillReturnRows(liabilitiesRows)
			mock.
				ExpectQuery("SELECT * FROM get_balances($1, $2)").
				WithArgs(i, i).
				WillReturnRows(liabilitiesRows)
		}
		err := fm.InitAccounts()
		So(err, ShouldBeNil)

		balances, err := fm.GetAccountBalances(1, 0)
		So(err, ShouldBeNil)
		So(balances, ShouldResemble, fm.users[1].accounts[0])

		balances, err = fm.GetAccountBalances(1, 1)
		So(err, ShouldBeNil)
		So(balances, ShouldResemble, fm.users[1].accounts[1])
	})

	Convey("return error on getting balances from fms with wrong userID", t, func() {
		_, err := fm.GetAccountBalances(4, 0)
		So(err, ShouldResemble, errors.New("unable to find the user balances"))
	})

	Convey("return error on getting balances from fms with wrong subAccount", t, func() {
		_, err := fm.GetAccountBalances(1, 4)
		So(err, ShouldResemble, errors.New("unable to find the user balances account"))
	})
}
