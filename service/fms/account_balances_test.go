package fms

import (
	"context"
	"errors"
	"github.com/ericlagergren/decimal"
	. "github.com/smartystreets/goconvey/convey"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"

	"testing"
)

var (
	coinNotFoundErr = errors.New("coin not found")
	balanceErr      = errors.New("unable to get balance")
	nanErr          = errors.New("amount is NaN")
	userId          = uint64(1)
	subAccountId    = uint64(0)
)

func TestAccountBalances_GetAll(t *testing.T) {
	ctx := context.TODO()
	fm := InitTestFE(ctx)

	testCases := []struct {
		inputCoin   string
		inputAmount *decimal.Big
	}{
		{
			"btc",
			getBigDecimalWithPrecision(1000000000, 8),
		},
		{
			"bch",
			getBigDecimalWithPrecision(2000000000, 8),
		},
		{
			"eth",
			getBigDecimalWithPrecision(3000000000, 8),
		},
	}
	for _, input := range testCases {
		Convey("It should return account balances", t, func() {
			accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
			So(err, ShouldBeNil)
			err = accountBalances.Deposit(input.inputCoin, input.inputAmount)
			So(err, ShouldBeNil)

			balances := accountBalances.GetAll()
			So(balances[input.inputCoin].Available.String(), ShouldEqual, input.inputAmount.String())
		})
	}
}

func TestAccountBalances_GetAvailableBalanceForCoin(t *testing.T) {
	ctx := context.TODO()
	fm := InitTestFE(ctx)

	testCases := []struct {
		inputCoin   string
		inputAmount *decimal.Big
	}{
		{
			"btc",
			getBigDecimalWithPrecision(1000000000, 8),
		},
		{
			"usdt",
			getBigDecimalWithPrecision(2000000000, 8),
		},
	}

	for _, input := range testCases {
		Convey("It should return available balances for a specific coin", t, func() {
			accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
			So(err, ShouldBeNil)
			err = accountBalances.Deposit(input.inputCoin, input.inputAmount)
			So(err, ShouldBeNil)

			balance, err := accountBalances.GetAvailableBalanceForCoin(input.inputCoin)
			So(err, ShouldBeNil)
			So(balance.String(), ShouldEqual, input.inputAmount.String())
		})
	}

	Convey("It should return err on invalid coin", t, func() {
		const coin = "btc"
		amount := getBigDecimalWithPrecision(1000000000, 8)
		accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		err = accountBalances.Deposit(coin, amount)
		So(err, ShouldBeNil)

		_, err = accountBalances.GetAvailableBalanceForCoin("sol")
		So(err.Error(), ShouldEqual, balanceErr.Error())
	})
}

func TestAccountBalances_GetLockedBalanceForCoin(t *testing.T) {
	ctx := context.TODO()
	fm := InitTestFE(ctx)

	testCases := []struct {
		inputCoin   string
		inputAmount *decimal.Big
	}{
		{
			"bch",
			getBigDecimalWithPrecision(1000000000, 8),
		},
		{
			"prdx",
			getBigDecimalWithPrecision(2000000000, 8),
		},
	}

	for _, input := range testCases {
		Convey("It should return locked balances for a specific coin", t, func() {
			accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
			So(err, ShouldBeNil)
			err = accountBalances.Deposit(input.inputCoin, input.inputAmount)
			So(err, ShouldBeNil)
			err = accountBalances.Lock(input.inputCoin, input.inputAmount, false)
			So(err, ShouldBeNil)

			balance, err := accountBalances.GetLockedBalanceForCoin(input.inputCoin)
			So(err, ShouldBeNil)
			So(balance.String(), ShouldEqual, input.inputAmount.String())
		})
	}

	Convey("It should return err on invalid coin", t, func() {
		const coin = "btc"
		amount := getBigDecimalWithPrecision(1000000000, 8)
		accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		err = accountBalances.Deposit(coin, amount)
		So(err, ShouldBeNil)
		err = accountBalances.Lock(coin, amount, false)
		So(err, ShouldBeNil)

		_, err = accountBalances.GetLockedBalanceForCoin("sol")
		So(err.Error(), ShouldEqual, balanceErr.Error())
	})
}

func TestAccountBalances_GetTotalBalanceForCoin(t *testing.T) {
	ctx := context.TODO()
	fm := InitTestFE(ctx)

	testCases := []struct {
		inputCoin      string
		inputAmount    *decimal.Big
		expectedAmount string
	}{
		{
			"bch",
			getBigDecimalWithPrecision(1000000000, 8),
			"20.00000000",
		},
		{
			"prdx",
			getBigDecimalWithPrecision(2000000000, 8),
			"40.00000000",
		},
	}

	for _, input := range testCases {
		Convey("It should return total balances for a specific coin", t, func() {
			accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
			So(err, ShouldBeNil)
			err = accountBalances.Deposit(input.inputCoin, input.inputAmount)
			So(err, ShouldBeNil)
			err = accountBalances.Deposit(input.inputCoin, input.inputAmount)
			So(err, ShouldBeNil)
			err = accountBalances.Lock(input.inputCoin, input.inputAmount, false)
			So(err, ShouldBeNil)

			balance, err := accountBalances.GetTotalBalanceForCoin(input.inputCoin)
			So(err, ShouldBeNil)
			So(balance.String(), ShouldEqual, input.expectedAmount)
		})
	}

	Convey("It should return err on invalid coin", t, func() {
		const coin = "btc"
		amount := getBigDecimalWithPrecision(1000000000, 8)
		accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		err = accountBalances.Deposit(coin, amount)
		So(err, ShouldBeNil)
		err = accountBalances.Deposit(coin, amount)
		So(err, ShouldBeNil)
		err = accountBalances.Lock(coin, amount, false)
		So(err, ShouldBeNil)

		_, err = accountBalances.GetTotalBalanceForCoin("sol")
		So(err.Error(), ShouldEqual, balanceErr.Error())
	})
}

func TestAccountBalances_Deposit(t *testing.T) {
	ctx := context.TODO()
	fm := InitTestFE(ctx)

	testCases := []struct {
		inputCoin   string
		inputAmount *decimal.Big
		expectedErr error
	}{
		{
			"sol",
			getBigDecimalWithPrecision(1000000000, 8),
			errors.New("coin not found"),
		},
		{
			"prdx",
			conv.NewDecimalWithPrecision().SetNaN(true),
			errors.New("amount is NaN"),
		},
	}

	for _, input := range testCases {
		Convey("It should return errors", t, func() {
			accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
			So(err, ShouldBeNil)
			err = accountBalances.Deposit(input.inputCoin, input.inputAmount)
			So(err.Error(), ShouldEqual, input.expectedErr.Error())
		})
	}
}

func TestAccountBalances_Withdrawal(t *testing.T) {
	ctx := context.TODO()
	fm := InitTestFE(ctx)

	testCases := []struct {
		inputCoin      string
		inputAmount    *decimal.Big
		expectedAmount string
	}{
		{
			"bch",
			getBigDecimalWithPrecision(1000000000, 8),
			"0E-8",
		},
		{
			"prdx",
			getBigDecimalWithPrecision(2000000000, 8),
			"0E-8",
		},
	}

	for _, input := range testCases {
		Convey("It should return deduct amount for a specific coin", t, func() {
			accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
			So(err, ShouldBeNil)
			err = accountBalances.Deposit(input.inputCoin, input.inputAmount)
			So(err, ShouldBeNil)

			err = accountBalances.Withdrawal(input.inputCoin, input.inputAmount)
			So(err, ShouldBeNil)
			balance, err := accountBalances.GetAvailableBalanceForCoin(input.inputCoin)
			So(err, ShouldBeNil)
			So(balance.String(), ShouldEqual, input.expectedAmount)
		})
	}

	errTestCases := []struct {
		inputCoin   string
		inputAmount *decimal.Big
		expectedErr error
	}{
		{
			"sol",
			getBigDecimalWithPrecision(1000000000, 8),
			errors.New("coin not found"),
		},
		{
			"prdx",
			conv.NewDecimalWithPrecision().SetNaN(true),
			errors.New("amount is NaN"),
		},
	}

	for _, input := range errTestCases {
		Convey("It should return error", t, func() {
			accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
			So(err, ShouldBeNil)

			err = accountBalances.Withdrawal(input.inputCoin, input.inputAmount)
			So(err.Error(), ShouldEqual, input.expectedErr.Error())
		})
	}
}

func TestAccountBalances_Lock(t *testing.T) {
	ctx := context.TODO()
	fm := InitTestFE(ctx)

	testCases := []struct {
		inputCoin   string
		inputAmount *decimal.Big
	}{
		{
			"xrp",
			getBigDecimalWithPrecision(1000000000, 8),
		},
		{
			"usdt",
			getBigDecimalWithPrecision(2000000000, 8),
		},
	}

	for _, input := range testCases {
		Convey("It should lock amount for a specific coin", t, func() {
			accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
			So(err, ShouldBeNil)
			err = accountBalances.Deposit(input.inputCoin, input.inputAmount)
			So(err, ShouldBeNil)

			err = accountBalances.Lock(input.inputCoin, input.inputAmount, false)
			So(err, ShouldBeNil)
			balance, err := accountBalances.GetLockedBalanceForCoin(input.inputCoin)
			So(err, ShouldBeNil)
			So(balance.String(), ShouldEqual, input.inputAmount.String())
		})
	}

	errTestCases := []struct {
		inputCoin   string
		inputAmount *decimal.Big
		expectedErr error
	}{
		{
			"sol",
			getBigDecimalWithPrecision(1000000000, 8),
			errors.New("coin not found"),
		},
		{
			"usdt",
			conv.NewDecimalWithPrecision().SetNaN(true),
			errors.New("amount is NaN"),
		},
	}

	for _, input := range errTestCases {
		Convey("It should check for errors", t, func() {
			accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
			So(err, ShouldBeNil)

			err = accountBalances.Lock(input.inputCoin, input.inputAmount, false)
			So(err.Error(), ShouldEqual, input.expectedErr.Error())
		})
	}
}

func TestAccountBalances_Unlock(t *testing.T) {
	ctx := context.TODO()
	fm := InitTestFE(ctx)

	testCases := []struct {
		inputCoin        string
		inputAmount      *decimal.Big
		expectedUnlocked string
	}{
		{
			"xrp",
			getBigDecimalWithPrecision(1000000000, 8),
			"0E-8",
		},
		{
			"usdt",
			getBigDecimalWithPrecision(2000000000, 8),
			"0E-8",
		},
	}

	for _, input := range testCases {
		Convey("It should unlock amount for a specific coin", t, func() {
			accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
			So(err, ShouldBeNil)
			err = accountBalances.Deposit(input.inputCoin, input.inputAmount)
			So(err, ShouldBeNil)
			err = accountBalances.Lock(input.inputCoin, input.inputAmount, true)
			So(err, ShouldBeNil)

			err = accountBalances.Unlock(input.inputCoin, input.inputAmount, true)
			balance, err := accountBalances.GetAvailableBalanceForCoin(input.inputCoin)
			So(err, ShouldBeNil)
			So(balance.String(), ShouldEqual, input.inputAmount.String())
			balance, err = accountBalances.GetLockedBalanceForCoin(input.inputCoin)
			So(err, ShouldBeNil)
			So(balance.String(), ShouldEqual, input.expectedUnlocked)
		})
	}

	errTestCases := []struct {
		inputCoin   string
		inputAmount *decimal.Big
		expectedErr error
	}{
		{
			"sol",
			getBigDecimalWithPrecision(1000000000, 8),
			errors.New("coin not found"),
		},
		{
			"usdt",
			conv.NewDecimalWithPrecision().SetNaN(true),
			errors.New("amount is NaN"),
		},
	}

	for _, input := range errTestCases {
		Convey("It should check for errors", t, func() {
			accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
			So(err, ShouldBeNil)

			err = accountBalances.Unlock(input.inputCoin, input.inputAmount, false)
			So(err.Error(), ShouldEqual, input.expectedErr.Error())
		})
	}
}

func TestAccountBalances_GetSubAccountID(t *testing.T) {
	ctx := context.TODO()
	fm := InitTestFE(ctx)

	Convey("it should return subAccountID", t, func() {
		accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)

		id := accountBalances.GetSubAccountID()
		So(id, ShouldEqual, 0)
	})
}
