package oms

import (
	"errors"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"sync"
)

type marketDepthCacheItem struct {
	level2 model.MarketDepthLevel2
	level1 model.MarketDepthLevel1
}

type marketDepthCache struct {
	ob   map[string]marketDepthCacheItem
	lock *sync.RWMutex
}

func (m *marketDepthCache) SetDepth(marketID string, depth marketDepthCacheItem) {
	m.lock.Lock()
	m.ob[marketID] = depth
	m.lock.Unlock()
}

func (m *marketDepthCache) GetDepthLevel1(marketID string) (*model.MarketDepthLevel1, error) {
	m.lock.RLock()
	depth, ok := m.ob[marketID]
	m.lock.RUnlock()

	if !ok {
		return nil, errors.New("depth not found")
	}

	return &depth.level1, nil
}

func (m *marketDepthCache) GetDepthLevel2(marketID string) (*model.MarketDepthLevel2, error) {
	m.lock.RLock()
	depth, ok := m.ob[marketID]
	m.lock.RUnlock()

	if !ok {
		return nil, errors.New("depth not found")
	}

	return &depth.level2, nil
}
