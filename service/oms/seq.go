package oms

import (
	"context"
	"fmt"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/gostop"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

type seq struct {
	lastOrderSeq *uint64
	lastTradeSeq *uint64
	prevOrderSeq *uint64
	prevTradeSeq *uint64
	repo         *queries.Repo
	ctx          context.Context
}

func (s *seq) init() *seq {
	s.lastOrderSeq = new(uint64)
	s.lastTradeSeq = new(uint64)
	s.prevOrderSeq = new(uint64)
	s.prevTradeSeq = new(uint64)

	var lastTradeSeq uint64
	if err := s.repo.Conn.Raw("select last_value from trades_id_seq").Row().Scan(&lastTradeSeq); err != nil {
		log.Error().Err(err).Msg("Unable to load last trade id")
	} else {
		atomic.StoreUint64(s.lastTradeSeq, lastTradeSeq)
		atomic.StoreUint64(s.prevTradeSeq, lastTradeSeq)
	}

	var lastOrderSeq uint64
	if err := s.repo.Conn.Raw("select last_value from orders_id_seq").Row().Scan(&lastOrderSeq); err != nil {
		log.Error().Err(err).Msg("Unable to load last order id")
	} else {
		atomic.StoreUint64(s.lastOrderSeq, lastOrderSeq)
		atomic.StoreUint64(s.prevOrderSeq, lastOrderSeq)
	}

	gostop.GetInstance().Go("oms_seq_refresher", s.refresher, true)

	return s
}

func (s *seq) refresher(ctx context.Context, wait *sync.WaitGroup) {
	log.Info().Str("cron", "oms_seq_refresher").Str("action", "start").Msg("OMS Sequence refresher - started")
	tick := time.NewTicker(time.Second)

	for {
		select {
		case <-ctx.Done():
			tick.Stop()
			log.Info().Str("cron", "oms_seq_refresher").Str("action", "stop").Msg("13 => OMS Sequence refresher - stopped")
			s.updateSeqIds()
			wait.Done()
			return
		case <-tick.C:
			s.updateSeqIds()
		}
	}
}

func (s *seq) updateSeqIds() {
	currentOrderSeq := atomic.LoadUint64(s.lastOrderSeq)
	currentTradeSeq := atomic.LoadUint64(s.lastTradeSeq)
	prevOrderSeq := atomic.LoadUint64(s.prevOrderSeq)
	prevTradeSeq := atomic.LoadUint64(s.prevTradeSeq)

	tx := s.repo.Conn.Begin()
	var changed bool

	if currentOrderSeq > prevOrderSeq {
		if err := tx.Exec(fmt.Sprintf("SELECT setval('orders_id_seq', %d, true);", currentOrderSeq)).Error; err != nil {
			log.Error().Err(err).Msg("error in updating orders_id_seq in db")
		}
		log.Debug().Uint64("prevSeq", prevOrderSeq).Uint64("currSeq", currentOrderSeq).Msg("Store last order seq")
		atomic.StoreUint64(s.prevOrderSeq, currentOrderSeq)
		changed = true
	}

	if currentTradeSeq > prevTradeSeq {
		if err := tx.Exec(fmt.Sprintf("SELECT setval('trades_id_seq', %d, true);", currentTradeSeq)).Error; err != nil {
			log.Error().Err(err).Msg("error in updating trades_id_seq in db")
		}
		log.Debug().Uint64("prevSeq", prevTradeSeq).Uint64("currSeq", currentTradeSeq).Msg("Store last trade seq")
		atomic.StoreUint64(s.prevTradeSeq, currentTradeSeq)
		changed = true
	}
	if changed {
		tx.Commit()
	} else {
		tx.Rollback()
	}
}

func (s *seq) NextOrderID() uint64 {
	return atomic.AddUint64(s.lastOrderSeq, 1)
}

func (s *seq) NextTradeID() uint64 {
	return atomic.AddUint64(s.lastTradeSeq, 1)
}

func (s *seq) GetLastTradeID() uint64 {
	return atomic.LoadUint64(s.lastTradeSeq)
}

func (s *seq) GetLastOrderID() uint64 {
	return atomic.LoadUint64(s.lastOrderSeq)
}

func (s *seq) SetLastTradeID(tradeID uint64) {
	atomic.StoreUint64(s.lastTradeSeq, tradeID)
}

func (s *seq) SetLastOrderID(orderID uint64) {
	atomic.StoreUint64(s.lastOrderSeq, orderID)
}
