package fms

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ericlagergren/decimal"
	postgres2 "github.com/ericlagergren/decimal/sql/postgres"
	"github.com/rs/zerolog/log"
	. "github.com/smartystreets/goconvey/convey"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"math/rand"
	"testing"
	"time"
)

var zero = conv.NewDecimalWithPrecision()

func setupDB() (*gorm.DB, sqlmock.Sqlmock) {
	logger := log.With().Str("test", "OMS").Str("method", "setupDB").Logger()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		logger.Fatal().Msgf("can't create sqlmock: %s", err)
	}

	dialector := postgres.New(postgres.Config{
		DSN:                  "postgres-mock",
		DriverName:           "postgres",
		Conn:                 db,
		PreferSimpleProtocol: true,
	})

	gormDB, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		logger.Fatal().Msgf("can't open gorm connection: %s", err)
	}

	return gormDB, mock
}

func setupRepo() (*queries.Repo, sqlmock.Sqlmock) {
	db, mock := setupDB()
	return &queries.Repo{
		Conn:            db,
		ConnReader:      db,
		ConnReaderAdmin: db,
	}, mock
}

func TestCheckNaNs(t *testing.T) {
	amount := conv.NewDecimalWithPrecision()

	Convey("it should return nil for non NaN values", t, func() {
		amount.SetString(conv.FromUnits(1000, 8))
		err := checkNaNs(amount)

		So(err, ShouldBeNil)
	})

	Convey("should return error on NaN for decimal", t, func() {
		expected := errors.New("amount is NaN")
		amount.SetNaN(true)
		err := checkNaNs(amount)

		So(err, ShouldResemble, expected)
	})
}

func getTestOrderWithPrice(orderID uint64, marketID string, ownerID uint64, price *decimal.Big, subAccount uint64) *model.Order {
	order := getTestOrder(orderID, marketID, ownerID, subAccount)
	order.Price.V = price
	return order
}

// getTestOrder create returns a dummy order instance
func getTestOrder(orderID uint64, marketID string, ownerID uint64, subAccount uint64) *model.Order {
	price := conv.NewDecimalWithPrecision()
	price.SetString(conv.FromUnits(1000, 8))
	var parentOrderId uint64 = 0
	orderTYpe := model.OrderType_Market
	stopPrice := model.TrailingStopPriceType_Absolute
	order := model.NewOrder(
		ownerID,
		marketID,
		model.MarketSide_Buy,
		model.OrderType_Market,
		model.OrderStop_None,
		price,
		price,
		price,
		price,
		price,
		price,
		price,
		price,
		price,
		price,
		price,
		price,
		price,
		&parentOrderId,
		&orderTYpe,
		0,
		model.UIType_Api,
		"0",
		price,
		price,
		&stopPrice,
	)
	order.ID = orderID
	order.SubAccount = subAccount
	return order
}

func getTestMarket(marketId string) *model.Market {
	big := conv.NewDecimalWithPrecision()
	return model.NewMarket(
		marketId,
		"Test",
		"btc",
		"usdt",
		model.MarketStatusActive,
		8, 8, 8, 8,
		big,
		big,
		big,
		big,
		big,
	)
}

func getTestTrade(ID uint64, marketID string, bidOwnerID, askOwnerID uint64, side model.MarketSide) *model.Trade {
	timestamp := time.Now().Unix()
	price := conv.NewDecimalWithPrecision()
	price.SetString(conv.FromUnits(1000, 8))
	seqID := rand.Uint64()
	eventSeqID := rand.Uint64()
	askID := rand.Uint64()
	bidID := rand.Uint64()
	trade := model.NewTrade(seqID, marketID, eventSeqID, side, price, price, price, askID, bidID, askOwnerID, bidOwnerID, timestamp)
	trade.ID = ID
	return trade
}

func InitTestFE(ctx context.Context) *FundsEngine {
	r, mock := setupRepo()
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
			liabilitiesRows.AddRow(coin, price, &postgres2.Decimal{V: zero}, &postgres2.Decimal{V: zero})
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

func getBigDecimalWithPrecision(amount uint64, precision uint8) *decimal.Big {
	result := conv.NewDecimalWithPrecision()
	result.SetString(conv.FromUnits(amount, precision))
	return result
}
