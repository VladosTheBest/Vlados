package oms

import (
	"sync"
	"time"

	"github.com/ericlagergren/decimal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

type LastPriceItem struct {
	Price  *decimal.Big
	Volume *decimal.Big
	Time   int64
}
type lastPrice struct {
	repo   *queries.Repo
	prices map[string]LastPriceItem
	lock   *sync.RWMutex
}

func (p *lastPrice) Set(trade *model.Trade) {
	p.lock.Lock()
	p.prices[trade.MarketID] = LastPriceItem{Price: trade.Price.V, Volume: trade.Volume.V, Time: time.Unix(0, trade.Timestamp).Unix()}
	p.lock.Unlock()
}

func (p *lastPrice) Get(marketID string) LastPriceItem {
	p.lock.RLock()
	lastPrice, ok := p.prices[marketID]
	p.lock.RUnlock()

	if !ok {
		var trade *model.Trade
		if err := p.repo.ConnReader.
			Table("trades").
			Where("market_id = ?", marketID).
			Order("seqid DESC").
			First(&trade).Error; err != nil {
			return LastPriceItem{
				Price:  conv.NewDecimalWithPrecision(),
				Volume: conv.NewDecimalWithPrecision(),
				Time:   0,
			}
		}
		p.Set(trade)
		return p.Get(marketID)
	}

	return lastPrice
}
