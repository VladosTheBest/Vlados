package fms

import (
	"errors"

	"github.com/ericlagergren/decimal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

var ErrInsufficientFunds = errors.New("INSUFFICIENT_FUNDS")
var ErrInvalidAmount = errors.New("INVALID_AMOUNT")
var ErrInvalidUserBalance = errors.New("INVALID_USER_BALANCE")
var ErrInvalidUserBalanceAccount = errors.New("INVALID_USER_BALANCE_ACCOUNT")
var ErrInvalidCoinSymbol = errors.New("INVALID_COIN_SYMBOL")

type PrevalidateBalance func(available *decimal.Big) error

// Add locked funds for a new deposit made
func (fe *FundsEngine) DoNewDeposit(userID, subAccountID uint64, coin string, amount *decimal.Big) error {
	if conv.NewDecimalWithPrecision().CheckNaNs(amount, nil) {
		return ErrInvalidAmount
	}

	// wrap the operation in a lock
	fe.usersLock.Lock()
	defer fe.usersLock.Unlock()

	user, ok := fe.users[userID]
	if !ok {
		return ErrInvalidUserBalance
	}
	account, ok := user.accounts[subAccountID]
	if !ok {
		return ErrInvalidUserBalanceAccount
	}
	account.balancesLock.Lock()
	defer account.balancesLock.Unlock()
	balance, ok := account.balances[coin]
	if !ok {
		return ErrInvalidCoinSymbol
	}

	balance.Locked.Add(balance.Locked, amount)

	return nil
}

func (fe *FundsEngine) DoNewManualWithdraw(userID, subAccountID uint64, coin string, amount *decimal.Big) error {
	if conv.NewDecimalWithPrecision().CheckNaNs(amount, nil) {
		return ErrInvalidAmount
	}

	// wrap the operation in a lock
	fe.usersLock.Lock()
	defer fe.usersLock.Unlock()

	user, ok := fe.users[userID]
	if !ok {
		return ErrInvalidUserBalance
	}
	account, ok := user.accounts[subAccountID]
	if !ok {
		return ErrInvalidUserBalanceAccount
	}
	account.balancesLock.Lock()
	defer account.balancesLock.Unlock()
	balance, ok := account.balances[coin]
	if !ok {
		return ErrInvalidCoinSymbol
	}

	balance.Locked.Add(balance.Locked, amount)
	balance.Available.Sub(balance.Available, amount)

	return nil
}

// Move funds from locked funds for a deposit to the main account
func (fe *FundsEngine) DoConfirmDeposit(userID, subAccountID uint64, coin string, amount *decimal.Big) error {
	if conv.NewDecimalWithPrecision().CheckNaNs(amount, nil) {
		return errors.New("amount is NaN")
	}

	// wrap the operation in a lock
	fe.usersLock.Lock()
	defer fe.usersLock.Unlock()

	user, ok := fe.users[userID]
	if !ok {
		return errors.New("unable to find the user balances")
	}
	account, ok := user.accounts[subAccountID]
	if !ok {
		return errors.New("unable to find the user balances account")
	}
	account.balancesLock.Lock()
	defer account.balancesLock.Unlock()
	balance, ok := account.balances[coin]
	if !ok {
		return errors.New("coin not found")
	}

	balance.Locked.Sub(balance.Locked, amount)
	balance.Available.Add(balance.Available, amount)

	return nil
}

func (fe *FundsEngine) DoConfirmWithdraw(userID, subAccountID uint64, coin string, amount *decimal.Big) error {
	if conv.NewDecimalWithPrecision().CheckNaNs(amount, nil) {
		return errors.New("amount is NaN")
	}

	// wrap the operation in a lock
	fe.usersLock.Lock()
	defer fe.usersLock.Unlock()

	user, ok := fe.users[userID]
	if !ok {
		return errors.New("unable to find the user balances")
	}
	account, ok := user.accounts[subAccountID]
	if !ok {
		return errors.New("unable to find the user balances account")
	}
	account.balancesLock.Lock()
	defer account.balancesLock.Unlock()
	balance, ok := account.balances[coin]
	if !ok {
		return errors.New("coin not found")
	}

	balance.Locked.Sub(balance.Locked, amount)

	return nil
}

func (fe *FundsEngine) DoNewWithdraw(userID, subAccountID uint64, coin string, amount, fee *decimal.Big) error {
	// wrap the operation in a lock
	fe.usersLock.Lock()
	defer fe.usersLock.Unlock()

	user, ok := fe.users[userID]
	if !ok {
		return errors.New("unable to find the user balances")
	}
	account, ok := user.accounts[subAccountID]
	if !ok {
		return errors.New("unable to find the user balances account")
	}
	account.balancesLock.Lock()
	defer account.balancesLock.Unlock()
	balance, ok := account.balances[coin]
	if !ok {
		return errors.New("coin not found")
	}
	totalAmount := (&decimal.Big{}).Add(amount, fee)
	if totalAmount.Cmp(balance.Locked) == 1 {
		return ErrInsufficientFunds
	}

	balance.Locked.Sub(balance.Locked, totalAmount)

	return nil
}

func (fe *FundsEngine) DoRevertWithdraw(userID, subAccountID uint64, coin string, amount, fee *decimal.Big) error {
	// wrap the operation in a lock
	fe.usersLock.Lock()
	defer fe.usersLock.Unlock()

	user, ok := fe.users[userID]
	if !ok {
		return errors.New("unable to find the user balances")
	}
	account, ok := user.accounts[subAccountID]
	if !ok {
		return errors.New("unable to find the user balances account")
	}
	account.balancesLock.Lock()
	defer account.balancesLock.Unlock()
	balance, ok := account.balances[coin]
	if !ok {
		return errors.New("coin not found")
	}
	totalAmount := (&decimal.Big{}).Add(amount, fee)

	balance.Locked.Add(balance.Locked, totalAmount)

	return nil
}

// Lock funds for new order
func (fe *FundsEngine) DoLockNewOrder(userID, subAccountID uint64, coin string, amount *decimal.Big, callback PrevalidateBalance) error {
	if conv.NewDecimalWithPrecision().CheckNaNs(amount, nil) {
		return ErrInvalidAmount
	}

	return fe.Wrap(userID, subAccountID, coin, func(balance BalanceView) error {
		if err := callback(balance.Available); err != nil {
			return err
		}
		balance.Locked.Add(balance.Locked, amount)
		balance.InOrders.Add(balance.InOrders, amount)
		balance.Available.Sub(balance.Available, amount)
		return nil
	})
}

// @todo Execute a trade
func (fe *FundsEngine) DoTrade(
	trade *model.Trade,
	askOrder, bidOrder *model.Order,
	market *model.Market,
	askCreditAmount, bidCreditAmount *decimal.Big,
) error {
	amount := conv.CloneToPrecision(trade.Volume.V)
	quoteVolume := conv.CloneToPrecision(trade.QuoteVolume.V)
	bidOwnerID := trade.BidOwnerID
	askOwnerID := trade.AskOwnerID

	marketCoin := market.MarketCoinSymbol
	quoteCoin := market.QuoteCoinSymbol

	askOrderLockedFunds := conv.NewDecimalWithPrecision().Copy(askOrder.LockedFunds.V)
	askOrderUsedFunds := conv.NewDecimalWithPrecision().Copy(askOrder.UsedFunds.V)
	bidOrderLockedFunds := conv.NewDecimalWithPrecision().Copy(bidOrder.LockedFunds.V)
	bidOrderUsedFunds := conv.NewDecimalWithPrecision().Copy(bidOrder.UsedFunds.V)
	askSubAccountID := askOrder.SubAccount
	bidSubAccountID := bidOrder.SubAccount

	var err error

	// check for NaNs
	if err = checkNaNs(quoteVolume); err != nil {
		return err
	}
	if err = checkNaNs(amount); err != nil {
		return err
	}
	if err = checkNaNs(askCreditAmount); err != nil {
		return err
	}
	if err = checkNaNs(bidCreditAmount); err != nil {
		return err
	}

	// wrap the operation in a lock
	fe.usersLock.Lock()
	defer fe.usersLock.Unlock()

	askOwner, ok := fe.users[askOwnerID]
	if !ok {
		return errors.New("unable to find the user balances")
	}
	askAccount, ok := askOwner.accounts[askSubAccountID]
	if !ok {
		return errors.New("unable to find the user balances account")
	}
	askAccount.balancesLock.Lock()
	defer askAccount.balancesLock.Unlock()
	askAccountBalance := askAccount
	var bidAccountBalance *AccountBalances

	isSelfTrading := askOwnerID == bidOwnerID && askSubAccountID == bidSubAccountID
	if isSelfTrading {
		bidAccountBalance = askAccountBalance
	} else {
		bidOwner, ok := fe.users[bidOwnerID]
		if !ok {
			return errors.New("unable to find the user balances")
		}
		bidAccount, ok := bidOwner.accounts[bidSubAccountID]
		if !ok {
			return errors.New("unable to find the user balances account")
		}
		bidAccount.balancesLock.Lock()
		defer bidAccount.balancesLock.Unlock()
		bidAccountBalance = bidAccount
	}

	bidAccountBalance.balances[quoteCoin].Locked.Sub(bidAccountBalance.balances[quoteCoin].Locked, quoteVolume)
	bidAccountBalance.balances[quoteCoin].InOrders.Sub(bidAccountBalance.balances[quoteCoin].InOrders, quoteVolume)
	bidAccountBalance.balances[marketCoin].Available.Add(bidAccountBalance.balances[marketCoin].Available, bidCreditAmount)
	askAccountBalance.balances[marketCoin].Locked.Sub(askAccountBalance.balances[marketCoin].Locked, amount)
	askAccountBalance.balances[marketCoin].InOrders.Sub(askAccountBalance.balances[marketCoin].InOrders, amount)
	askAccountBalance.balances[quoteCoin].Available.Add(askAccountBalance.balances[quoteCoin].Available, askCreditAmount)

	// @todo CH: Verify is this is actually needed and if there is a case in which this can happen
	// Since the seller always specifies the exact amount of assets to sell and only those are locked then if a trade sells
	// all his tokens then there is nothing to return back to the user. The sell order should never complete without
	// selling all assets locked in that order
	if askOrder.Status == model.OrderStatus_Filled && askOrderLockedFunds.Cmp(askOrderUsedFunds) == 1 {
		// distribute funds from the trade to the seller
		unused := conv.NewDecimalWithPrecision().Sub(askOrderLockedFunds, askOrderUsedFunds)
		conv.RoundToPrecision(unused)
		if err = checkNaNs(unused); err != nil {
			return err
		}
		askAccountBalance.balances[marketCoin].Locked.Sub(askAccountBalance.balances[marketCoin].Locked, unused)
		askAccountBalance.balances[marketCoin].InOrders.Sub(askAccountBalance.balances[marketCoin].InOrders, unused)
		askAccountBalance.balances[marketCoin].Available.Add(askAccountBalance.balances[marketCoin].Available, unused)
	}

	// distribute unused funds from the filled orders to the owners
	if bidOrder.Status == model.OrderStatus_Filled && bidOrderLockedFunds.Cmp(bidOrderUsedFunds) == 1 {
		unused := conv.NewDecimalWithPrecision().Sub(bidOrderLockedFunds, bidOrderUsedFunds)
		conv.RoundToPrecision(unused)
		if err = checkNaNs(unused); err != nil {
			return err
		}
		bidAccountBalance.balances[quoteCoin].Locked.Sub(bidAccountBalance.balances[quoteCoin].Locked, unused)
		bidAccountBalance.balances[quoteCoin].InOrders.Sub(bidAccountBalance.balances[quoteCoin].InOrders, unused)
		bidAccountBalance.balances[quoteCoin].Available.Add(bidAccountBalance.balances[quoteCoin].Available, unused)
	}

	return nil
}

type WrappedCallable func(balance BalanceView) error

func (fe *FundsEngine) Wrap(userID, subAccountID uint64, coin string, callback WrappedCallable) error {
	// wrap the operation in a lock
	fe.usersLock.Lock()
	defer fe.usersLock.Unlock()

	user, ok := fe.users[userID]
	if !ok {
		return errors.New("unable to find the user balances")
	}
	account, ok := user.accounts[subAccountID]
	if !ok {
		return errors.New("unable to find the user balances account")
	}
	account.balancesLock.Lock()
	defer account.balancesLock.Unlock()
	balance, ok := account.balances[coin]

	if !ok {
		return errors.New("coin not found")
	}

	err := callback(balance)
	if err != nil {
		return err
	}

	return nil
}

// Cancel order
func (fe *FundsEngine) DoCancelOrder(userID, subAccountID uint64, coin string, amount *decimal.Big) error {
	if conv.NewDecimalWithPrecision().CheckNaNs(amount, nil) {
		return errors.New("amount is NaN")
	}

	return fe.Wrap(userID, subAccountID, coin, func(balance BalanceView) error {
		balance.Locked.Sub(balance.Locked, amount)
		balance.InOrders.Sub(balance.InOrders, amount)
		balance.Available.Add(balance.Available, amount)
		return nil
	})

	// // wrap the operation in a lock
	// fe.usersLock.Lock()
	// defer fe.usersLock.Unlock()

	// user, ok := fe.users[userID]
	// if !ok {
	// 	return errors.New("unable to find the user balances")
	// }
	// account, ok := user.accounts[subAccountID]
	// if !ok {
	// 	return errors.New("unable to find the user balances account")
	// }
	// account.balancesLock.Lock()
	// defer account.balancesLock.Unlock()
	// balance, ok := account.balances[coin]

	// if !ok {
	// 	return errors.New("coin not found")
	// }

	// balance.Locked.Sub(balance.Locked, amount)
	// balance.InOrders.Sub(balance.InOrders, amount)
	// balance.Available.Add(balance.Available, amount)
	// return nil
}

// Process trade
func (fe *FundsEngine) ProcessTrade(trade model.Trade, askOrder, bidOrder model.Order) error {
	return nil
}
