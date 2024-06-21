package market_engine

import (
	"context"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/data"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/engine"
	net "gitlab.com/paramountdax-exchange/exchange_api_v2/net/kafka"

	"github.com/segmentio/kafka-go"
)

var (
	engineOrderCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "engine_order_count",
		Help: "Trading engine order count",
	}, []string{
		// Which market are the orders from?
		"market",
	})
	engineEventCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "engine_trade_count",
		Help: "Trading engine trade count",
	}, []string{
		// Which market are the events from?
		"market",
	})
	messagesQueued = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "engine_message_queue_count",
		Help: "Number of messages from Apache Kafka received and waiting to be processed.",
	}, []string{
		// Which market are the orders from?
		"market",
	})
	ordersQueued = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "engine_order_queue_count",
		Help: "Number of orders waiting to be processed.",
	}, []string{
		// Which market are the orders from?
		"market",
	})
	eventsQueued = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "engine_trade_queue_count",
		Help: "Number of events waiting to be processed.",
	}, []string{
		// Which market are the events from?
		"market",
	})
)

func init() {
	prometheus.MustRegister(engineOrderCount)
	prometheus.MustRegister(engineEventCount)
	prometheus.MustRegister(messagesQueued)
	prometheus.MustRegister(ordersQueued)
	prometheus.MustRegister(eventsQueued)
}

// MarketEngine defines how we can communicate to the trading engine for a specific market
type MarketEngine interface {
	Start(context.Context, *sync.WaitGroup)
	Close()
	LoadFromOrders(marketID string, orders []data.Order, eventSeqID, tradeSeqID uint64) error
	Process(data.Order)
	GetOutput() <-chan []data.Event
	SetOutput(chan []data.Event)
	GetProducer() net.KafkaProducer
}

// marketEngine structure
type marketEngine struct {
	name              string
	engine            engine.TradingEngine
	orders            chan engine.Event
	events            chan engine.Event
	output            chan []data.Event
	waitOrders        *sync.WaitGroup
	waitEvents        *sync.WaitGroup
	waitSaver         *sync.WaitGroup
	initialEventSeqID uint64
	initialTradeSeqID uint64
	dumpRequest       chan struct{}
	dump              chan string
	// sync     *sync.RWMutex
	producer   net.KafkaProducer
	closeSaver context.CancelFunc
	saver      *SequenceSaver
}

// NewMarketEngine open a new market
func NewMarketEngine(marketID string, producer net.KafkaProducer, pricePrec, volPrec int) MarketEngine {
	saver := NewSequenceSaver(SequenceSaverConfig{
		MarketID: marketID,
		Interval: 1,
		Storage:  "redis",
	})
	return &marketEngine{
		producer:          producer,
		name:              marketID,
		engine:            engine.NewTradingEngine(marketID, pricePrec, volPrec),
		orders:            make(chan engine.Event, 20000),
		events:            make(chan engine.Event, 20000),
		output:            make(chan []data.Event, 20000),
		dumpRequest:       make(chan struct{}, 1),
		dump:              make(chan string, 1),
		initialEventSeqID: 0,
		initialTradeSeqID: 0,
		waitOrders:        &sync.WaitGroup{},
		waitEvents:        &sync.WaitGroup{},
		waitSaver:         &sync.WaitGroup{},
		saver:             saver,
	}
}

func (mkt *marketEngine) GetProducer() net.KafkaProducer {
	return mkt.producer
}

// Start the engine for this market
func (mkt *marketEngine) Start(ctx context.Context, wait *sync.WaitGroup) {
	if err := mkt.producer.Start(context.Background()); err != nil {
		log.Fatal().Err(err).Str("section", "matching_engine").Str("action", "start_producer").Str("market", mkt.name).Msg("Unable to start producer")
	}
	// process each order by the trading engine and forward events to the events channel
	mkt.waitOrders.Add(1)
	go mkt.ProcessOrder(mkt.waitOrders)
	// publish events to the kafka server
	mkt.waitEvents.Add(1)
	go mkt.PublishEvents(mkt.waitEvents)
	mkt.waitSaver.Add(1)
	saverCtx, closeSaver := context.WithCancel(context.Background())
	mkt.closeSaver = closeSaver
	go mkt.saver.StartPersistLoop(saverCtx, mkt.waitSaver)
	go mkt.waitForCloseContext(ctx, wait)
}

func (mkt *marketEngine) GetDump() string {
	mkt.dumpRequest <- struct{}{}
	dump := <-mkt.dump
	return dump
}

// SetOutput return a channel on which to listen to generated events by the market engine
func (mkt *marketEngine) SetOutput(output chan []data.Event) {
	mkt.output = output
}

// GetOutput return a channel on which to listen to generated events by the market engine
func (mkt *marketEngine) GetOutput() <-chan []data.Event {
	return mkt.output
}

// LoadFromOrders Load from orders
func (mkt *marketEngine) LoadFromOrders(marketID string, orders []data.Order, eventSeqID, tradeSeqID uint64) error {
	log.Debug().Str("section", "matching_engine").Str("action", "LoadFromOrders").Str("market", mkt.name).Msg("Prepopulate order book from a list of orders")
	err := mkt.saver.Load()
	if err != nil {
		log.Error().Err(err).Str("section", "matching_engine").Str("action", "LoadFromOrders").Str("market", mkt.name).
			Uint64("event_seq_id", eventSeqID).
			Uint64("trade_seq_id", tradeSeqID).
			Msg("Unable to load saved sequences from Redis... using values from Kafka")
	}
	savedEventSeqID, savedTradeSeqID := mkt.saver.Get()
	if int64(eventSeqID) < savedEventSeqID {
		log.Warn().Err(err).Str("section", "matching_engine").Str("action", "LoadFromOrders").Str("market", mkt.name).
			Uint64("event_seq_id", eventSeqID).
			Int64("redis_event_seq_id", savedEventSeqID).
			Msg("Using event seq id from Redis because it's higher")
		eventSeqID = uint64(savedEventSeqID)
	}
	if int64(tradeSeqID) < savedTradeSeqID {
		log.Warn().Err(err).Str("section", "matching_engine").Str("action", "LoadFromOrders").Str("market", mkt.name).
			Uint64("trade_seq_id", eventSeqID).
			Int64("redis_trade_seq_id", savedTradeSeqID).
			Msg("Using trade seq id from Redis because it's higher")
		tradeSeqID = uint64(savedTradeSeqID)
	}
	_ = mkt.engine.LoadMarket(marketID, orders, eventSeqID, tradeSeqID)
	mkt.initialEventSeqID = eventSeqID
	mkt.initialTradeSeqID = tradeSeqID
	log.Debug().Str("section", "matching_engine").Str("action", "LoadFromOrders").Str("market", mkt.name).Msg("Prepopulate order book completed")
	return nil
}

// Process a new message from the consumer
func (mkt *marketEngine) Process(order data.Order) {
	// Monitor: Increment the number of messages that has been received by the market
	ordersQueued.WithLabelValues(mkt.name).Inc()
	mkt.orders <- engine.Event{Order: order}
}

func (mkt *marketEngine) waitForCloseContext(ctx context.Context, wait *sync.WaitGroup) {
	// wait for the context to be closed before closing the market
	<-ctx.Done()
	// close each channel and wait for all to finish processing
	mkt.Close()
	// signal that the market engine was closed
	wait.Done()
}

// Close the market by closing all communication channels
func (mkt *marketEngine) Close() {
	close(mkt.orders)
	mkt.waitOrders.Wait()
	close(mkt.events)
	mkt.waitEvents.Wait()
	mkt.closeSaver()
	mkt.waitSaver.Wait()
}

// ProcessOrder process each order by the trading engine and forward events to the events channel
//
// Message flow is unidirectional from the orders channel to the events channel
func (mkt *marketEngine) ProcessOrder(wait *sync.WaitGroup) {
	log.Info().Str("worker", "engine_process_order").Str("action", "start").Str("market", mkt.name).Msg("Engine order processor - started")

	for {
		select {
		case event, more := <-mkt.orders:
			{
				if !more {
					log.Info().Str("worker", "engine_process_order").Str("action", "stop").Str("market", mkt.name).Msg("Engine order processor - stopped")
					wait.Done()
					return
				}
				order := event.Order
				log.Debug().
					Str("section", "matching_engine").Str("action", "process_order").
					Str("market", mkt.name).
					Dict("event", zerolog.Dict().
						Str("event_type", order.EventType.String()).
						Str("side", order.Side.String()).
						Str("type", order.Type.String()).
						Str("event_type", order.EventType.String()).
						Str("market", order.Market).
						Uint64("id", order.ID).
						Uint64("amount", order.Amount).
						Str("stop", order.Stop.String()).
						Uint64("stop_price", order.StopPrice).
						Uint64("funds", order.Funds).
						Uint64("price", order.Price),
					).
					Msg("New order")

				if !order.Valid() {
					log.Warn().
						Str("section", "matching_engine").Str("action", "process_order").
						Str("market", mkt.name).
						Dict("event", zerolog.Dict().
							Str("event_type", order.EventType.String()).
							Str("side", order.Side.String()).
							Str("type", order.Type.String()).
							Str("event_type", order.EventType.String()).
							Str("market", order.Market).
							Uint64("id", order.ID).
							Uint64("amount", order.Amount).
							Str("stop", order.Stop.String()).
							Uint64("stop_price", order.StopPrice).
							Uint64("funds", order.Funds).
							Uint64("price", order.Price),
						).
						Msg("Invalid order received, ignoring")
					// send invalid notification
					events := make([]data.Event, 0, 1)
					mkt.engine.AppendInvalidOrder(order, &events)
					event.SetEvents(events)

					// Monitor: Update order count for monitoring with prometheus
					engineOrderCount.WithLabelValues(mkt.name).Inc()
					ordersQueued.WithLabelValues(mkt.name).Dec()
					eventsQueued.WithLabelValues(mkt.name).Add(float64(len(event.Events)))
					// send generated events for storage
					mkt.output <- event.Events
					mkt.events <- event
					continue
				}
				events := make([]data.Event, 0, 5)
				// Process each order and generate events
				mkt.engine.ProcessEvent(event.Order, &events)
				event.SetEvents(events)
				// Monitor: Update order count for monitoring with prometheus
				engineOrderCount.WithLabelValues(mkt.name).Inc()
				ordersQueued.WithLabelValues(mkt.name).Dec()
				eventsQueued.WithLabelValues(mkt.name).Add(float64(len(event.Events)))
				// send generated events for storage
				mkt.output <- event.Events
				mkt.events <- event
			}
		case <-mkt.dumpRequest:
			{
				exp := ""
				mkt.engine.GetOrderBook()
				mkt.dump <- exp
			}
		}
	}
}

// PublishEvents listens for new events from the trading engine and publishes them to the Kafka server
func (mkt *marketEngine) PublishEvents(wait *sync.WaitGroup) {
	log.Info().Str("worker", "engine_process_events").Str("action", "start").Str("market", mkt.name).Msg("Engine events processor - started")

	var lastAskID uint64
	var lastBidID uint64

	var lastEventSeqID uint64
	var lastTradeSeqID uint64

	lastEventSeqID = mkt.initialEventSeqID
	lastTradeSeqID = mkt.initialTradeSeqID

	for event := range mkt.events {
		events := make([]kafka.Message, len(event.Events))
		for index, ev := range event.Events {
			logEvent := zerolog.Dict()
			switch ev.Type {
			case data.EventType_OrderStatusChange:
				{
					payload := ev.GetOrderStatus()
					logEvent = logEvent.
						Uint64("id", payload.ID).
						Uint64("owner_id", payload.OwnerID).
						Str("type", payload.Type.String()).
						Str("side", payload.Side.String()).
						Str("status", payload.Status.String()).
						Uint64("price", payload.Price).
						Uint64("funds", payload.Funds).
						Uint64("amount", payload.Amount)
				}
			case data.EventType_OrderActivated:
				{
					payload := ev.GetOrderActivation()
					logEvent = logEvent.
						Uint64("id", payload.ID).
						Uint64("owner_id", payload.OwnerID).
						Str("type", payload.Type.String()).
						Str("side", payload.Side.String()).
						Str("status", payload.Status.String()).
						Uint64("price", payload.Price).
						Uint64("funds", payload.Funds).
						Uint64("amount", payload.Amount)
				}
			case data.EventType_Error:
				{
					payload := ev.GetError()
					logEvent = logEvent.
						Uint64("order_id", payload.OrderID).
						Uint64("owner_id", payload.OwnerID).
						Str("type", payload.Type.String()).
						Str("side", payload.Side.String()).
						Str("err_code", payload.Code.String()).
						Uint64("price", payload.Price).
						Uint64("funds", payload.Funds).
						Uint64("amount", payload.Amount)
				}
			case data.EventType_NewTrade:
				{
					trade := ev.GetTrade()
					logEvent = logEvent.
						Uint64("seqid", trade.SeqID).
						Str("taker_side", trade.TakerSide.String()).
						Uint64("ask_id", trade.AskID).
						Uint64("ask_owner_id", trade.AskOwnerID).
						Uint64("bid_id", trade.BidID).
						Uint64("bid_owner_id", trade.BidOwnerID).
						Uint64("price", trade.Price).
						Uint64("amount", trade.Amount)
					if lastAskID == trade.AskID && lastBidID == trade.BidID {
						log.Error().Str("section", "matching_engine").Str("action", "post:trade:check").
							Str("market", mkt.name).
							Uint64("last_order_id", event.Order.ID).
							Str("event_type", ev.Type.String()).
							Uint64("event_seqid", ev.SeqID).
							Int64("event_timestamp", ev.CreatedAt).
							Dict("event", logEvent).
							Msg("An bid order matched with the same sell order twice. Orderbook is in an inconsistent state.")
					}
					lastBidID = trade.BidID
					lastAskID = trade.AskID
					lastTradeSeqID = trade.SeqID
				}
			}
			log.Debug().Str("section", "matching_engine").Str("action", "publish").
				Str("market", mkt.name).
				Uint64("last_order_id", event.Order.ID).
				Str("event_type", ev.Type.String()).
				Uint64("event_seqid", ev.SeqID).
				Int64("event_timestamp", ev.CreatedAt).
				Dict("event", logEvent).
				Msg("Generated event")
			rawTrade, _ := ev.ToBinary() // @todo add better error handling on encoding
			events[index] = kafka.Message{
				Value: rawTrade,
			}
			lastEventSeqID = ev.SeqID
		}
		//err := mkt.producer.WriteMessages(context.Background(), events...)
		//if err != nil {
		//	log.Error().Err(err).Str("section", "matching_engine").Str("action", "publish").Str("market", mkt.name).Msg("Unable to publish events")
		//}
		mkt.saver.Set(int64(lastEventSeqID), int64(lastTradeSeqID))

		// Monitor: Update the number of events processed after sending them back to Kafka
		eventCount := float64(len(event.Events))
		eventsQueued.WithLabelValues(mkt.name).Sub(eventCount)
		engineEventCount.WithLabelValues(mkt.name).Add(eventCount)
	}
	log.Info().Str("worker", "engine_process_events").Str("action", "stop").Str("market", mkt.name).Msg("Engine events processor - stopped")
	wait.Done()
}
