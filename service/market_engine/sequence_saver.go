package market_engine

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/net/redis"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var redisStorage *redis.Client

func InitSequenceStorage(cfg redis.Config) error {
	redisStorage = redis.NewClient(cfg)
	return redisStorage.Connect()
}

func DisconnectSequenceStorage() error {
	if redisStorage != nil {
		return redisStorage.Disconnect()
	}
	return nil
}

type SequenceSaverConfig struct {
	MarketID string `mapstructure:"market_id"`
	Interval int64  `mapstructure:"interval"` // in seconds
	Storage  string `mapstructure:"storage"`
}

type SequenceSaver struct {
	cfg        SequenceSaverConfig
	eventSeqID *int64
	tradeSeqID *int64
}

func NewSequenceSaver(cfg SequenceSaverConfig) *SequenceSaver {
	return &SequenceSaver{
		cfg:        cfg,
		eventSeqID: new(int64),
		tradeSeqID: new(int64),
	}
}

func (s *SequenceSaver) EventSeqIdKey() string {
	return fmt.Sprintf("seqid__%s__%s", s.cfg.MarketID, "event")
}

func (s *SequenceSaver) TradeSeqIdKey() string {
	return fmt.Sprintf("seqid__%s__%s", s.cfg.MarketID, "trade")
}

func (s *SequenceSaver) Save() error {
	eventSeqID, tradeSeqID := s.Get()
	err := redisStorage.Exec(nil, "SET", s.EventSeqIdKey(), eventSeqID)
	if err != nil {
		return err
	}
	err = redisStorage.Exec(nil, "SET", s.TradeSeqIdKey(), tradeSeqID)
	return err
}

func (s *SequenceSaver) Load() error {
	var eventSeqIDStr, tradeSeqIDStr string
	if redisStorage == nil {
		log.Fatal().Msg("Sequence saver not connected to Redis")
	}
	err := redisStorage.Exec(&eventSeqIDStr, "GET", s.EventSeqIdKey())
	if err != nil {
		return err
	}
	err = redisStorage.Exec(&tradeSeqIDStr, "GET", s.EventSeqIdKey())
	if err != nil {
		return err
	}

	eventSeqID, err := strconv.Atoi(eventSeqIDStr)
	if err != nil {
		return err
	}
	tradeSeqID, err := strconv.Atoi(tradeSeqIDStr)
	if err != nil {
		return err
	}
	s.Set(int64(eventSeqID), int64(tradeSeqID))
	return nil
}

func (s *SequenceSaver) StartPersistLoop(ctx context.Context, w *sync.WaitGroup) {
	log.Info().Str("worker", "engine_sequence_saver").Str("action", "start").Str("market", s.cfg.MarketID).Msg("Sequence saver - started")
	ticker := time.NewTicker(time.Duration(s.cfg.Interval) * time.Second)
	for {
		select {
		case <-ticker.C:
			err := s.Save()
			if err != nil {
				log.Error().Err(err).Str("worker", "engine_sequence_saver").Str("action", "save").Str("market", s.cfg.MarketID).
					Int64("event_seq_id", s.GetEventSeqID()).
					Int64("trade_seq_id", s.GetTradeSeqID()).
					Msg("Failed to save trade sequences")
			}
		case <-ctx.Done():
			ticker.Stop()
			err := s.Save()
			if err != nil {
				log.Error().Err(err).Str("worker", "engine_sequence_saver").Str("action", "save").Str("market", s.cfg.MarketID).
					Int64("event_seq_id", s.GetEventSeqID()).
					Int64("trade_seq_id", s.GetTradeSeqID()).
					Msg("Failed to save trade sequences")
			}
			log.Info().Str("worker", "engine_sequence_saver").Str("action", "stop").Str("market", s.cfg.MarketID).Msg("Sequence saver - stopped")
			w.Done()
		}
	}
}

func (s *SequenceSaver) Set(eventSeqID, tradeSeqID int64) {
	atomic.StoreInt64(s.eventSeqID, eventSeqID)
	atomic.StoreInt64(s.tradeSeqID, tradeSeqID)
}

func (s *SequenceSaver) Get() (int64, int64) {
	return atomic.LoadInt64(s.eventSeqID), atomic.LoadInt64(s.tradeSeqID)
}

func (s *SequenceSaver) GetTradeSeqID() int64 {
	return atomic.LoadInt64(s.tradeSeqID)
}

func (s *SequenceSaver) SetTradeSeqID(seqId int64) {
	atomic.StoreInt64(s.tradeSeqID, seqId)
}

func (s *SequenceSaver) GetEventSeqID() int64 {
	return atomic.LoadInt64(s.eventSeqID)
}

func (s *SequenceSaver) SetEventSeqID(seqId int64) {
	atomic.StoreInt64(s.eventSeqID, seqId)
}
