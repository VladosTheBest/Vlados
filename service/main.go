package service

import (
	"context"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/datasync"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/gostop"
	"strings"
	"sync"
	"time"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/otc_desk"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/solaris"

	"github.com/Unleash/unleash-client-go/v3"
	"github.com/centrifugal/centrifuge"
	"github.com/rs/zerolog/log"
	kafkaGo "github.com/segmentio/kafka-go"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/apps/coinsLocal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/apps/wallet"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/cancelConfirmation"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/markets"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/userbalance"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/config"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/crons"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/data"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/data/referral_earnings"
	loader "gitlab.com/paramountdax-exchange/exchange_api_v2/lastprices"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/sendgrid"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/net/kafka"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/net/manager"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/ops"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/fms"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/manage_token"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/market_engine"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/oms"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/order_queues"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/payments/clear_junction"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/two_fa_recovery"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/trades"
)

// Service structure
type Service struct {
	apiConfig            config.APIConfig
	adminConfig          config.AdminConfig
	dm                   *manager.DataManager
	coinValues           *coinsLocal.App
	walletApp            *wallet.App
	repo                 *queries.Repo
	sendgrid             sendgrid.Sendgrid
	kyc                  config.KYCConfig
	ctx                  context.Context
	cfg                  config.Config
	ops                  *ops.Ops
	tradesApp            *trades.App
	eventsInput          chan []data.Event
	WsNode               *centrifuge.Node
	notificationChan     chan<- *model.NotificationWithTotalUnread
	pushNotificationChan chan<- *model.Notification
	ClearJunction        *clear_junction.ClearJunctionProcessor
	OMS                  *oms.OMS
	FundsEngine          *fms.FundsEngine
	kybConfig            config.KYBConfig
	otcDesk              *otc_desk.OtcDesk
	SolarisSession       *solaris.AccessToken

	Markets map[string]market_engine.MarketEngine
}

func getLatestIDs(cfg config.Config, client kafka.KafkaConsumer, fromOffset int64) (uint64, uint64) {
	log.Info().Int64("fromOffset", fromOffset).Msg("Getting latest id... setting offset and reading lag")
	defaultLag := cfg.Engine.IDsOffset
	_ = client.SetOffset(fromOffset)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	lastOrderID := uint64(0)
	lastTradeID := uint64(0)
	lag, err := client.ReadLag(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Unable to read kafka lag")
		return lastOrderID, lastTradeID
	}

	log.Info().Int64("lag", lag).Msg("read kafka lag")

	if lag > defaultLag {
		_ = client.SetOffset(fromOffset + lag - defaultLag)
		lag, err = client.ReadLag(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Unable to read kafka lag")
			return lastOrderID, lastTradeID
		}
	}

	log.Info().Int64("lag", lag).Msg("Search latest kafka messages for trades and orders")
	for i := int64(0); i < lag; i++ {
		readCtx, _ := context.WithTimeout(context.Background(), 2*time.Second)
		msg, err := client.ReadMessage(readCtx)
		if err != nil {
			log.Error().Err(err).Msg("Unable to read kafka event")
			if err.Error() == "context deadline exceeded" {
				break
			}
			continue
		}
		event := data.SyncEvent{}
		if err := event.FromBinary(msg.Value); err != nil {
			log.Error().Err(err).Msg("Unable to parse event")
			continue
		}
		if event.Model == "orders" {
			payload := event.GetPayload()
			id, ok := payload["id"]
			if ok {
				lastOrderID = id.GetUint64Value()
			}
		}
		if event.Model == "trades" {
			payload := event.GetPayload()
			id, ok := payload["id"]
			if ok {
				lastTradeID = id.GetUint64Value()
			}
		}
	}

	log.Info().
		Uint64("lastOrderID", lastOrderID).
		Uint64("lastTradeID", lastTradeID).
		Msg("latest order and trade ids from kafka")

	return lastOrderID, lastTradeID
}

func getLastKafkaOrderAndTradeIds(cfg config.Config) (uint64, uint64) {
	var orderID, tradeID uint64
	topic := formatTopic(cfg.Preprocessors.QueueManager.Patterns["sync_data"], map[string]string{})
	client, err := kafka.NewKafkaConsumer(cfg.Kafka.Reader, cfg.Kafka.Brokers, cfg.Kafka.UseTLS, topic, 0)
	if err != nil {
		log.Error().Err(err).
			Uint64("lastOrderID", orderID).
			Uint64("lastTradeID", tradeID).
			Msg("starting kafka consumer")
		return orderID, tradeID
	}

	log.Info().
		Str("topic", topic).
		Msg("Loading kafka data for topic")
	// offset := int64(0)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	lag, err := client.ReadLag(ctx)
	offset := client.GetOffset()

	log.Info().
		Str("topic", topic).
		Int64("lag", lag).
		Int64("offset", offset).
		Msg("Read kafka lag")

	historyOffset := cfg.Engine.IDsOffset

	startOffset := offset
	if startOffset < 0 && lag <= historyOffset {
		startOffset = 0
	} else if startOffset < 0 && lag > historyOffset {
		startOffset = lag - historyOffset
	} else {
		startOffset = offset - historyOffset // look for trades and orders within the last 10000 events
		if startOffset < 0 {
			startOffset = 0
		}
	}

	log.Info().
		Str("topic", topic).
		Int64("startOffset", startOffset).
		Msg("Get latest ids")
	orderID, tradeID = getLatestIDs(cfg, client, startOffset)

	client.Close()

	log.Error().
		Err(err).
		Int64("offset", offset).
		Uint64("orderID", orderID).
		Uint64("tradeID", tradeID).
		Int64("lag", lag).
		Str("topic", topic).
		Msg("latest trades and order ids")
	return orderID, tradeID
}

func getLastKafkaEventIds(marketID string, cfg config.Config) (uint64, uint64, error) {
	topic := formatTopic(cfg.Preprocessors.QueueManager.Patterns["events"], map[string]string{
		"market": marketID,
	})
	client, err := kafka.NewKafkaConsumer(cfg.Kafka.Reader, cfg.Kafka.Brokers, cfg.Kafka.UseTLS, topic, 0)
	if err != nil {
		panic(err)
	}

	log.Info().
		Str("topic", topic).
		Msg("Loading kafka data for topic")
	historyOffset := cfg.Engine.SeqIDOffset
	if historyOffset <= 0 {
		historyOffset = 10000 // default value
	}

	firstOffset, offset, lag, err := client.TraverseGetLastOffsetAndLag()
	// exit if no messages are available in the topic
	if err == nil && offset == kafkaGo.FirstOffset && lag == 0 {
		return 0, 0, nil
	}
	eventSeqID, tradeSeqID := uint64(0), uint64(0)
	toOffset := offset
	fromOffset := offset - historyOffset
	if fromOffset < 0 {
		fromOffset = kafkaGo.FirstOffset
	}
	for {
		log.Debug().
			Err(err).
			Str("topic", topic).
			Int64("firstOffset", firstOffset).
			Int64("historyOffset", historyOffset).
			Int64("offset", offset).
			Int64("toOffset", toOffset).
			Int64("lag", lag).
			Msg("Loaded offsets and lag from kafka")
		err = client.Traverse(fromOffset, toOffset, func(msg kafkaGo.Message) (bool, error) {
			event := data.Event{}
			if err := event.FromBinary(msg.Value); err != nil {
				return true, err
			}
			if event.SeqID > eventSeqID {
				eventSeqID = event.SeqID
			}
			if event.Type == data.EventType_NewTrade {
				tradeID := event.GetTrade().SeqID
				if tradeID > tradeSeqID {
					tradeSeqID = tradeID
				}
			}
			return false, nil
		})
		if err == nil && tradeSeqID == 0 && fromOffset > firstOffset {
			toOffset = fromOffset
			fromOffset = fromOffset - historyOffset
			if fromOffset < firstOffset {
				fromOffset = firstOffset
			}
			continue
		}
		break
	}

	if err != nil {
		return eventSeqID, tradeSeqID, err
	}

	//log.Info().
	//	Str("topic", topic).
	//	Int64("lag", lag).
	//	Int64("offset", offset).
	//	Msg("Read kafka lag")
	//
	//startOffset := offset
	//if startOffset < 0 && lag <= historyOffset {
	//	startOffset = 0
	//} else if startOffset < 0 && lag > historyOffset {
	//	startOffset = lag - historyOffset
	//} else {
	//	startOffset = offset - historyOffset // look for trades within the last 10000 events
	//	if startOffset < 0 {
	//		startOffset = 0
	//	}
	//}

	//log.Info().
	//	Str("topic", topic).
	//	Int64("startOffset", startOffset).
	//	Msg("Get latest seq id")
	//eventSeqID, tradeSeqID := getLatestSeqID(client, marketID, startOffset)

	err = client.Close()
	if err != nil {
		return eventSeqID, tradeSeqID, err
	}

	log.Error().
		Err(err).
		Int64("offset", offset).
		Uint64("eventSeqID", eventSeqID).
		Uint64("tradeSeqID", tradeSeqID).
		Int64("lag", lag).
		Str("topic", topic).
		Msg("Kafka offset for market")
	return eventSeqID, tradeSeqID, nil
}

// NewService constructor
func NewService(
	ctx context.Context,
	cfg config.Config,
	dm *manager.DataManager,
	depositChan chan<- model.Transaction,
	withdrawRequestChan chan<- model.WithdrawRequest,
	notificationChan chan<- *model.NotificationWithTotalUnread,
	pushNotificationChan chan<- *model.Notification,
) *Service {
	// connect to the database
	repo := queries.NewRepo(
		cfg.DatabaseCluster.Writer,
		cfg.DatabaseCluster.Reader,
		cfg.DatabaseCluster.ReaderAdmin,
	)
	order_queues.SetRepo(repo)
	loader.NewLoader(cfg.Server.Candles.URL)
	// wait for data to be synced in the data source before continuing:
	datasync.WaitUntilSynced(cfg.Datasync.Url)

	coinValues := coinsLocal.NewApp(cfg.CoinValuesAPI.UrlCoinValues, cfg.CoinValuesAPI.UrlLastPrices, ctx)
	gostop.GetInstance().Go("coins_local_app", coinValues.Start, true)
	wireEncoder := data.NewWireEncoder("wire")
	omsInstance := oms.Init(repo, dm, wireEncoder, ctx)
	fmsInstance := fms.Init(repo, ctx)

	if err := fmsInstance.InitAccounts(); err != nil {
		log.Fatal().Err(err).Str("section", "FMS").Msg("Unable to init user balances")
	} else {
		log.Warn().Str("section", "FMS").Msg("User balances loaded successfully")
	}

	op := ops.New(repo, omsInstance, fmsInstance)

	order_queues.SetFundsEngine(fmsInstance)
	order_queues.SetCurrentRiskLevel(cfg.BonusAccount.GetRiskLevel())
	order_queues.SetOps(op)
	order_queues.SetCoinsRates(coinValues)

	cj := clear_junction.Init(cfg.ClearJunction.ApiKey, cfg.ClearJunction.ApiPassword, cfg.ClearJunction.ApiUrl, cfg.ClearJunction.WalletUUID)

	// get latest order and trade ids from kafka
	lastOrderID, lastTradeID := getLastKafkaOrderAndTradeIds(cfg)
	currentLastOrderID := omsInstance.Seq.GetLastOrderID()
	currentLastTradeID := omsInstance.Seq.GetLastTradeID()

	if lastOrderID > currentLastOrderID {
		omsInstance.Seq.SetLastOrderID(lastOrderID)
	}
	if lastTradeID > currentLastTradeID {
		omsInstance.Seq.SetLastTradeID(lastTradeID)
	}

	// load active markets and start the matching engine for each of them
	// preload all open orders and populate them in the ME as needed
	marketEnginesByID := make(map[string]market_engine.MarketEngine)
	markets, err := repo.GetMarketsByStatus(model.MarketStatusActive)
	if err != nil {
		log.Fatal().Err(err).Str("section", "matching_engine").Msg("Unable to init active markets from the db")
		return nil
	}

	for _, market := range markets {
		log.Info().Str("section", "matching_engine").Str("market", market.ID).Interface("config", cfg.Kafka).Msg("Initializing matching engine for market")

		// get last trade & event seq id
		eventSeqID, tradeSeqID, err := getLastKafkaEventIds(market.ID, cfg)
		if err != nil {
			log.Fatal().Err(err).Str("section", "matching_engine").Str("market", market.ID).Msg("Unable to load last trade seq id and event seq id from Kafka")
		}

		log.Info().Str("section", "matching_engine").Str("market", market.ID).Uint64("event_seq", eventSeqID).Uint64("trade_seq", tradeSeqID).Msg("Started matching engine from these sequence ids")

		// start the market engine
		marketEnginesByID[market.ID] = market_engine.NewMarketEngine(
			market.ID,
			kafka.NewKafkaProducer(
				cfg.Kafka.Writer,
				cfg.Kafka.Brokers,
				cfg.Kafka.UseTLS,
				formatTopic(cfg.Preprocessors.QueueManager.Patterns["events"], map[string]string{
					"market": market.ID,
				}),
			),
			market.QuotePrecision,
			market.MarketPrecision,
		)
		orders, _ := omsInstance.GetOrdersByMarketID(market.ID)
		log.Info().
			Str("section", "matching_engine").
			Str("market", market.ID).
			Int("order_count", len(orders)).
			Msg("Loading active orders in matching engine")
		events := []data.Order{}
		for _, o := range orders {
			order, err := GetDataOrderFromModel(o, market)
			if err != nil {
				log.Fatal().Err(err).Str("section", "matching_engine").Str("market", market.ID).Msg("Error converting order")
				return nil
			}
			events = append(events, order)
		}
		// dataOrders := []data.Order{}
		// for _, order := range orders {
		// 	dataOrder := ConvertOrderFromModelToData(order, uint8(market.QuotePrecision), uint8(market.MarketPrecision))
		// 	dataOrders = append(dataOrders, dataOrder)
		// }
		err = marketEnginesByID[market.ID].LoadFromOrders(market.ID, events, eventSeqID, tradeSeqID)
		// err = marketEnginesByID[market.ID].LoadFromOrders(dataOrders)
		if err != nil {
			log.Fatal().Err(err).Str("section", "matching_engine").Msg("Unable to prepopulate the matching engine from database orders")
			return nil
		}
	}

	eventsInput := make(chan []data.Event, 20000)
	for _, market := range markets {
		marketEnginesByID[market.ID].SetOutput(eventsInput)
	}

	// start cancel confirmation listeners
	for _, market := range markets {
		cancelConfirmation.
			GetInstance().
			InitMarket(market.ID).
			StartReceiveLoop()
	}

	// for _, market := range markets {
	// 	orders, err := repo.GetOrdersByMarketID(market.ID, []model.OrderStatus{
	// 		model.OrderStatus_PartiallyFilled,
	// 		model.OrderStatus_Pending,
	// 		model.OrderStatus_Untouched,
	// 	})
	// 	if err != nil {
	// 		log.Fatal().Err(err).Str("section", "matching_engine").Msg("Unable to init active markets from the db")
	// 		return nil
	// 	}
	// 	dataOrders := []data.Order{}
	// 	for _, order := range orders {
	// 		dataOrder := ConvertOrderFromModelToData(order, uint8(market.QuotePrecision), uint8(market.MarketPrecision))
	// 		dataOrders = append(dataOrders, dataOrder)
	// 	}
	// 	err = marketEnginesByID[market.ID].LoadFromOrders(dataOrders)

	// 	if err != nil {
	// 		log.Fatal().Err(err).Str("section", "matching_engine").Msg("Unable to prepopulate the matching engine from database orders")
	// 		return nil
	// 	}
	// }

	service := &Service{
		apiConfig:   cfg.Server.API,
		adminConfig: cfg.Server.Admin,
		cfg:         cfg,
		dm:          dm,
		eventsInput: eventsInput,
		// configure sendgrid with the from email and list of templates available
		sendgrid:             sendgrid.NewSendgrid(cfg.Server.Sendgrid.Key, cfg.Server.Sendgrid.From, cfg.Server.Sendgrid.Templates),
		kyc:                  cfg.Server.KYC,
		repo:                 repo,
		coinValues:           coinValues,
		walletApp:            wallet.NewApp(repo, op, depositChan, withdrawRequestChan),
		ctx:                  ctx,
		ops:                  op,
		notificationChan:     notificationChan,
		ClearJunction:        cj,
		pushNotificationChan: pushNotificationChan,
		tradesApp:            trades.NewApp(repo, op, coinValues, &cfg.ReferralConfig),
		OMS:                  omsInstance,
		FundsEngine:          fmsInstance,
		SolarisSession:       solaris.NewSolarisSession(),
		otcDesk:              otc_desk.NewOtcDesk(cfg.OtcDeskConfig),
		Markets:              marketEnginesByID,
		kybConfig:            cfg.Server.KYB,
	}

	// init two factor authentication password recover
	two_fa_recovery.Init()
	// start internal cron
	go service.StartCronSystemRevertFrozenOrder()

	// set Data Manager position handler loader
	dm.SetPositionHandler(service.NewPositionHandler())

	err = service.SetupOrderRootIds()
	if err != nil {
		log.Info().Str("section", "service").Str("action", "SetupRootIds").Err(err).Msg("Unable to setup order root ids")
	}

	return service
}

func formatTopic(pattern string, params map[string]string) string {
	for key, val := range params {
		pattern = strings.ReplaceAll(pattern, "${"+key+"}", val)
	}
	return pattern
}

// GetWalletApp godoc
func (s *Service) GetWalletApp() *wallet.App {
	return s.walletApp
}

// GetRepo godoc
func (s *Service) GetRepo() *queries.Repo {
	return s.repo
}

// Start godoc
func (s *Service) Start() {
	// start apps
	log.Debug().Str("section", "service").Str("action", "app:wallet:start").Msg("Starting wallet app")
	_ = s.walletApp.Init()
	log.Debug().Str("section", "service").Str("action", "manage_token:start").Msg("Starting Redis & token service")
	err := manage_token.Start(s.cfg.Redis)
	if err != nil {
		log.Error().Err(err).Str("section", "service").Str("action", "manage_token:start").Msg("Unable to connect to redis")
	}

	for id := range s.Markets {
		gostop.GetInstance().Exec("market_engine", s.Markets[id].Start, false)
	}

	crons.Start(s.cfg.Crons, s.repo, s.coinValues)
	crons.CronUpdateSubAccountsCache()
	crons.CronUpdateUserFeesCache()
	crons.CronUpdateCoinsCache()

	s.tradesApp.Init()

	log.Debug().Str("section", "service").Str("action", "manage_token:start").Msg("Starting Cron service")

	// register app listeners
	s.registerWalletEventListener()
	s.registerBalanceUpdateTriggerEventListener()
	s.registerOrderCancelConfirmationListener()
	s.registerBotEventsListener()

	gostop.GetInstance().Go("event_processor", s.loopEngineEventListenerToTradesApp, false)
	//s.registerEngineEventListener()
	s.registerReferralEarningsListener()
	s.registerOtoOcoOrderUpdateListener()

	go s.BotFixIssueEmptyBotID()
	go s.FillMissedLeadBonuses()
}

// Close godoc
func (s *Service) CloseCrons() {
	crons.Close()
}

func (s *Service) Close() {
	manage_token.Close()
}

func (s *Service) registerWalletEventListener() {
	s.dm.Subscribe("wallet_events", func(msg kafkaGo.Message) error {
		_, err := s.walletApp.Process(msg, s)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *Service) registerBalanceUpdateTriggerEventListener() {
	s.dm.Subscribe("balance_update_trigger", func(msg kafkaGo.Message) error {
		return userbalance.Process(&msg)
	})
}

func (s *Service) registerOrderCancelConfirmationListener() {
	s.dm.Subscribe("order_canceled_confirmation", func(msg kafkaGo.Message) error {
		return cancelConfirmation.Process(&msg)
	})
}

func (s *Service) registerBotEventsListener() {
	s.dm.Subscribe("bot_events", func(msg kafkaGo.Message) error {
		event := data.Bots{}
		err := event.FromBinary(msg.Value)
		if err != nil {
			return err
		}

		bot, err := s.GetBot(uint(event.BotID))
		if err != nil {
			return err
		}

		tx := s.GetRepo().Conn.Begin()
		if err, successfully := s.BotChangeStatus(tx, bot.UserId, bot.ID, model.BotStatusStoppedBySystemTrigger, true); err == nil {
			if successfully {
				if err := tx.Commit().Error; err != nil {
					return err
				}
			}
		} else {
			if !unleash.IsEnabled("api.bots.ignore-kafka-events-response") {
				return err
			}
		}

		return nil
	})
}

func (s *Service) loopEngineEventListenerToTradesApp(ctx context.Context, w *sync.WaitGroup) {
	isClosed := false
	for events := range s.eventsInput {
		for _, event := range events {
			// get the market of the event from the database or cache
			market, err := markets.Get(event.Market)
			if err != nil {
				log.Warn().Err(err).
					Str("section", "queue").
					Str("action", "event:process").
					Str("market_id", event.Market).
					Msg("Unable to find market for event")
				continue
			}
			// apply the event to the engine database and return the result
			err = s.tradesApp.Process(market, &event)
			if err != nil && err == ops.ErrOrderAlreadyCancelled {
				// if the order was already cancelled then skip it
				log.Warn().Err(err).
					Str("section", "queue").
					Str("action", "event:process").
					Str("event", "engine:cancel_order").
					Msg("Unable to cancel order, already cancelled")
			} else if err != nil {
				log.Error().Err(err).
					Str("section", "queue").
					Str("action", "event:process").
					Msg("Unknown error when processing engine event. Skipping")
			}
			// publish the event to kafka for other services to react to it
			err = s.publishEngineEvents(market.ID, events)
			if err != nil {
				log.Error().Err(err).Str("section", "matching_engine").Str("action", "publish").Str("market", market.ID).Msg("Unable to publish events")
			}
			// push events to cancellation notification queue
			cancelConfirmation.
				GetInstance().
				InitMarket(market.ID).GetInputChan() <- events
		}
		// close the channel at shutdown
		if !isClosed {
			select {
			case <-ctx.Done():
				close(s.eventsInput)
				isClosed = true
			default:
			}
		}
	}
	w.Done()
}

func (s *Service) publishEngineEvents(market string, generatedEvents []data.Event) error {
	events := make([]kafkaGo.Message, len(generatedEvents))
	for index, ev := range generatedEvents {
		rawTrade, _ := ev.ToBinary() // @todo add better error handling on encoding
		events[index] = kafkaGo.Message{
			Value: rawTrade,
		}
	}
	err := s.Markets[market].GetProducer().WriteMessages(context.Background(), events...)
	if err != nil {
		return err
	}
	return nil
}

//// Persist every generated trade in the database
//func (s *Service) registerEngineEventListener() {
//	log.Info().
//		Str("section", "service").
//		Str("action", "engine:subscribe").
//		Msg("Subscribing to matching engine events")
//	s.dm.Subscribe("events", func(msg kafkaGo.Message) error {
//		// decode the event from the binary message
//
//		event := data.Event{}
//		if err := event.FromBinary(msg.Value); err != nil {
//			return err
//		}
//
//		// get the market of the event from the database or cache
//		market, err := markets.Get(event.Market)
//		if err != nil {
//			return err
//		}
//		// apply the event to the engine database and return the result
//
//		err = s.tradesApp.Process(market, &event)
//
//		if err != nil && err == ops.ErrOrderAlreadyCancelled {
//			// if the order was already cancelled then skip it
//			log.Warn().Err(err).
//				Str("section", "queue").
//				Str("action", "event:process").
//				Str("event", "engine:cancel_order").
//				Msg("Unable to cancel order, already cancelled")
//			return nil
//		}
//		return err
//	})
//}

func (s *Service) registerReferralEarningsListener() {
	log.Info().
		Str("section", "service").
		Str("action", "registerReferralEarningsListener").
		Msg("Subscribing to matching engine events")
	s.dm.Subscribe("referral_earnings", func(msg kafkaGo.Message) error {
		// decode the event from the binary message
		event := referral_earnings.AddReferralEarningsFromUser{}
		_ = event.FromBinary(msg.Value)

		// apply the event to the engine database and return the result
		return s.tradesApp.ProcessExternalEarnings(&event)
	})
}

func (s *Service) registerOtoOcoOrderUpdateListener() {
	log.Info().
		Str("section", "service").
		Str("action", "registerOtoOcoOrderUpdateListener").
		Msg("Subscribing to oto/oco order update events")
	s.dm.Subscribe("oto_oco_update_order", func(msg kafkaGo.Message) error {
		// decode the event from the binary message
		event := data.Order{}
		_ = event.FromBinary(msg.Value)

		// apply the event to the engine database and return the result

		return s.ProcessOtoOcoOrderUpdateEvent(&event)
	})
}

func (service *Service) GetAllDepositsInUSD(userId int) (model.UserDeposits, error) {

	// Sum of all amount column where tx_type = deposit and fee_coin = eur by user_id
	var deposit model.UserDeposits

	allDeps, err := service.repo.GetAllDepositsById(userId)
	if err != nil {
		log.Info().
			Str("section", "service").
			Str("action", "GetAllDepositsInUSD").
			Str("event", "GetAllDepositsById").
			Err(err)
		return deposit, err
	}
	firstLogin, err := service.repo.GetUserFirstLogin(userId)
	if err != nil {
		log.Info().
			Str("section", "service").
			Str("action", "GetAllDepositsInUSD").
			Str("event", "GetUserFirstLogin").
			Err(err)
		return deposit, err
	}
	// created_at by user_id

	coins, err := service.GetCoinsValue()
	if err != nil {
		log.Info().
			Str("section", "service").
			Str("action", "GetAllDepositsInUSD").
			Str("event", "GetCoinsValue").
			Err(err)
		return deposit, err
	}

	for _, dep := range allDeps {
		coinInfo, err := service.GetCoin(dep.CoinSymbol)
		if err != nil {
			log.Info().
				Str("section", "service").
				Str("action", "GetAllDepositsInUSD").
				Str("event", "GetCoin").
				Err(err)
			return deposit, err
		}
		if coinInfo.Type == "crypto" {
			deposit.AmountCrypto = deposit.AmountCrypto.Add(coins[dep.CoinSymbol]["usdt"], deposit.AmountCrypto)
		} else {
			deposit.AmountFiat = deposit.AmountFiat.Add(coins[dep.CoinSymbol]["usdt"], deposit.AmountFiat)
		}
	}
	deposit.FirstLogin = firstLogin

	return deposit, nil
}
