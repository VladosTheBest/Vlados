package service

import (
	"github.com/ericlagergren/decimal"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/fms"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
	"strconv"
	"strings"
	"sync"
)

// Balance virtual entity
type Balance struct {
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	Available    string `json:"available"`
	Locked       string `json:"locked"`
	InOrders     string `json:"in_orders"`
	InBTC        string `json:"in_btc"`
	TotalBalance string `json:"total_balance"`
}

type balances struct {
	balances map[string]map[string]*Balance
	lock     *sync.RWMutex
}

type BalancesWithBotIDAndContractID struct {
	BotID        uint64              `json:"bot_id"`
	ContractID   uint64              `json:"contract_id"`
	SubAccountID string              `json:"sub_account_id"`
	Balances     map[string]*Balance `json:"balances"`
}

func (b *balances) Set(accountID string, balance map[string]*Balance) {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.balances[accountID] = balance
}

func (b *balances) Get(accountID string) map[string]*Balance {
	b.lock.Lock()
	defer b.lock.Unlock()

	return b.balances[accountID]
}

func (b *balances) GetUint64(accountID uint64) map[string]*Balance {
	id := strconv.FormatUint(accountID, 10)

	return b.Get(id)
}

func (b *balances) GetAll() map[string]map[string]*Balance {
	b.lock.RLock()
	defer b.lock.RUnlock()

	return b.balances
}

func NewBalances() *balances {
	return &balances{
		balances: map[string]map[string]*Balance{},
		lock:     &sync.RWMutex{},
	}
}

// GetLiabilityBalances - get all liability balances for a given user mapped for currencies
func (service *Service) GetLiabilityBalances(userID uint64, account *model.SubAccount) (model.Balances, error) {
	return service.repo.GetLiabilityBalances(userID, account)
}

// AddWithdrawRequest - add a new withdraw request to the db
func (service *Service) AddWithdrawRequest(userID uint64, coin string, amount, fee *decimal.Big, to string, fromAccount *model.SubAccount, externalSystem model.WithdrawExternalSystem, data string, accountBalances *fms.AccountBalances) (*model.WithdrawRequest, error) {
	return service.walletApp.AddWithdrawRequest(userID, coin, amount, fee, to, fromAccount, externalSystem, data, accountBalances)
}

// BlockUserFunds - blocks a user's funds when withdrawing for 30 minutes
func (service *Service) BlockUserFunds(userID uint64, coin string, withdrawRequest *model.WithdrawRequest, accountBalances *fms.AccountBalances) error {
	return service.ops.BlockUserFunds(userID, coin, withdrawRequest, accountBalances)
}

// AcceptWithdrawRequest - user accepts withdraw request (update to db) and lock funds for the user
func (service *Service) AcceptWithdrawRequest(withdrawRequest *model.WithdrawRequest) (*model.WithdrawRequest, error) {
	return service.walletApp.AcceptWithdrawRequest(withdrawRequest)
}

// GetAllLiabilityBalances returns balances for all active coins
func (service *Service) GetAllLiabilityBalances(balances *balances, userID uint64) error {

	wg := sync.WaitGroup{}

	accounts, err := subAccounts.GetUserSubAccounts(model.MarketTypeSpot, userID)
	if err != nil {
		return err
	}

	// create balance map
	for _, account := range accounts {
		wg.Add(1)
		go func(subAccount *model.SubAccount) {
			if err := service.GetAllLiabilityBalancesForSubAccount(balances, userID, subAccount); err != nil {
				log.Error().Err(err).Uint64("subAccountId", subAccount.ID).Msg("Unable to get balances")
			}
			wg.Done()
		}(account)

	}

	wg.Wait()

	return nil
}

// GetAllBalancesWithBotIDAndContractID returns balances for all active coins with bot id and contract id.
func (service *Service) GetAllBalancesWithBotIDAndContractID(balances *balances, userID uint64) ([]*BalancesWithBotIDAndContractID, error) {
	allBalances := balances.GetAll()
	var allBalancesWithBotIDAndContractID []*BalancesWithBotIDAndContractID
	var bots []model.Bot
	var contracts []*model.BonusAccountContract

	q := service.repo.ConnReader

	db := q.Find(&contracts, "user_id = ?", userID)
	if db.Error != nil {
		return nil, db.Error
	}

	dbc := q.Find(&bots, "user_id = ?", userID)
	if dbc.Error != nil {
		return nil, db.Error
	}

	for subAccountID, mapBalances := range allBalances {
		balance := BalancesWithBotIDAndContractID{
			SubAccountID: subAccountID,
			Balances:     mapBalances,
		}
		allBalancesWithBotIDAndContractID = append(allBalancesWithBotIDAndContractID, &balance)
	}

	for _, bot := range bots {
		for _, balance := range allBalancesWithBotIDAndContractID {
			balanceSubID, _ := strconv.Atoi(balance.SubAccountID)
			if bot.SubAccount == uint64(balanceSubID) {
				balance.BotID = bot.ID
			}
		}
	}

	for _, contract := range contracts {
		for _, balance := range allBalancesWithBotIDAndContractID {
			balanceSubID, _ := strconv.Atoi(balance.SubAccountID)
			if contract.SubAccount == uint64(balanceSubID) {
				balance.ContractID = contract.ID
			}
		}
	}

	return allBalancesWithBotIDAndContractID, nil
}

// GetAllLiabilityBalancesForSubAccount returns balances for all active coins
func (service *Service) GetAllLiabilityBalancesForSubAccount(balances *balances, userID uint64, subAccount *model.SubAccount) error {
	b := map[string]*Balance{}

	coins, err := service.ListCoins()
	if err != nil {
		return err
	}

	for _, coin := range coins {
		b[coin.Symbol] = &Balance{
			Symbol:       coin.Symbol,
			Name:         coin.Name,
			Available:    "0",
			Locked:       "0",
			InOrders:     "0",
			InBTC:        "0",
			TotalBalance: "0",
		}
	}

	accountBalances, err := service.FundsEngine.GetAccountBalances(userID, subAccount.ID)
	if err != nil {
		return err
	}

	accountBalances.RLockAccount()
	defer accountBalances.RUnlockAccount()
	// get coin values
	coinValues, _ := service.coinValues.GetAll()

	for symbol, balance := range accountBalances.GetAll() {
		totalBalance := new(decimal.Big)
		totalBalance.Add(totalBalance, balance.Available)
		totalBalance.Add(totalBalance, balance.Locked)

		b[symbol].Available = utils.Fmt(balance.Available)
		b[symbol].Locked = utils.Fmt(balance.Locked)
		b[symbol].InOrders = utils.Fmt(balance.InOrders)
		b[symbol].TotalBalance = utils.Fmt(totalBalance)

		if symbol == "btc" {
			b[symbol].InBTC = b[symbol].Available
		} else if val, ok := coinValues[strings.ToUpper(symbol)]; ok {
			if coinBtcValue, ok := val["BTC"]; ok {
				b[symbol].InBTC = utils.Fmt(new(decimal.Big).Mul(new(decimal.Big).Add(balance.Available, balance.InOrders), coinBtcValue))
			}
		}
	}

	var label = strconv.FormatUint(subAccount.ID, 10)
	if subAccount.ID == 0 {
		switch subAccount.AccountGroup {
		case model.AccountGroupMain:
			label = "main"
		case model.AccountGroupBonus:
			label = "bonus"
		}
	}

	balances.Set(label, b)

	return nil
}
