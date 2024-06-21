package fms

import (
	"errors"

	"github.com/ericlagergren/decimal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
)

func checkNaNs(amount *decimal.Big) error {
	if conv.NewDecimalWithPrecision().CheckNaNs(amount, nil) {
		return errors.New("amount is NaN")
	}
	return nil
}
