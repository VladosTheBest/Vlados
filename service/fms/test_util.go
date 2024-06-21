package fms

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	postgres2 "github.com/ericlagergren/decimal/sql/postgres"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

var (
	Zero = conv.NewDecimalWithPrecision()
)

func GetTestFE(r *queries.Repo, mock sqlmock.Sqlmock, ctx context.Context) *FundsEngine {
	fm := Init(r, ctx)

	bMap := make(map[string]*postgres2.Decimal)
	coins := []string{"bch", "btc", "eos", "eth", "prdx", "usdt", "xrp"}
	for _, coin := range coins {
		a := conv.NewDecimalWithPrecision()
		bMap[coin] = &postgres2.Decimal{V: a}
	}

	userRows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email", "role", "account_type", "status"}).
		AddRow(1, "user1", "shah1", "user1@gmail.com", model.Admin, "private", model.UserStatusActive).
		AddRow(2, "user2", "shah2", "user2@gmail.com", model.Member, "private", model.UserStatusActive).
		AddRow(3, "user3", "shah3", "user3@gmail.com", model.Member, "private", model.UserStatusActive)
	subAccountRows := sqlmock.NewRows([]string{"id", "user_id", "account_group", "market_type", "status"}).
		AddRow(1, 1, model.AccountGroupMain, model.MarketTypeSpot, model.SubAccountStatusActive).
		AddRow(2, 2, model.AccountGroupMain, model.MarketTypeSpot, model.SubAccountStatusActive).
		AddRow(3, 3, model.AccountGroupMain, model.MarketTypeSpot, model.SubAccountStatusActive)

	mock.
		ExpectQuery("SELECT * FROM \"users\" WHERE status = $1").
		WithArgs(model.UserStatusActive).
		WillReturnRows(userRows)
	mock.
		ExpectQuery("SELECT * FROM \"sub_accounts\" WHERE status = $1").
		WithArgs(model.SubAccountStatusActive).
		WillReturnRows(subAccountRows)
	for i := 1; i <= 3; i++ {
		liabilitiesRows := sqlmock.NewRows([]string{"coin_symbol", "available", "locked", "in_orders"})
		for coin, price := range bMap {
			liabilitiesRows.AddRow(coin, price, &postgres2.Decimal{V: Zero}, &postgres2.Decimal{V: Zero})
		}
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
	if err != nil {
		log.Error().Err(err).
			Msg("Unable to initiate account balances")
	}

	return fm
}
