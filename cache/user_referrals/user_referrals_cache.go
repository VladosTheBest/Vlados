package user_referrals

import (
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"sync"
)

type Cache struct {
	userReferred struct {
		users map[uint64]*model.ReferralTree
		lock  *sync.RWMutex
	}
	userReferrals struct {
		users map[uint64]*struct {
			L1 uint64
			L2 uint64
			L3 uint64
		}
		lock *sync.RWMutex
	}
}

var cache *Cache

func init() {
	cache = &Cache{
		userReferred: struct {
			users map[uint64]*model.ReferralTree
			lock  *sync.RWMutex
		}{users: make(map[uint64]*model.ReferralTree), lock: &sync.RWMutex{}},

		userReferrals: struct {
			users map[uint64]*struct {
				L1 uint64
				L2 uint64
				L3 uint64
			}
			lock *sync.RWMutex
		}{users: make(map[uint64]*struct {
			L1 uint64
			L2 uint64
			L3 uint64
		}), lock: &sync.RWMutex{}},
	}
}

func GetUserReferrals(userId uint64) *model.ReferralTree {
	tree := new(model.ReferralTree)
	cache.userReferrals.lock.RLock()
	defer cache.userReferrals.lock.RUnlock()
	referralTree, ok := cache.userReferrals.users[userId]
	if ok {
		if referralTree.L1 != 0 {
			tree.L1 = append(tree.L1, referralTree.L1)
		}
		if referralTree.L2 != 0 {
			tree.L2 = append(tree.L2, referralTree.L2)
		}
		if referralTree.L3 != 0 {
			tree.L3 = append(tree.L3, referralTree.L3)
		}
	}

	return tree
}

func AddReferralUser(referralUserId uint64, user *model.User) {
	cache.userReferrals.lock.Lock()
	referredL2User := cache.userReferrals.users[referralUserId].L1
	referredL3User := cache.userReferrals.users[referralUserId].L2
	if cache.userReferrals.users[user.ID] == nil {
		cache.userReferrals.users[user.ID] = new(struct {
			L1 uint64
			L2 uint64
			L3 uint64
		})
	}
	userReferral := cache.userReferrals.users[user.ID]
	userReferral.L1 = referralUserId
	userReferral.L2 = referredL2User
	userReferral.L3 = referredL3User
	cache.userReferrals.users[user.ID] = userReferral
	cache.userReferrals.lock.Unlock()

	cache.userReferred.lock.Lock()
	cache.userReferred.users[user.ID] = new(model.ReferralTree)
	cache.userReferred.users[referralUserId].L1 = append(cache.userReferred.users[referralUserId].L1, user.ID)
	if referredL2User != 0 {
		cache.userReferred.users[referredL2User].L2 = append(cache.userReferred.users[referredL2User].L2, user.ID)
	}
	if referredL3User != 0 {
		cache.userReferred.users[referredL3User].L3 = append(cache.userReferred.users[referredL3User].L3, user.ID)
	}
	cache.userReferred.lock.Unlock()
}

func SetUserReferralData(userReferrals, userReferreds *model.ReferralAllTreeList) {
	cache.userReferred.lock.Lock()
	for _, userReferred := range userReferreds.ReferralsList {
		if cache.userReferred.users == nil {
			cache.userReferred.users = make(map[uint64]*model.ReferralTree)
		}
		if cache.userReferred.users[userReferred.UserId] == nil {
			cache.userReferred.users[userReferred.UserId] = new(model.ReferralTree)
		}
		cache.userReferred.users[userReferred.UserId].L1 = userReferred.L1
		cache.userReferred.users[userReferred.UserId].L2 = userReferred.L2
		cache.userReferred.users[userReferred.UserId].L3 = userReferred.L3
	}
	cache.userReferred.lock.Unlock()

	cache.userReferrals.lock.Lock()
	for _, userReferral := range userReferrals.ReferralsList {
		if cache.userReferrals.users[userReferral.UserId] == nil {
			cache.userReferrals.users[userReferral.UserId] = new(struct {
				L1 uint64
				L2 uint64
				L3 uint64
			})
		}
		cachedUserReferral := cache.userReferrals.users[userReferral.UserId]

		if cachedUserReferral == nil {
			cachedUserReferral = new(struct {
				L1 uint64
				L2 uint64
				L3 uint64
			})
		}
		if len(userReferral.L1) != 0 {
			cachedUserReferral.L1 = userReferral.L1[0]
		}
		if len(userReferral.L2) != 0 {
			cachedUserReferral.L2 = userReferral.L2[0]
		}
		if len(userReferral.L3) != 0 {
			cachedUserReferral.L3 = userReferral.L3[0]
		}
	}
	cache.userReferrals.lock.Unlock()
}
