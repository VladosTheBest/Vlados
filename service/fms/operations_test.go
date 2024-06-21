package fms

import (
	"context"
	"errors"
	"fmt"
	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	. "github.com/smartystreets/goconvey/convey"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"testing"
)

var (
	userID                 = uint64(1)
	subAccountID           = uint64(0)
	amount                 = conv.NewDecimalWithPrecision()
	fee, _                 = conv.NewDecimalWithPrecision().SetString(conv.FromUnits(1, 8))
	ErrInvalidUserBalances = errors.New("unable to find the user balances")
	ErrInvalidSubAccount   = errors.New("unable to find the user balances account")
	ErrInvalidCoin         = errors.New("coin not found")
	ErrNaNAmount           = errors.New("amount is NaN")
	ErrInsufficientFund    = errors.New("Insufficient funds required for withdrawal")
	ErrInvalidLockedFunds  = errors.New("Invalid amount locked")
)

func TestFundsEngine_DoNewDeposit(t *testing.T) {
	const coin = "bch"
	ctx := context.TODO()
	fm := InitTestFE(ctx)
	amount := conv.NewDecimalWithPrecision()
	amount.SetString(conv.FromUnits(10000000000, 8))

	Convey("It should deposit specified amount into the user balances", t, func() {
		accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		balance, err := accountBalances.GetLockedBalanceForCoin(coin)
		So(err, ShouldBeNil)

		err = fm.DoNewDeposit(userId, subAccountId, coin, amount)
		So(err, ShouldBeNil)
		So(balance.String(), ShouldEqual, amount.String())
	})

	Convey("It should return err on NaN amount", t, func() {
		a := conv.NewDecimalWithPrecision().SetNaN(true)
		err := fm.DoNewDeposit(userID, subAccountID, coin, a)

		So(err.Error(), ShouldEqual, ErrInvalidAmount.Error())
	})

	Convey("It should return err on userId which is not in FundsEngine users list", t, func() {
		err := fm.DoNewDeposit(8, subAccountID, coin, amount)
		So(err.Error(), ShouldEqual, ErrInvalidUserBalance.Error())
	})

	Convey("It should return err on subAccountId which is not in a users subAccounts list", t, func() {
		err := fm.DoNewDeposit(userID, 4, coin, amount)
		So(err.Error(), ShouldEqual, ErrInvalidUserBalanceAccount.Error())
	})

	Convey("It should return err on coin which is not in FundsEngine coins list", t, func() {
		err := fm.DoNewDeposit(userID, subAccountID, "sol", amount)
		So(err.Error(), ShouldEqual, ErrInvalidCoinSymbol.Error())
	})
}

func TestFundsEngine_DoNewManualWithdraw(t *testing.T) {
	const coin = "eth"
	ctx := context.TODO()
	fm := InitTestFE(ctx)
	amount.SetString(conv.FromUnits(10000000000, 8))

	Convey("it should add the specified amount to user's locked and sub it from the available", t, func() {
		accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		availableBalance, err := accountBalances.GetAvailableBalanceForCoin(coin)
		So(err, ShouldBeNil)
		lockedBalance, err := accountBalances.GetLockedBalanceForCoin(coin)
		So(err, ShouldBeNil)
		err = depositFunds(fm, userId, subAccountId, coin, amount)
		So(err, ShouldBeNil)

		err = fm.DoNewManualWithdraw(userId, subAccountId, coin, amount)
		So(err, ShouldBeNil)
		So(availableBalance.String(), ShouldEqual, "0E-8")
		So(lockedBalance.String(), ShouldEqual, amount.String())
	})

	Convey("It should return err on NaN amount", t, func() {
		a := conv.NewDecimalWithPrecision().SetNaN(true)
		err := fm.DoNewManualWithdraw(userID, subAccountID, coin, a)

		So(err.Error(), ShouldEqual, ErrInvalidAmount.Error())
	})

	Convey("It should return err on user not in FundsEngine users list", t, func() {
		err := fm.DoNewManualWithdraw(8, subAccountID, coin, amount)
		So(err.Error(), ShouldEqual, ErrInvalidUserBalance.Error())
	})

	Convey("It should return err on subAccount not in user's subAccount list in FundsEngine", t, func() {
		err := fm.DoNewManualWithdraw(userID, 4, coin, amount)
		So(err.Error(), ShouldEqual, ErrInvalidUserBalanceAccount.Error())
	})

	Convey("It should return err on coin not in FundsEngine coins list", t, func() {
		err := fm.DoNewManualWithdraw(userID, subAccountID, "sol", amount)
		So(err.Error(), ShouldEqual, ErrInvalidCoinSymbol.Error())
	})
}

func TestFundsEngine_DoConfirmDeposit(t *testing.T) {
	const coin = "prdx"
	ctx := context.TODO()
	fm := InitTestFE(ctx)
	amount.SetString(conv.FromUnits(10000000000, 8))

	Convey("It should subtract a specified amount from locked and add it to user's available balances in FundsEngine", t, func() {
		accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		availableBalance, err := accountBalances.GetAvailableBalanceForCoin(coin)
		So(err, ShouldBeNil)
		err = fm.DoNewDeposit(userId, subAccountId, coin, amount)
		So(err, ShouldBeNil)

		err = fm.DoConfirmDeposit(userId, subAccountId, coin, amount)
		So(err, ShouldBeNil)
		So(availableBalance.String(), ShouldEqual, amount.String())
	})

	Convey("It should return error on NaN amount", t, func() {
		a := conv.NewDecimalWithPrecision().SetNaN(true)
		err := fm.DoConfirmDeposit(userID, subAccountID, coin, a)

		So(err, ShouldResemble, ErrNaNAmount)
	})

	Convey("It should return error on user not in FundsEngine users list", t, func() {
		err := fm.DoConfirmDeposit(9, subAccountID, coin, amount)
		So(err.Error(), ShouldEqual, ErrInvalidUserBalances.Error())
	})

	Convey("It should return error on subAccountId not in user's accounts list", t, func() {
		err := fm.DoConfirmDeposit(userID, 9, coin, amount)
		So(err.Error(), ShouldEqual, ErrInvalidSubAccount.Error())
	})

	Convey("It should return error on coin not in FundsEngine coins list", t, func() {
		err := fm.DoConfirmDeposit(userID, subAccountID, "sol", amount)
		So(err.Error(), ShouldEqual, ErrInvalidCoin.Error())
	})
}

func TestFundsEngine_DoConfirmWithdraw(t *testing.T) {
	const coin = "eos"
	ctx := context.TODO()
	fm := InitTestFE(ctx)
	amount.SetString(conv.FromUnits(10000000000, 8))

	Convey("It should deduct a specified amount form user's locked balance", t, func() {
		accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		lockedBalance, err := accountBalances.GetLockedBalanceForCoin(coin)
		So(err, ShouldBeNil)
		err = depositFunds(fm, userId, subAccountId, coin, amount)
		So(err, ShouldBeNil)
		err = fm.DoNewManualWithdraw(userId, subAccountId, coin, amount)
		So(err, ShouldBeNil)

		err = fm.DoConfirmWithdraw(userId, subAccountId, coin, amount)
		So(err, ShouldBeNil)
		So(lockedBalance.String(), ShouldEqual, "0E-8")
	})

	Convey("It should return error on NaN amount", t, func() {
		a := conv.NewDecimalWithPrecision().SetNaN(true)
		err := fm.DoConfirmWithdraw(userID, subAccountID, coin, a)

		So(err.Error(), ShouldEqual, ErrNaNAmount.Error())
	})

	Convey("It should return error on userId not in list of users in FundsEngine", t, func() {
		err := fm.DoConfirmWithdraw(8, subAccountID, coin, amount)
		So(err.Error(), ShouldEqual, ErrInvalidUserBalances.Error())
	})

	Convey("It should return error on subAccountId not in list of user's subAccounts list", t, func() {
		err := fm.DoConfirmWithdraw(userID, 8, coin, amount)
		So(err.Error(), ShouldEqual, ErrInvalidSubAccount.Error())
	})

	Convey("It should return error on coin not in list of coins", t, func() {
		err := fm.DoConfirmWithdraw(userID, subAccountID, "sol", amount)
		So(err.Error(), ShouldEqual, ErrInvalidCoin.Error())
	})
}

func TestFundsEngine_DoNewWithdraw(t *testing.T) {
	const coin = "xrp"
	ctx := context.TODO()
	fm := InitTestFE(ctx)
	amount.SetString(conv.FromUnits(10000000000, 8))

	Convey("It should deduct a specified amount including the fee of the withdraw from user's locked balance", t, func() {
		totalAmount := conv.NewDecimalWithPrecision().Add(amount, fee)
		accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		lockedBalance, err := accountBalances.GetLockedBalanceForCoin(coin)
		So(err, ShouldBeNil)
		err = fm.DoNewDeposit(userId, subAccountId, coin, totalAmount)

		err = fm.DoNewWithdraw(userID, subAccountID, coin, amount, fee)
		So(err, ShouldBeNil)
		So(lockedBalance.String(), ShouldEqual, "0E-8")
	})

	Convey("It should return error if userID is not in FundsEngine users list", t, func() {
		err := fm.DoNewWithdraw(9, subAccountID, coin, amount, fee)
		So(err.Error(), ShouldEqual, ErrInvalidUserBalances.Error())
	})

	Convey("It should return error if subAccountID is not in user's accountsID list", t, func() {
		err := fm.DoNewWithdraw(userID, 9, coin, amount, fee)
		So(err.Error(), ShouldEqual, ErrInvalidSubAccount.Error())
	})

	Convey("It should return error on coin not in list of coins in FundsEngine", t, func() {
		err := fm.DoNewWithdraw(userID, subAccountID, "sol", amount, fee)
		So(err.Error(), ShouldEqual, ErrInvalidCoin.Error())
	})

	Convey("It should return if there are insufficient funds in users locked balance", t, func() {
		accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		So(err, ShouldBeNil)
		err = accountBalances.Deposit(coin, amount)
		So(err, ShouldBeNil)
		err = accountBalances.Lock(coin, amount, false)

		err = fm.DoNewWithdraw(userID, subAccountID, coin, amount, fee)
		So(err.Error(), ShouldEqual, ErrInsufficientFunds.Error())
	})
}

func TestFundsEngine_DoRevertWithdraw(t *testing.T) {
	const coin = "bch"
	ctx := context.TODO()
	fm := InitTestFE(ctx)
	amount.SetString(conv.FromUnits(10000000000, 8))

	Convey("It should add the deducted specified to locked balances", t, func() {
		totalAmount := conv.NewDecimalWithPrecision().Add(amount, fee)
		accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		lockedBalance, err := accountBalances.GetLockedBalanceForCoin(coin)
		So(err, ShouldBeNil)

		err = fm.DoRevertWithdraw(userID, subAccountID, coin, amount, fee)
		So(err, ShouldBeNil)
		So(lockedBalance.String(), ShouldEqual, totalAmount.String())
	})

	Convey("It should return error for userID not in list of users in FundsEngine", t, func() {
		err := fm.DoRevertWithdraw(9, subAccountID, coin, amount, fee)
		So(err.Error(), ShouldEqual, ErrInvalidUserBalances.Error())
	})

	Convey("It should return error for subAccountID not in user's subAccounts list", t, func() {
		err := fm.DoRevertWithdraw(userID, 9, coin, amount, fee)
		So(err.Error(), ShouldEqual, ErrInvalidSubAccount.Error())
	})

	Convey("It should return error for coin not in coins list for a user in FundsEngine", t, func() {
		err := fm.DoRevertWithdraw(userID, subAccountID, "sol", amount, fee)
		So(err.Error(), ShouldEqual, ErrInvalidCoin.Error())
	})
}

func TestFundsEngine_DoLockNewOrder(t *testing.T) {
	const coin = "bch"
	const marketId = "bchusdt"
	ctx := context.TODO()
	fm := InitTestFE(ctx)
	order := getTestOrder(1, marketId, userId, subAccountId)
	Zero := conv.NewDecimalWithPrecision()
	market := getTestMarket(marketId)
	locked, _ := conv.NewDecimalWithPrecision().SetString(order.LockedFunds.V.String())

	Convey("It should add a specified amount in locked, Inorders and sub it from available", t, func() {
		accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		lockedBalance, err := accountBalances.GetLockedBalanceForCoin(coin)
		So(err, ShouldBeNil)
		availableBalance, err := accountBalances.GetAvailableBalanceForCoin(coin)
		So(err, ShouldBeNil)
		err = depositFunds(fm, userId, subAccountId, coin, locked)
		So(err, ShouldBeNil)

		err = fm.DoLockNewOrder(order.OwnerID, order.SubAccount, coin, locked, func(available *decimal.Big) error {
			if err := order.IsValidAgainstBalance(available, market); err != nil {
				return err
			}
			if locked.Cmp(Zero) == 0 {
				return ErrInvalidLockedFunds
			}
			if available.Cmp(locked) < 0 {
				return ErrInsufficientFund
			}
			return nil
		})
		So(err, ShouldBeNil)
		So(lockedBalance.String(), ShouldEqual, locked.String())
		So(availableBalance.String(), ShouldEqual, "0E-8")
		So(accountBalances.balances[coin].InOrders.String(), ShouldEqual, locked.String())
	})

	Convey("It should return error on NaN amount", t, func() {
		a := conv.NewDecimalWithPrecision().SetNaN(true)
		err := fm.DoLockNewOrder(order.OwnerID, order.SubAccount, coin, a, func(available *decimal.Big) error {
			if err := order.IsValidAgainstBalance(available, market); err != nil {
				return err
			}
			if locked.Cmp(Zero) == 0 {
				return ErrInvalidLockedFunds
			}
			if available.Cmp(locked) < 0 {
				return ErrInsufficientFund
			}
			return nil
		})

		So(err.Error(), ShouldEqual, ErrInvalidAmount.Error())
	})

	Convey("It should return error if the locked balance is 0", t, func() {
		err := depositFunds(fm, userId, subAccountId, coin, locked)
		So(err, ShouldBeNil)

		a := conv.NewDecimalWithPrecision()
		err = fm.DoLockNewOrder(order.OwnerID, order.SubAccount, coin, a, func(available *decimal.Big) error {
			if err := order.IsValidAgainstBalance(available, market); err != nil {
				return err
			}
			if a.Cmp(Zero) == 0 {
				return ErrInvalidLockedFunds
			}
			if available.Cmp(locked) < 0 {
				return ErrInsufficientFund
			}
			return nil
		})

		So(err.Error(), ShouldEqual, ErrInvalidLockedFunds.Error())
	})

	Convey("It should return error on if userID is not in the FundsEngine", t, func() {
		err := fm.DoLockNewOrder(8, order.SubAccount, coin, locked, func(available *decimal.Big) error {
			if err := order.IsValidAgainstBalance(available, market); err != nil {
				return err
			}
			if locked.Cmp(Zero) == 0 {
				return ErrInvalidLockedFunds
			}
			if available.Cmp(locked) < 0 {
				return ErrInsufficientFund
			}
			return nil
		})

		So(err.Error(), ShouldEqual, ErrInvalidUserBalances.Error())
	})

	Convey("It should return error on subAccount not in users subAccounts in FundsEngine", t, func() {
		err := fm.DoLockNewOrder(order.OwnerID, 9, coin, locked, func(available *decimal.Big) error {
			if err := order.IsValidAgainstBalance(available, market); err != nil {
				return err
			}
			if locked.Cmp(Zero) == 0 {
				return ErrInvalidLockedFunds
			}
			if available.Cmp(locked) < 0 {
				return ErrInsufficientFund
			}
			return nil
		})

		So(err.Error(), ShouldEqual, ErrInvalidSubAccount.Error())
	})

	Convey("It should return error on coin symbol not in FundsEngine", t, func() {
		err := fm.DoLockNewOrder(order.OwnerID, order.SubAccount, "sol", locked, func(available *decimal.Big) error {
			if err := order.IsValidAgainstBalance(available, market); err != nil {
				return err
			}
			if locked.Cmp(Zero) == 0 {
				return ErrInvalidLockedFunds
			}
			if available.Cmp(locked) < 0 {
				return ErrInsufficientFund
			}
			return nil
		})

		So(err.Error(), ShouldEqual, ErrInvalidCoin.Error())
	})
}

func TestFundsEngine_DoTrade(t *testing.T) {
	const marketId = "btcusdt"
	ctx := context.TODO()
	fm := InitTestFE(ctx)
	amount.SetString(conv.FromUnits(666668, 8))
	bidFee, _ := conv.NewDecimalWithPrecision().SetString(conv.FromUnits(1, 8))
	conv.RoundToPrecision(bidFee)
	askFee, _ := conv.NewDecimalWithPrecision().SetString(conv.FromUnits(1, 8))
	conv.RoundToPrecision(askFee)
	amount.SetString(conv.FromUnits(1000, 8))
	conv.RoundToPrecision(amount)
	quoteVolume, _ := conv.NewDecimalWithPrecision().SetString(conv.FromUnits(1000, 8))
	conv.RoundToPrecision(quoteVolume)
	marketCoin := "btc"
	quoteCoin := "usdt"
	bidCreditAmount := conv.NewDecimalWithPrecision().Sub(amount, bidFee)
	askCreditAmount := conv.NewDecimalWithPrecision().Sub(quoteVolume, askFee)
	conv.RoundToPrecision(bidCreditAmount)
	conv.RoundToPrecision(askCreditAmount)
	market := getTestMarket(marketId)

	Convey("It should do a self trade", t, func() {
		trade := getTestTrade(1, marketId, userId, userId, model.MarketSide_Buy)
		askOrder := getTestOrder(1, marketId, userId, subAccountId)
		askOrder.Side = model.MarketSide_Sell
		bidOrder := getTestOrder(2, marketId, userId, subAccountId)
		bidOrder.Side = model.MarketSide_Buy
		//askOrderLockedFunds, bidOrderLockedFunds := conv.NewDecimalWithPrecision().Copy(askOrder.LockedFunds.V), conv.NewDecimalWithPrecision().Copy(bidOrder.LockedFunds.V)
		//askOrderUsedFunds, bidOrderUsedFunds := conv.NewDecimalWithPrecision().Copy(askOrder.UsedFunds.V), conv.NewDecimalWithPrecision().Copy(bidOrder.UsedFunds.V)

		accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		err = accountBalances.Deposit(marketCoin, amount)
		So(err, ShouldBeNil)
		err = accountBalances.Deposit(quoteCoin, quoteVolume)
		So(err, ShouldBeNil)
		err = accountBalances.Lock(marketCoin, amount, true)
		So(err, ShouldBeNil)
		err = accountBalances.Lock(quoteCoin, quoteVolume, true)
		askAvailableBalance, err := accountBalances.GetAvailableBalanceForCoin(quoteCoin)
		So(err, ShouldBeNil)
		askLockedBalance, err := accountBalances.GetLockedBalanceForCoin(marketCoin)
		So(err, ShouldBeNil)
		askInOrdersBalance := accountBalances.balances[marketCoin].InOrders
		bidAvailableBalance, err := accountBalances.GetAvailableBalanceForCoin(marketCoin)
		So(err, ShouldBeNil)
		bidLockedBalance, err := accountBalances.GetLockedBalanceForCoin(quoteCoin)
		So(err, ShouldBeNil)
		bidInOrdersBalance := accountBalances.balances[quoteCoin].InOrders

		err = fm.DoTrade(trade, askOrder, bidOrder, market, askCreditAmount, bidCreditAmount)

		So(err, ShouldBeNil)
		So(askAvailableBalance.String(), ShouldEqual, askCreditAmount.String())
		So(askLockedBalance.String(), ShouldEqual, "0E-8")
		So(askInOrdersBalance.String(), ShouldEqual, "0E-8")
		So(bidAvailableBalance.String(), ShouldEqual, bidCreditAmount.String())
		So(bidLockedBalance.String(), ShouldEqual, "0E-8")
		So(bidInOrdersBalance.String(), ShouldEqual, "0E-8")
	})

	Convey("It should do a trade", t, func() {
		userId2 := uint64(2)
		trade := getTestTrade(1, marketId, userId2, userId, model.MarketSide_Buy)
		askOrder := getTestOrder(1, marketId, userId, subAccountId)
		askOrder.Side = model.MarketSide_Sell
		bidOrder := getTestOrder(2, marketId, userId2, subAccountId)
		bidOrder.Side = model.MarketSide_Buy
		//askOrderLockedFunds, bidOrderLockedFunds := conv.NewDecimalWithPrecision().Copy(askOrder.LockedFunds.V), conv.NewDecimalWithPrecision().Copy(bidOrder.LockedFunds.V)
		//askOrderUsedFunds, bidOrderUsedFunds := conv.NewDecimalWithPrecision().Copy(askOrder.UsedFunds.V), conv.NewDecimalWithPrecision().Copy(bidOrder.UsedFunds.V)

		askAccountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		bidAccountBalances, err := fm.GetAccountBalances(userId2, subAccountId)
		So(err, ShouldBeNil)
		err = askAccountBalances.Deposit(marketCoin, amount)
		So(err, ShouldBeNil)
		err = bidAccountBalances.Deposit(quoteCoin, quoteVolume)
		So(err, ShouldBeNil)
		err = askAccountBalances.Lock(marketCoin, amount, true)
		So(err, ShouldBeNil)
		err = bidAccountBalances.Lock(quoteCoin, quoteVolume, true)
		So(err, ShouldBeNil)
		askAvailableBalance, err := askAccountBalances.GetAvailableBalanceForCoin(quoteCoin)
		So(err, ShouldBeNil)
		askLockedBalance, err := askAccountBalances.GetLockedBalanceForCoin(marketCoin)
		So(err, ShouldBeNil)
		askInOrdersBalance := askAccountBalances.balances[marketCoin].InOrders
		bidAvailableBalance, err := bidAccountBalances.GetAvailableBalanceForCoin(marketCoin)
		So(err, ShouldBeNil)
		bidLockedBalance, err := bidAccountBalances.GetLockedBalanceForCoin(quoteCoin)
		So(err, ShouldBeNil)
		bidInOrdersBalance := bidAccountBalances.balances[quoteCoin].InOrders

		err = fm.DoTrade(trade, askOrder, bidOrder, market, askCreditAmount, bidCreditAmount)
		So(err, ShouldBeNil)
		So(askAvailableBalance.String(), ShouldEqual, "0.00001998")
		So(askLockedBalance.String(), ShouldEqual, "0E-8")
		So(askInOrdersBalance.String(), ShouldEqual, "0E-8")
		So(bidAvailableBalance.String(), ShouldEqual, bidCreditAmount.String())
		So(bidLockedBalance.String(), ShouldEqual, "0E-8")
		So(bidInOrdersBalance.String(), ShouldEqual, "0E-8")
	})

	Convey("orders with status filled and locked funds greater then used funds should get added to user's available balance", t, func() {
		userId2 := uint64(2)
		trade := getTestTrade(1, marketId, userId2, userId, model.MarketSide_Buy)
		askOrder := getTestOrder(1, marketId, userId, subAccountId)
		askOrder.Side = model.MarketSide_Sell
		askOrder.Status = model.OrderStatus_Filled
		askOrder.LockedFunds = &postgres.Decimal{V: decimal.New(10000, 0)}
		bidOrder := getTestOrder(2, marketId, userId2, subAccountId)
		bidOrder.Side = model.MarketSide_Buy
		bidOrder.Status = model.OrderStatus_Filled
		bidOrder.LockedFunds = &postgres.Decimal{V: decimal.New(10000, 0)}
		askOrderLockedFunds, bidOrderLockedFunds := conv.NewDecimalWithPrecision().Copy(askOrder.LockedFunds.V), conv.NewDecimalWithPrecision().Copy(bidOrder.LockedFunds.V)
		askOrderUsedFunds, bidOrderUsedFunds := conv.NewDecimalWithPrecision().Copy(askOrder.UsedFunds.V), conv.NewDecimalWithPrecision().Copy(bidOrder.UsedFunds.V)
		askUnused := conv.NewDecimalWithPrecision().Sub(askOrderLockedFunds, askOrderUsedFunds)
		conv.RoundToPrecision(askUnused)
		bidUnused := conv.NewDecimalWithPrecision().Sub(bidOrderLockedFunds, bidOrderUsedFunds)
		conv.RoundToPrecision(bidUnused)

		askAccountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		bidAccountBalances, err := fm.GetAccountBalances(userId2, subAccountId)
		askAvailableExpected := conv.NewDecimalWithPrecision().Add(askAccountBalances.balances[marketCoin].Available, askUnused)
		So(err, ShouldBeNil)
		err = askAccountBalances.Deposit(marketCoin, amount)
		So(err, ShouldBeNil)
		err = bidAccountBalances.Deposit(quoteCoin, quoteVolume)
		So(err, ShouldBeNil)
		err = askAccountBalances.Deposit(marketCoin, askUnused)
		So(err, ShouldBeNil)
		err = bidAccountBalances.Deposit(quoteCoin, bidUnused)
		So(err, ShouldBeNil)
		err = askAccountBalances.Lock(marketCoin, amount, true)
		So(err, ShouldBeNil)
		err = bidAccountBalances.Lock(quoteCoin, quoteVolume, true)
		So(err, ShouldBeNil)
		err = askAccountBalances.Lock(marketCoin, askUnused, true)
		So(err, ShouldBeNil)
		err = bidAccountBalances.Lock(quoteCoin, bidUnused, true)
		So(err, ShouldBeNil)
		askAvailableBalance, err := askAccountBalances.GetAvailableBalanceForCoin(marketCoin)
		So(err, ShouldBeNil)
		askLockedBalance, err := askAccountBalances.GetLockedBalanceForCoin(marketCoin)
		So(err, ShouldBeNil)
		askInOrdersBalance := askAccountBalances.balances[marketCoin].InOrders
		bidAvailableBalance, err := bidAccountBalances.GetAvailableBalanceForCoin(quoteCoin)
		So(err, ShouldBeNil)
		bidLockedBalance, err := bidAccountBalances.GetLockedBalanceForCoin(quoteCoin)
		So(err, ShouldBeNil)
		bidInOrdersBalance := bidAccountBalances.balances[quoteCoin].InOrders

		err = fm.DoTrade(trade, askOrder, bidOrder, market, askCreditAmount, bidCreditAmount)

		So(err, ShouldBeNil)
		So(askAvailableBalance.String(), ShouldEqual, askAvailableExpected.String())
		So(askLockedBalance.String(), ShouldEqual, "0E-8")
		So(askInOrdersBalance.String(), ShouldEqual, "0E-8")
		So(bidAvailableBalance.String(), ShouldEqual, bidUnused.String())
		So(bidLockedBalance.String(), ShouldEqual, "0E-8")
		So(bidInOrdersBalance.String(), ShouldEqual, "0E-8")
	})

	Convey("It should check for errors", t, func() {
		type testInput struct {
			askOwnerID, askSubAccountID, bidOwnerID, bidSubAccountID uint64
			quoteCoin, marketCoin                                    string
			amount, quoteVolume, askCreditAmount, bidCreditAmount    *decimal.Big
			askOrderStatus, bidOrderStatus                           model.OrderStatus
			askOrderLockedFunds, askOrderUsedFunds                   *decimal.Big
			bidOrderLockedFunds, bidOrderUsedFunds                   *decimal.Big
		}
		a := conv.NewDecimalWithPrecision().SetNaN(true)
		input := testInput{
			askOwnerID:          userId,
			askSubAccountID:     subAccountID,
			bidOwnerID:          2,
			bidSubAccountID:     subAccountID,
			quoteCoin:           quoteCoin,
			marketCoin:          marketCoin,
			amount:              amount,
			quoteVolume:         quoteVolume,
			askCreditAmount:     askCreditAmount,
			bidCreditAmount:     bidCreditAmount,
			askOrderStatus:      model.OrderStatus_Pending,
			bidOrderStatus:      model.OrderStatus_Pending,
			askOrderLockedFunds: amount,
			askOrderUsedFunds:   amount,
			bidOrderLockedFunds: amount,
			bidOrderUsedFunds:   amount,
		}
		input1 := input
		input1.quoteVolume = a
		input2 := input
		input2.amount = a
		input3 := input
		input3.askCreditAmount = a
		input4 := input
		input4.bidCreditAmount = a
		input5 := input
		input5.askOwnerID = 9
		input6 := input
		input6.askSubAccountID = 4
		input7 := input
		input7.bidOwnerID = 8
		input8 := input
		input8.bidSubAccountID = 4

		testCases := []struct {
			Input       testInput
			ExpectedErr error
		}{
			{
				Input:       input1,
				ExpectedErr: ErrNaNAmount,
			},
			{
				Input:       input2,
				ExpectedErr: ErrNaNAmount,
			},
			{
				Input:       input3,
				ExpectedErr: ErrNaNAmount,
			},
			{
				Input:       input4,
				ExpectedErr: ErrNaNAmount,
			},
			{
				Input:       input5,
				ExpectedErr: ErrInvalidUserBalances,
			},
			{
				Input:       input6,
				ExpectedErr: ErrInvalidSubAccount,
			},
			{
				Input:       input7,
				ExpectedErr: ErrInvalidUserBalances,
			},
			{
				Input:       input8,
				ExpectedErr: ErrInvalidSubAccount,
			},
		}

		for i, testCase := range testCases {
			Convey(fmt.Sprintf("%d it should return this error: %v", i, testCase.ExpectedErr), func() {
				input := testCase.Input
				trade := getTestTrade(1, marketId, input.bidOwnerID, input.askOwnerID, model.MarketSide_Buy)
				askOrder := getTestOrder(1, marketId, input.askOwnerID, input.askSubAccountID)
				bidOrder := getTestOrder(2, marketId, input.bidOwnerID, input.bidSubAccountID)

				trade.Volume.V = input.amount
				trade.QuoteVolume.V = input.quoteVolume
				trade.AskOwnerID = input.askOwnerID
				trade.BidOwnerID = input.bidOwnerID
				market.MarketCoinSymbol = input.marketCoin
				market.QuoteCoinSymbol = input.quoteCoin
				askOrder.LockedFunds.V = input.askOrderLockedFunds
				askOrder.UsedFunds.V = input.askOrderUsedFunds
				bidOrder.LockedFunds.V = input.bidOrderLockedFunds
				bidOrder.UsedFunds.V = input.bidOrderUsedFunds
				askOrder.SubAccount = input.askSubAccountID
				bidOrder.SubAccount = input.bidSubAccountID

				err := fm.DoTrade(trade, askOrder, bidOrder, market, input.askCreditAmount, input.bidCreditAmount)

				So(err.Error(), ShouldEqual, testCase.ExpectedErr.Error())
			})
		}
	})
}

func TestFundsEngine_DoCancelOrder(t *testing.T) {
	const coin = "prdx"
	ctx := context.TODO()
	fm := InitTestFE(ctx)
	amount.SetString(conv.FromUnits(77777, 8))

	Convey("It should cancel an order deduct amount from locked and inorders and add funds to available", t, func() {
		accountBalances, err := fm.GetAccountBalances(userId, subAccountId)
		So(err, ShouldBeNil)
		lockedBalance, err := accountBalances.GetLockedBalanceForCoin(coin)
		So(err, ShouldBeNil)
		availableBalance, err := accountBalances.GetAvailableBalanceForCoin(coin)
		So(err, ShouldBeNil)
		err = accountBalances.Deposit(coin, amount)
		So(err, ShouldBeNil)
		err = accountBalances.Lock(coin, amount, true)

		err = fm.DoCancelOrder(userID, subAccountID, coin, amount)
		So(err, ShouldBeNil)
		So(lockedBalance.String(), ShouldEqual, "0E-8")
		So(availableBalance.String(), ShouldEqual, amount.String())
		So(accountBalances.balances[coin].InOrders.String(), ShouldEqual, "0E-8")
	})

	Convey("It should return error on NaN amount", t, func() {
		a := conv.NewDecimalWithPrecision().SetNaN(true)

		err := fm.DoCancelOrder(userID, subAccountID, coin, a)
		So(err, ShouldResemble, ErrNaNAmount)
	})
}

func depositFunds(fm *FundsEngine, userID, subAccountID uint64, coin string, amount *decimal.Big) error {
	err := fm.DoNewDeposit(userID, subAccountID, coin, amount)
	if err != nil {
		return err
	}
	err = fm.DoConfirmDeposit(userID, subAccountID, coin, amount)
	if err != nil {
		return err
	}

	return nil
}
