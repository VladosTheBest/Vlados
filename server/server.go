package server

import (
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/cancelConfirmation"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/userbalance"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/gostop"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/monitor"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
	botGridConnector "gitlab.com/paramountdax-exchange/exchange_api_v2/service/bots/grid"
	botTrendConnector "gitlab.com/paramountdax-exchange/exchange_api_v2/service/bots/trend"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/market_engine"
	"net/http"
	"time"

	// import http profilling when the server profilling configuration is set
	_ "net/http/pprof"

	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/actions"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/config"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/net/manager"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service"
)

// Server interface
type Server interface {
	Listen()
}

type server struct {
	config  config.Config
	actions *actions.Actions
	service *service.Service
	dm      *manager.DataManager
	ctx     context.Context
	close   context.CancelFunc
	HTTP    *http.Server
}

// NewServer constructor
// @todo Allow dynamic configuration of connected markets.
//
//	Currently all markets are configured in a configuration file and in order to add a new market configuration
//	The entire system needs to be restarted with an updated configuration file that includes the new market.
//	This can be improved by allowing an administrator to configure and add the market in an external database and
//	the market-price service could connect to it as soon as the new configuration is added.
func NewServer(cfg config.Config) Server {
	ctx, close := context.WithCancel(context.Background())

	err := market_engine.InitSequenceStorage(cfg.Redis)
	if err != nil {
		log.Fatal().Str("section", "server").Err(err).Msg("Unable to init sequence storage")
	}

	dm := manager.NewDataManager(cfg)
	dataServices := service.NewService(
		ctx,
		cfg,
		dm,
		actions.GetSocketDepositChan(),
		actions.GetSocketWithdrawRequestChan(),
		actions.GetSocketNotificationChan(),
		actions.GetPushNotificationChan(),
	)

	dataServices.SolarisSession.InitSession(cfg.Server.Card.UserName, cfg.Server.Card.Password)
	dataServices.SetInitSubAccounts()
	dataServices.SetInitUserReferrals()

	// initiate the websocket node first to start processing messages from the channels
	userActions := actions.NewActions(cfg, dataServices, cfg.Server.API.JWTTokenSecret, cfg.Server.API.JWA2FATokenSecret, ctx)
	dataServices.WsNode = userActions.StartNode(cfg.Server.API.JWTTokenSecret)
	// start the process for users balance update
	gostop.GetInstance().Go("user_actions_balance_update", userActions.BalanceUpdate, true)
	gostop.GetInstance().Go("user_actions_order_update", userActions.OrderUpdate, true)
	cancelConfirmation.Init()

	botGridConnector.Init(cfg.Bots.Grid)
	botTrendConnector.Init(cfg.Bots.Trend)

	cache := userbalance.Init(dm)
	gostop.GetInstance().Go("user_balance", cache.Process, true)
	gostop.GetInstance().Go("user_actions_push_notifications", userActions.PushNotifications, true)

	// start the services to process and generate data
	dataServices.Start()
	return &server{
		config:  cfg,
		dm:      dm,
		service: dataServices,
		actions: userActions,
		ctx:     ctx,
		close:   close,
	}
}

// Listen for new events that affect the market and process them
func (srv *server) Listen() {
	// start listening to queue messages
	markets, err := srv.service.LoadMarketIDs()
	if err != nil {
		log.Fatal().Str("section", "server").Err(err).Msg("Unable to load active markets")
	}
	srv.dm.SetMarkets(markets)
	gostop.GetInstance().Exec("data_manager", srv.dm.Start, true)

	// start the http server
	go srv.ListenToRequests()
	go monitor.LoopProfilingServer(srv.config.Server.Monitoring)

	gostop.GetInstance().Go("user_activity_logger", srv.actions.InitUserActivityLogger, true)

	srv.stopOnSignal()
}

func (srv *server) stopOnSignal() {
	// listen for termination signals
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigc

	log.Info().Str("section", "server").Str("app_event", "terminate").Str("signal", sig.String()).Msg("Shutting down services")
	srv.closeApp(5 * time.Second)
}

func (srv *server) closeApp(timeout time.Duration) {
	// define a timeout in which the graceful shutdown procedure should happen before forcing the shutdown
	timeoutFunc := time.AfterFunc(timeout, func() {
		log.Printf("timeout %d ms has been elapsed, force exit", timeout.Milliseconds())
		os.Exit(0)
	})
	defer timeoutFunc.Stop()

	monitor.ShutdownServer()
	if err := srv.HTTP.Shutdown(context.Background()); err != nil {
		log.Error().Err(err).Str("section", "server").Str("action", "terminate").Msg("Unable to shutdown HTTP server")
	}
	//srv.HTTP.Close()

	// close crons
	srv.service.CloseCrons()
	gostop.GetInstance().
		CancelAndWait("user_actions_push_notifications").
		CancelAndWait("user_activity_logger").
		CancelAndWait("user_actions_balance_update").
		CancelAndWait("user_actions_order_update").
		CancelAndWait("user_balance").
		CancelAndWait("coins_local_app").
		// OMS
		CancelAndWait("market_depth_cache_processor").
		CancelAndWait("cron_active_monitor").
		CancelAndWait("cron_cache_cleanup").
		CancelAndWait("oms_seq_refresher").
		// Websockets
		CancelAndWait("websocket_ticket_stats_publisher").
		CancelAndWait("websocket_market_level2_publisher").
		CancelAndWait("websocket_public_trades_publisher").
		CancelAndWait("websocket_withdraw_request_publisher").
		CancelAndWait("websocket_reload_balance_publisher").
		CancelAndWait("websocket_rates_publisher")

	if err := srv.actions.StopNode(context.Background()); err != nil {
		log.Error().Err(err).Str("worker", "websocket").Str("action", "stop").Msg("Unable to shutdown websocket node")
	} else {
		log.Info().Str("worker", "websocket").Str("action", "stop").Msg("20 => Websocket worker - stopped")
	}

	gostop.GetInstance().
		CancelAndWait("worker_withdraw_request_loop").
		CancelAndWait("cancel_confirmation_internal_receive_loop").
		CancelAndWait("data_manager_kafka_consumer").
		CancelAndWait("data_manager_receive_loop").
		CancelAndWait("market_engine")

	gostop.GetInstance().CancelAndWait("event_processor")
	gostop.GetInstance().CancelAndWait("data_manager")

	srv.close()
	srv.service.Close()
	market_engine.DisconnectSequenceStorage()

	featureflags.Close()
	// make sure database connection is closed on program exit
	queries.Close()

	//log.Debug().
	//	Str("app", "server").
	//	Str("action", "stop").
	//	Int("goroutines_remaining", runtime.NumGoroutine()).
	//	Msg("Exit debug")
	//pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)

	log.Info().Str("section", "server").Str("app_event", "terminate").Str("state", "complete").Msg("All workers terminated")
}
