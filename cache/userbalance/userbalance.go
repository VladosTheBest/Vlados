package userbalance

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	kafkaGo "github.com/segmentio/kafka-go"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/data/balance_update_trigger"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/net/manager"
)

// Cache godoc
type Cache struct {
	// the local cache of all users (who need balance update)
	users     map[uint64]map[uint64]struct{}
	lock      *sync.RWMutex
	dm        *manager.DataManager
	publisher chan *balance_update_trigger.BalanceUpdateEvent
}

var (
	// cache instance that keeps keys of users in the system
	cache *Cache
	//  channel that accepts userID (key) with timestamp (value) for balance update
	// TsWaiter chan map[uint64]int64
)

func Init(dm *manager.DataManager) *Cache {
	cache = &Cache{
		users:     make(map[uint64]map[uint64]struct{}),
		lock:      &sync.RWMutex{},
		publisher: make(chan *balance_update_trigger.BalanceUpdateEvent, 10000),
		dm:        dm,
	}
	return cache
}

func (c *Cache) publish(triggerEvents []*balance_update_trigger.BalanceUpdateEvent) error {
	msgs := []kafkaGo.Message{}

	for _, event := range triggerEvents {
		bytes, err := event.ToBinary()
		if err != nil {
			return err
		}
		msg := kafkaGo.Message{Value: bytes}
		msgs = append(msgs, msg)
	}

	return cache.dm.Publish("balance_update_trigger", map[string]string{}, msgs...)
}

func (c *Cache) Process(ctx context.Context, wait *sync.WaitGroup) {
	log.Info().Str("cron", "user_balance_cache").Str("action", "start").Msg("User balance cache cron - started")

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	events := []*balance_update_trigger.BalanceUpdateEvent{}

	for {
		select {
		case <-ctx.Done():
			if err := c.publish(events); err != nil {
				log.Error().
					Str("section", "SetWithPublish").
					Err(err).
					Msg("Unable to send balance_update_trigger")
			}
			log.Info().Str("cron", "user_balance_cache").Str("action", "stop").Msg("8 => User balance cache cron - stopped")
			wait.Done()
			return
		case triggerEvent, ok := <-c.publisher:
			if !ok {
				return
			}
			events = append(events, triggerEvent)
		case <-ticker.C:
			if len(events) == 0 {
				continue
			}
			if err := c.publish(events); err != nil {
				log.Error().
					Str("section", "SetWithPublish").
					Err(err).
					Msg("Enable to send balance_update_trigger")
			} else {
				events = events[0:0]
			}
		}
	}
}

// IsEmpty godoc
// Checks if user cache map is empty with use of RLock
func IsEmpty() bool {
	cache.lock.RLock()
	isEmpty := len(cache.users) == 0
	cache.lock.RUnlock()
	return isEmpty
}

// PopAll godoc
// Extract the map with all users from the cache and clear it
func PopAll() map[uint64]map[uint64]struct{} {
	cache.lock.Lock()
	users := cache.users
	cache.users = make(map[uint64]map[uint64]struct{})
	cache.lock.Unlock()
	return users
}

// Set godoc
// Update a single userID key in the cache
func set(userID uint64, subAccountId uint64) {
	cache.lock.Lock()
	if _, ok := cache.users[userID]; !ok {
		cache.users[userID] = make(map[uint64]struct{})
	}
	cache.users[userID][subAccountId] = struct{}{}
	cache.lock.Unlock()
}

// SetAll godoc
// Replace users map in cache with the given map
func SetAll(users map[uint64]map[uint64]struct{}) {
	cache.lock.Lock()
	cache.users = users
	cache.lock.Unlock()
}

func Process(msg *kafkaGo.Message) error {
	event := &balance_update_trigger.BalanceUpdateEvent{}
	err := event.FromBinary(msg.Value)
	if err != nil {
		return err
	}

	if event.IsInternal {
		set(event.UserID, event.SubAccount)
	}

	return nil
}

func Set(userID uint64, subAccountID uint64) {
	set(userID, subAccountID)
}

func SetWithPublish(userID uint64, subAccountID uint64) {
	set(userID, subAccountID)

	triggerEvent := balance_update_trigger.BalanceUpdateEvent{
		UserID:     userID,
		SubAccount: subAccountID,
		IsInternal: true,
	}

	cache.publisher <- &triggerEvent
}
