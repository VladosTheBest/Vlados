package fms

import (
	"context"
	"errors"
	"sync"

	"github.com/rs/zerolog/log"
	subAccounts2 "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

func Init(repo *queries.Repo, ctx context.Context) *FundsEngine {
	return &FundsEngine{
		ctx:       ctx,
		usersLock: &sync.RWMutex{},
		users:     map[uint64]*user{},
		repo:      repo,
	}
}

func (fe *FundsEngine) InitAccounts() error {
	fe.usersLock.Lock()
	defer fe.usersLock.Unlock()

	var users []*model.User
	if err := fe.repo.Conn.Find(&users, "status = ?", model.UserStatusActive).Error; err != nil {
		log.Error().Err(err).Str("section", "FMS").Msg("Unable to load active users")
		return err
	}

	var subAccountsList []*model.SubAccount
	if err := fe.repo.Conn.Find(&subAccountsList, "status = ?", model.SubAccountStatusActive).Error; err != nil {
		return err
	}

	accountsMap := map[uint64][]*model.SubAccount{}

	for _, subAccount := range subAccountsList {
		accountsMap[subAccount.UserId] = append(accountsMap[subAccount.UserId], subAccount)
	}

	for _, u := range users {
		fe.users[u.ID] = &user{
			accountsLock: &sync.RWMutex{},
			accounts:     map[uint64]*AccountBalances{},
		}

		if subAccount, err := subAccounts2.GetUserMainSubAccount(model.MarketTypeSpot, u.ID, model.AccountGroupMain); err == nil {
			if _, err := fe.InitAccountBalances(subAccount, true); err != nil {
				return err
			}
		} else {
			return err
		}

		if userSubAccounts, ok := accountsMap[u.ID]; ok {
			for _, subAccount := range userSubAccounts {
				if _, err := fe.InitAccountBalances(subAccount, true); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (fe *FundsEngine) InitAccountBalances(subAccount *model.SubAccount, skipLock bool) (*AccountBalances, error) {
	log.Debug().Str("package", "FMS").Str("func", "InitAccountBalances").Uint64("user_id", subAccount.UserId).Msg("Init account balances")
	balances, err := fe.repo.GetLiabilityBalances(subAccount.UserId, subAccount)
	if err != nil {
		log.Debug().Err(err).Str("package", "FMS").Str("func", "GetAccountBalances").Msg("Unable to get liability balances")
		return nil, err
	}

	if !skipLock {
		fe.usersLock.Lock()
		defer fe.usersLock.Unlock()
	}

	fmsBalances := Balances{}
	for k, v := range balances {
		fmsBalances[k] = BalanceView{
			Available: v.Available,
			Locked:    v.Locked,
			InOrders:  v.InOrders,
		}
		log.Warn().Str("package", "FMS").Str("func", "InitAccountBalances").
			Uint64("user_id", subAccount.UserId).
			Uint64("subaccount_id", subAccount.ID).
			Str("coin", k).
			Str("available", v.Available.String()).
			Str("locked", v.Locked.String()).
			Str("in_orders", v.InOrders.String()).
			Msg("User balance loaded")
	}

	ab := &AccountBalances{
		balancesLock: &sync.RWMutex{},
		balances:     fmsBalances,
		subAccountID: subAccount.ID,
		userID:       subAccount.UserId,
	}

	newUser, ok := fe.users[subAccount.UserId]
	if !ok {
		newUser = &user{
			accountsLock: &sync.RWMutex{},
			accounts:     map[uint64]*AccountBalances{},
		}
		fe.users[subAccount.UserId] = newUser
	}

	newUser.accountsLock.Lock()
	newUser.accounts[subAccount.ID] = ab
	newUser.accountsLock.Unlock()

	return ab, nil
}

func (fe *FundsEngine) GetAccountBalances(userID, subAccountID uint64) (*AccountBalances, error) {
	log.Debug().Str("package", "FMS").Str("func", "GetAccountBalances").Uint64("user_id", userID).Msg("Get account balances")
	fe.usersLock.RLock()
	user, ok := fe.users[userID]
	fe.usersLock.RUnlock()

	if !ok {
		log.Debug().Str("package", "FMS").Str("func", "GetAccountBalances").Uint64("user_id", userID).Msg("unable to find the user balances")
		return nil, errors.New("unable to find the user balances")
	}

	user.accountsLock.RLock()
	account, ok := user.accounts[subAccountID]
	user.accountsLock.RUnlock()

	if !ok {
		log.Debug().Str("package", "FMS").Str("func", "GetAccountBalances").Uint64("user_id", userID).Msg("unable to find the user balances account")
		return nil, errors.New("unable to find the user balances account")
	}

	return account, nil
}
