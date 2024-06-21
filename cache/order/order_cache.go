package order

import (
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"sync"
)

type UpdateOrderData map[string]model.Order

type Cache struct {
	orders map[uint64]struct {
		all           map[uint64]*model.Order
		bySubAccounts map[uint64]map[uint64]*model.Order
	}

	lock *sync.RWMutex
}

type SubscribeCache struct {
	subscribes map[uint64]map[int64]uint64

	lock *sync.RWMutex
}

var cache *Cache
var subscribeCache *SubscribeCache

// userid => all -> orderId ->  order
//
//	bySubAccounts -> subaccountid -> orderid -> order
func init() {
	cache = &Cache{
		orders: make(map[uint64]struct {
			all           map[uint64]*model.Order
			bySubAccounts map[uint64]map[uint64]*model.Order
		}),
		lock: &sync.RWMutex{},
	}
	subscribeCache = &SubscribeCache{
		subscribes: make(map[uint64]map[int64]uint64),
		lock:       &sync.RWMutex{},
	}
}

func GetAllByUserId(userId uint64) (orders []*model.Order) {
	cache.lock.Lock()
	defer cache.lock.Unlock()
	orders = make([]*model.Order, 0)
	userIdOrders, ok := cache.orders[userId]
	if !ok {
		return orders
	}
	if userIdOrders.all == nil {
		return orders
	}
	for i := range userIdOrders.all {
		orders = append(orders, userIdOrders.all[i])
		delete(userIdOrders.all, i)
	}

	return orders
}

func GetBySubAccounts(userId, subAccountId uint64) (orders []*model.Order) {
	cache.lock.Lock()
	defer cache.lock.Unlock()
	orders = make([]*model.Order, 0)
	userIdOrders, ok := cache.orders[userId]
	if !ok {
		return orders
	}
	if userIdOrders.bySubAccounts == nil {
		return orders
	}
	subAccountOrders, ok := userIdOrders.bySubAccounts[subAccountId]
	if !ok {
		return orders
	}
	for i := range subAccountOrders {
		orders = append(orders, subAccountOrders[i])
		delete(subAccountOrders, i)
	}
	return orders
}

func SetSubscribe(userId uint64, subAccountId int64) {
	subscribeCache.lock.Lock()
	_, ok := subscribeCache.subscribes[userId]
	if !ok {
		subscribeCache.subscribes[userId] = make(map[int64]uint64)
	}
	subscribeCache.subscribes[userId][subAccountId] = userId

	subscribeCache.lock.Unlock()
}

func IsSubscribed(userId, subAccountId uint64) bool {
	subscribeCache.lock.RLock()
	defer subscribeCache.lock.RUnlock()
	userIdSubscribe, ok := subscribeCache.subscribes[userId]
	if !ok {
		return false
	}
	_, ok = userIdSubscribe[-1]
	if ok {
		return true
	}
	_, ok = userIdSubscribe[int64(subAccountId)]

	return ok
}

func DeleteSubscribe(userId uint64, subAccountId int64) {
	subscribeCache.lock.Lock()
	_, ok := subscribeCache.subscribes[userId]
	if ok {
		delete(subscribeCache.subscribes[userId], subAccountId)
	}
	subscribeCache.lock.Unlock()
}

func GetAllSubscribes() map[uint64]map[int64]uint64 {
	var subscribes map[uint64]map[int64]uint64
	subscribeCache.lock.RLock()
	subscribes = subscribeCache.subscribes
	subscribeCache.lock.RUnlock()
	return subscribes
}

func isSubscribedToAll(userId uint64) bool {
	subscribeCache.lock.RLock()
	defer subscribeCache.lock.RUnlock()
	userIdSubscribe, ok := subscribeCache.subscribes[userId]
	if !ok {
		return false
	}
	_, ok = userIdSubscribe[-1]

	return ok
}

func isSubscribedToSubAccounts(userId, subAccountId uint64) bool {
	subscribeCache.lock.RLock()
	defer subscribeCache.lock.RUnlock()
	userIdSubscribe, ok := subscribeCache.subscribes[userId]
	if !ok {
		return false
	}
	_, ok = userIdSubscribe[int64(subAccountId)]

	return ok
}

func Set(order *model.Order) {
	cache.lock.Lock()
	defer cache.lock.Unlock()
	userIdOrders := cache.orders[order.OwnerID]
	if isSubscribedToAll(order.OwnerID) {
		if userIdOrders.all == nil {
			userIdOrders.all = make(map[uint64]*model.Order)
		}
		userIdOrders.all[order.ID] = order
	}
	if isSubscribedToSubAccounts(order.OwnerID, order.SubAccount) {
		if userIdOrders.bySubAccounts == nil {
			userIdOrders.bySubAccounts = make(map[uint64]map[uint64]*model.Order)
		}
		_, ok := userIdOrders.bySubAccounts[order.SubAccount]
		if !ok {
			userIdOrders.bySubAccounts[order.SubAccount] = make(map[uint64]*model.Order)
		}
		userIdOrders.bySubAccounts[order.SubAccount][order.ID] = order
	}

	cache.orders[order.OwnerID] = userIdOrders
}

func EraseAllOrderHolder(userId uint64) {
	cache.lock.Lock()
	defer cache.lock.Unlock()
	userIdOrders := cache.orders[userId]
	userIdOrders.all = make(map[uint64]*model.Order)
	cache.orders[userId] = userIdOrders
}

func EraseSubAccountOrderHolder(userId, subAccountId uint64) {
	cache.lock.Lock()
	defer cache.lock.Unlock()
	userIdOrders := cache.orders[userId]
	if userIdOrders.bySubAccounts == nil {
		userIdOrders.bySubAccounts = make(map[uint64]map[uint64]*model.Order)
	}
	userIdOrders.bySubAccounts[subAccountId] = make(map[uint64]*model.Order)
	cache.orders[userId] = userIdOrders
}
