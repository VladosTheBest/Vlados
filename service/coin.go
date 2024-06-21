package service

import (
	"github.com/ericlagergren/decimal"
	coins "gitlab.com/paramountdax-exchange/exchange_api_v2/apps/coinsLocal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// ListCoins - return all defined coins available in the database
func (service *Service) ListCoins() ([]model.Coin, error) {
	coins := make([]model.Coin, 0)
	err := service.repo.FindAll(&coins)
	return coins, err
}

// ListCoinsByChain - list all coins for a chain
func (service *Service) ListCoinsByChain(symbol string) ([]model.Coin, error) {
	coins := make([]model.Coin, 0)
	db := service.repo.ConnReader.Where("chain_symbol = ?", symbol).Find(&coins)
	return coins, db.Error
}

// GetCoin - return a single coin by symbol
func (service *Service) GetCoin(symbol string) (*model.Coin, error) {
	coin := model.Coin{}
	db := service.repo.ConnReader.Where("symbol = ?", symbol).First(&coin)
	return &coin, db.Error
}

func (service *Service) GetCoinRate(symbolCrypto, symbolFiat string) (*decimal.Big, error) {
	currencyRates, err := service.coinValues.GetAll()
	if err != nil {
		return nil, err
	}
	return currencyRates[symbolCrypto][symbolFiat], nil
}

// AddCoin - add a new coin in the database
func (service *Service) AddCoin(
	coinType model.CoinType,
	chainSymbol, symbol, name string,
	digits, precision int,
	minWithdraw, withdrawFee, depositFee *decimal.Big,
	contractAddress string,
	status model.CoinStatus,
	costSymbol, blockchainExplorer string,
	minConfirmations int,
	shouldGetValue bool,
	withdrawFeeAdvCash, withdrawFeeClearJunction *decimal.Big,
) (*model.Coin, error) {
	coin := model.NewCoin(coinType, chainSymbol, symbol, name, digits, precision, minWithdraw, withdrawFee, depositFee, contractAddress, status, costSymbol, blockchainExplorer, minConfirmations, shouldGetValue, withdrawFeeAdvCash, withdrawFeeClearJunction)
	err := service.repo.Create(coin)
	return coin, err
}

// UpdateCoin - Update information about a coin
func (service *Service) UpdateCoin(
	coin *model.Coin, coinType model.CoinType,
	name, coinSymbol string, digits int,
	minWithdraw, withdrawFee, depositFee *decimal.Big,
	contractAddress string,
	status model.CoinStatus, costSymbol, blockchainExplorer string, minConfirmations int, shouldGetValue bool, chainSymbol string) (*model.Coin, error) {
	coin.Name = name
	coin.Symbol = coinSymbol
	coin.Status = status
	coin.Type = coinType
	coin.Digits = digits
	coin.MinWithdraw.V = minWithdraw
	coin.WithdrawFee.V = withdrawFee
	coin.DepositFee.V = depositFee
	coin.ContractAddress = contractAddress
	coin.CostSymbol = costSymbol
	coin.BlockchainExplorer = blockchainExplorer
	coin.MinConfirmations = minConfirmations
	coin.ShouldGetValue = shouldGetValue
	coin.ChainSymbol = chainSymbol

	err := service.repo.Update(coin)
	if err != nil {
		return nil, err
	}
	return coin, nil
}

// DeleteCoin - sets coin to inactive | should never delete
func (service *Service) DeleteCoin(coin *model.Coin) error {
	status, err := model.GetCoinStatusFromString("inactive")
	if err != nil {
		return err
	}
	coin.Status = status

	err = service.repo.Update(coin)
	return err
}

// GetCoinsValue - gets coins value in BTC or selected crypto
func (service *Service) GetCoinsValue() (coins.CoinValueData, error) {
	coinValues, err := service.coinValues.GetAll()
	return coinValues, err
}

func (service *Service) SetCoinHighlight(coin *model.Coin, switcher string) error {

	var highlight bool
	if switcher == "on" {
		highlight = true
	}

	err := service.repo.Conn.Model(coin).Update("highlight", highlight).Error

	return err
}

func (service *Service) SetCoinNewListing(coin *model.Coin, switcher string) error {

	var newLogin bool
	if switcher == "on" {
		newLogin = true
	}

	err := service.repo.Conn.Model(coin).Update("new_listing", newLogin).Error

	return err
}
