package service

import (
	"github.com/ericlagergren/decimal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"strings"
)

func (service *Service) GetPercentFromBalance24h(userID uint64, account *model.SubAccount) (*decimal.Big, error) {
	var percent *decimal.Big
	totalCrossYesterday, err := service.repo.GetTotalCross(userID, account)
	if err != nil {
		return nil, err
	}

	crossRates, err := service.coinValues.GetAll()
	if err != nil {
		return nil, err
	}

	balances, err := service.GetLiabilityBalances(userID, account)
	if err != nil {
		return nil, err
	}

	currentBalance := conv.NewDecimalWithPrecision()
	for coinSymbol, balance := range balances {
		total := new(decimal.Big).Add(balance.Available, balance.Locked)
		if total != nil && crossRates[strings.ToUpper(coinSymbol)]["USDT"] != nil {
			totalCross := new(decimal.Big).Mul(crossRates[strings.ToUpper(coinSymbol)]["USDT"], total)
			currentBalance.Add(currentBalance, totalCross)
		}
	}
	difference := new(decimal.Big).Sub(currentBalance, totalCrossYesterday.V)
	percent = new(decimal.Big).Quo(difference, totalCrossYesterday.V)
	percent = percent.Mul(percent, decimal.New(100, 0))

	if percent.IsInf(0) || percent.CheckNaNs(percent, nil) {
		return new(decimal.Big), nil
	}
	result := percent.Quantize(2)

	return result, nil
}

func (service *Service) Balances24hForWeek(userID uint64, account *model.SubAccount) (map[string]map[string]*decimal.Big, error) {

	result := make(map[string]map[string]*decimal.Big)
	balances24hWeek, err := service.repo.GetBalances24hWeek(userID, account)
	if err != nil {
		return result, err
	}

	for _, balance24h := range balances24hWeek {
		balance, ok := result[balance24h.CreatedAt.String()]
		if !ok {
			balance = make(map[string]*decimal.Big)
			result[balance24h.CreatedAt.String()] = balance
		}
		balance[balance24h.CoinSymbol] = balance24h.Percent.V
	}

	return result, nil
}
