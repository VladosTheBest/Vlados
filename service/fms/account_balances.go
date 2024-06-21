package fms

import (
	"errors"

	"github.com/ericlagergren/decimal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
)

func (ab *AccountBalances) GetSubAccountID() uint64 {
	return ab.subAccountID
}

func (ab *AccountBalances) LockAccount() {
	ab.balancesLock.Lock()
}

func (ab *AccountBalances) UnlockAccount() {
	ab.balancesLock.Unlock()
}

func (ab *AccountBalances) RLockAccount() {
	ab.balancesLock.RLock()
}

func (ab *AccountBalances) RUnlockAccount() {
	ab.balancesLock.RUnlock()
}

func (ab *AccountBalances) GetAll() Balances {
	return ab.balances
}

func (ab *AccountBalances) GetAvailableBalanceForCoin(coin string) (*decimal.Big, error) {
	balance, ok := ab.balances[coin]
	if !ok {
		return nil, errors.New("unable to get balance")
	}

	return balance.Available, nil
}

func (ab *AccountBalances) GetLockedBalanceForCoin(coin string) (*decimal.Big, error) {
	balance, ok := ab.balances[coin]
	if !ok {
		return nil, errors.New("unable to get balance")
	}

	return balance.Locked, nil
}

func (ab *AccountBalances) GetTotalBalanceForCoin(coin string) (*decimal.Big, error) {
	balance, ok := ab.balances[coin]
	if !ok {
		return nil, errors.New("unable to get balance")
	}

	return conv.NewDecimalWithPrecision().Add(balance.Available, balance.Locked), nil
}

func (ab *AccountBalances) Deposit(coin string, amount *decimal.Big) error {
	balance, ok := ab.balances[coin]

	if !ok {
		return errors.New("coin not found")
	}

	if conv.NewDecimalWithPrecision().CheckNaNs(balance.Available, amount) {
		return errors.New("amount is NaN")
	}

	balance.Available.Add(balance.Available, amount)

	return nil
}

func (ab *AccountBalances) Withdrawal(coin string, amount *decimal.Big) error {
	balance, ok := ab.balances[coin]

	if !ok {
		return errors.New("coin not found")
	}

	if conv.NewDecimalWithPrecision().CheckNaNs(balance.Available, amount) {
		return errors.New("amount is NaN")
	}

	balance.Available.Sub(balance.Available, amount)

	return nil
}

func (ab *AccountBalances) Lock(coin string, amount *decimal.Big, inOrders bool) error {
	balance, ok := ab.balances[coin]

	if !ok {
		return errors.New("coin not found")
	}

	if conv.NewDecimalWithPrecision().CheckNaNs(amount, nil) {
		return errors.New("amount is NaN")
	}

	balance.Available.Sub(balance.Available, amount)
	balance.Locked.Add(balance.Locked, amount)
	if inOrders {
		balance.InOrders.Add(balance.InOrders, amount)
	}

	return nil
}

func (ab *AccountBalances) Unlock(coin string, amount *decimal.Big, inOrders bool) error {
	balance, ok := ab.balances[coin]

	if !ok {
		return errors.New("coin not found")
	}

	if conv.NewDecimalWithPrecision().CheckNaNs(amount, nil) {
		return errors.New("amount is NaN")
	}

	balance.Available.Add(balance.Available, amount)
	balance.Locked.Sub(balance.Locked, amount)
	if inOrders {
		balance.InOrders.Sub(balance.InOrders, amount)
	}

	return nil
}

// func (ab *AccountBalances) UnlockForOrder(coin string, amount *decimal.Big) error {
// 	balance, ok := ab.balances[coin]

// 	if !ok {
// 		return errors.New("coin not found")
// 	}

// 	if conv.NewDecimalWithPrecision().CheckNaNs(amount, nil) {
// 		return errors.New("amount is NaN")
// 	}

// 	balance.Locked.Sub(balance.Locked, amount)
// 	balance.InOrders.Sub(balance.InOrders, amount)

// 	return nil
// }
