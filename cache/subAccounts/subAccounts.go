package subAccounts

import (
	"errors"
	"strconv"
	"sync"
	"time"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

type subAccountsCache struct {
	allByUser               map[model.MarketType]map[uint64]map[uint64]*model.SubAccount
	defaultSubaccountByUser map[model.MarketType]map[uint64]map[model.AccountGroup]*model.SubAccount
	mainAccount             map[model.MarketType]map[uint64]map[model.AccountGroup]*model.SubAccount
	cardSubAccount          map[model.MarketType]map[uint64]map[model.AccountGroup]*model.SubAccount
	lock                    *sync.RWMutex
	repo                    *queries.Repo
}

var Error_UnableToFind = errors.New("unable to find accounts")

var cache *subAccountsCache

func init() {
	cache = &subAccountsCache{
		lock:                    &sync.RWMutex{},
		allByUser:               map[model.MarketType]map[uint64]map[uint64]*model.SubAccount{},
		mainAccount:             map[model.MarketType]map[uint64]map[model.AccountGroup]*model.SubAccount{},
		defaultSubaccountByUser: map[model.MarketType]map[uint64]map[model.AccountGroup]*model.SubAccount{},
		cardSubAccount:          map[model.MarketType]map[uint64]map[model.AccountGroup]*model.SubAccount{},
	}
}

func InitCacheRepo(repo *queries.Repo) {
	cache.repo = repo
}

func InitDefaultUsers(userIds []uint64) {
	for _, ag := range model.AccountGroupList {
		for _, userId := range userIds {
			_, _ = GetUserMainSubAccount(model.MarketTypeSpot, userId, ag)
		}
	}
}

func SetAll(accounts []*model.SubAccount) {
	cache.lock.Lock()
	for _, account := range accounts {
		Set(account, true)
	}
	cache.lock.Unlock()
}

func Set(account *model.SubAccount, skipLock bool) {
	if !skipLock {
		cache.lock.Lock()
		defer cache.lock.Unlock()
	}

	// Ensure all necessary maps are initialized
	ensureMapInitForUser(&cache.allByUser, account.MarketType, account.UserId)
	cache.allByUser[account.MarketType][account.UserId][account.ID] = account

	if account.IsMain {
		ensureMapInitForAccountGroup(&cache.mainAccount, account.MarketType, account.UserId)
		cache.mainAccount[account.MarketType][account.UserId][account.AccountGroup] = account
	}
	if account.IsDefault {
		ensureMapInitForAccountGroup(&cache.defaultSubaccountByUser, account.MarketType, account.UserId)
		cache.defaultSubaccountByUser[account.MarketType][account.UserId][account.AccountGroup] = account
	}
	if account.AccountGroup == model.AccountGroupCardPayment {
		ensureMapInitForAccountGroup(&cache.cardSubAccount, account.MarketType, account.UserId)
		cache.cardSubAccount[account.MarketType][account.UserId][account.AccountGroup] = account
	}
}

func ensureMapInitForUser(m *map[model.MarketType]map[uint64]map[uint64]*model.SubAccount, marketType model.MarketType, userId uint64) {
	if (*m)[marketType] == nil {
		(*m)[marketType] = make(map[uint64]map[uint64]*model.SubAccount)
	}
	if (*m)[marketType][userId] == nil {
		(*m)[marketType][userId] = make(map[uint64]*model.SubAccount)
	}
}

func ensureMapInitForAccountGroup(m *map[model.MarketType]map[uint64]map[model.AccountGroup]*model.SubAccount, marketType model.MarketType, userId uint64) {
	if (*m)[marketType] == nil {
		(*m)[marketType] = make(map[uint64]map[model.AccountGroup]*model.SubAccount)
	}
	if (*m)[marketType][userId] == nil {
		(*m)[marketType][userId] = make(map[model.AccountGroup]*model.SubAccount)
	}
}

func GetUserSubAccounts(marketType model.MarketType, userId uint64) (map[uint64]*model.SubAccount, error) {
	cache.lock.RLock()
	accounts, ok := cache.allByUser[marketType][userId]
	cache.lock.RUnlock()

	if !ok {
		return nil, Error_UnableToFind
	}

	return accounts, nil
}

func GetTradeUserSubAccountByID(marketType model.MarketType, askOwnerID, askSubaccountID, bidOwnerID, bidSubaccountID uint64) (*model.SubAccount, *model.SubAccount, error, error) {
	setAsk := false
	setBid := false
	var askErr, bidErr error = nil, nil
	cache.lock.RLock()
	askAccount, askOk := cache.allByUser[marketType][askOwnerID][askSubaccountID]
	bidAccount, bidOk := cache.allByUser[marketType][bidOwnerID][bidSubaccountID]

	if !askOk {
		if askSubaccountID == 0 {
			askAccount, askOk = cache.mainAccount[marketType][askOwnerID][model.AccountGroupMain]
			if !askOk {
				askAccount = getDefaultMainAccount(marketType, askOwnerID)
				setAsk = true
			}
		}
		askErr = Error_UnableToFind
	}

	if !bidOk {
		if bidSubaccountID == 0 {
			bidAccount, bidOk = cache.mainAccount[marketType][bidOwnerID][model.AccountGroupMain]
			if !bidOk {
				bidAccount = getDefaultMainAccount(marketType, bidOwnerID)
				setBid = true
			}
		}
		bidErr = Error_UnableToFind
	}
	cache.lock.RUnlock()

	if setAsk {
		Set(askAccount, false)
	}
	if setBid {
		Set(bidAccount, false)
	}

	if askAccount.Status != model.SubAccountStatusActive {
		askErr = errors.New("account not active")
	}

	if bidAccount.Status != model.SubAccountStatusActive {
		bidErr = errors.New("account not active")
	}

	return askAccount, bidAccount, askErr, bidErr
}

func GetUserSubAccountByID(marketType model.MarketType, userId uint64, id uint64) (*model.SubAccount, error) {
	if cache.repo == nil || cache.repo.Conn == nil {
		return nil, errors.New("repository or connection is not initialized")
	}
	var account *model.SubAccount
	cache.lock.RLock()
	account, ok := cache.allByUser[marketType][userId][id]
	cache.lock.RUnlock()

	if !ok {
		if err := cache.repo.Conn.Where("user_id = ? and id = ?", userId, id).First(&account).Error; err != nil {
			return nil, Error_UnableToFind
		}
		Set(account, false) // Make sure Set properly handles locking
	}

	if account.Status != model.SubAccountStatusActive {
		return nil, errors.New("account not active")
	}

	return account, nil
}

func GetUserCardAccount(marketType model.MarketType, userId uint64, ag model.AccountGroup) (*model.SubAccount, error) {
	cache.lock.RLock()
	account, ok := cache.cardSubAccount[marketType][userId][ag]
	cache.lock.RUnlock()

	if !ok {
		return nil, Error_UnableToFind
	}
	return account, nil
}

func GetUserMainSubAccount(marketType model.MarketType, userId uint64, ag model.AccountGroup) (*model.SubAccount, error) {
	cache.lock.RLock()
	account, ok := cache.mainAccount[marketType][userId][ag]
	cache.lock.RUnlock()

	if !ok {
		if ag == model.AccountGroupMain {
			account := getDefaultMainAccount(marketType, userId)
			Set(account, false)
			return account, nil
		}
		return nil, Error_UnableToFind
	}

	return account, nil
}

func GetUserDefaultSubAccount(marketType model.MarketType, userId uint64, ag model.AccountGroup) (*model.SubAccount, error) {
	cache.lock.RLock()
	account, ok := cache.defaultSubaccountByUser[marketType][userId][ag]
	cache.lock.RUnlock()

	if !ok {
		if ag == model.AccountGroupMain {
			account := getDefaultMainAccount(marketType, userId)
			Set(account, false)
			return account, nil
		}
		return nil, Error_UnableToFind
	}

	return account, nil
}

func SetUserSubAccountDefault(marketType model.MarketType, userId uint64, ag model.AccountGroup, newAccount *model.SubAccount) error {
	// Check if the new account is valid
	if newAccount == nil || newAccount.UserId != userId || newAccount.AccountGroup != ag {
		return errors.New("invalid new default account")
	}

	// Obtain write lock
	cache.lock.Lock()
	defer cache.lock.Unlock()

	// Get the current default account
	currentDefaultAccount, ok := cache.defaultSubaccountByUser[marketType][userId][ag]

	// If the current default account is already the new account, do nothing
	if ok && currentDefaultAccount.ID == newAccount.ID {
		return nil
	}

	// If the current default account exists, set its IsDefault flag to false
	if ok {
		currentDefaultAccount.IsDefault = false
	}

	// Set the new account as the default account
	newAccount.IsDefault = true
	cache.defaultSubaccountByUser[marketType][userId][ag] = newAccount

	return nil
}

func getDefaultMainAccount(marketType model.MarketType, userId uint64) *model.SubAccount {
	return &model.SubAccount{
		ID:                0,
		UserId:            userId,
		AccountGroup:      model.AccountGroupMain,
		MarketType:        marketType,
		DepositAllowed:    true,
		WithdrawalAllowed: true,
		TransferAllowed:   true,
		IsDefault:         true,
		IsMain:            true,
		Title:             "Main Account",
		Comment:           "Default Main Account",
		Status:            model.SubAccountStatusActive,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}
func ConvertAccountGroupToAccount(userId uint64, ag string) (uint64, error) {
	switch ag {
	case model.AccountGroupMain.String():
		account, err := GetUserDefaultSubAccount(model.MarketTypeSpot, userId, model.AccountGroupMain)
		if err != nil {
			return 0, err
		}
		return account.ID, nil
	case model.AccountGroupBonus.String():
		account, err := GetUserMainSubAccount(model.MarketTypeSpot, userId, model.AccountGroupBonus)
		if err != nil {
			return 0, err
		}
		return account.ID, nil
	default:
		accId, err := strconv.ParseUint(ag, 10, 64)
		if err != nil {
			return 0, errors.New("wrong account parameter")
		}

		account, err := GetUserSubAccountByID(model.MarketTypeSpot, userId, accId)
		if err != nil {
			return 0, err
		}
		return account.ID, nil
	}
}
