package actions

import (
	"context"
	"encoding/json"
	"expvar"
	"fmt"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/gostop"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/centrifugal/protocol"
	"github.com/gin-gonic/gin"
	order_cache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/order"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	cache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/userbalance"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service"

	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/monitor"

	"github.com/centrifugal/centrifuge"
	"github.com/ericlagergren/decimal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

//var node *centrifuge.Node

// var tradeChan chan model.Trade = make(chan model.Trade, 10000)
// var orderChan chan model.Order = make(chan model.Order, 10000)
var depositChan chan model.Transaction = make(chan model.Transaction, 1000)
var withdrawRequestChan chan model.WithdrawRequest = make(chan model.WithdrawRequest, 1000)
var notificationChan chan *model.NotificationWithTotalUnread = make(chan *model.NotificationWithTotalUnread, 1000)
var zero = new(decimal.Big)
var (
	apiBalanceUpdates    = expvar.NewInt("apiBalanceUpdates")
	socketBalanceUpdates = expvar.NewInt("socketBalanceUpdates")
)

// SocketEvent type for sending initial subscription data
type SocketEvent map[string]interface{}
type Command struct {
	ResponseChannel string `json:"responseChannel"`
	Channel         string `json:"channel"`
}

func (cmd *Command) String() string {
	bytes, _ := json.Marshal(cmd)
	return string(bytes)
}
func (cmd *Command) Parse(data string) error {
	return json.Unmarshal([]byte(data), cmd)
}

type Response struct {
	Send bool   `json:"send"`
	Data []byte `json:"data"`
}

func (res *Response) String() string {
	bytes, _ := json.Marshal(res)
	return string(bytes)
}

func (actions *Actions) getUserIdAndSubAccountFromChannel(channel string) (uint64, int64, error) {
	userChunks := strings.Split(channel, "#")
	userID := uint64(0)
	subAccountId := int64(-1)
	if len(userChunks) >= 2 {
		var err error
		userID, err = strconv.ParseUint(userChunks[1], 10, 64)
		if err != nil {
			return userID, subAccountId, err
		}
		if len(userChunks) >= 3 {
			subAccountId, err = strconv.ParseInt(userChunks[2], 10, 64)
			if err != nil {
				return userID, subAccountId, err
			}
		}
	}

	return userID, subAccountId, nil
}

func (actions *Actions) GetChannelPrefix(channel string) string {
	userChunks := strings.Split(channel, "#")
	chunks := strings.Split(userChunks[0], "/")
	channelPrefix := chunks[0]

	return channelPrefix
}

func (actions *Actions) GetInitialReplyForChannel(channel string, userID uint64, subAccountId int64) (bool, []byte) {
	startTime := time.Now() // Record the start time of the function execution

	userChunks := strings.Split(channel, "#")
	channelPrefix := actions.GetChannelPrefix(channel)

	switch channelPrefix {
	// stream latest trades on subscribe to user trades channel
	case "user:trades":
		return false, nil
	// get all open orders or completed orders in the last 24 hours
	case "user:orders":
		return false, nil
	case "user:notifications":
		return false, nil
	// get user balances
	case "user:order-update":
		return false, nil
	case "user:balances":
		if userID == 0 {
			return false, nil
		}

		balances := service.NewBalances()
		var err error
		if subAccountId == -1 {
			err = actions.service.GetAllLiabilityBalances(balances, userID)
		} else {

			accId, err := strconv.ParseUint(userChunks[2], 10, 64)
			if err != nil {
				log.Error().
					Err(err).
					Str("section", "websocket").
					Str("action", "node:balances").
					Str("channel", fmt.Sprintf("user:balances#%d", userID)).
					Msg("Unable to parse account ID")
				return false, nil
			}

			account, err := subAccounts.GetUserSubAccountByID(model.MarketTypeSpot, userID, accId)
			if err != nil {
				log.Error().
					Err(err).
					Str("section", "websocket").
					Str("action", "node:balances").
					Str("channel", fmt.Sprintf("user:balances#%d", userID)).
					Msg("Unable to get user subaccount by ID")
				return false, nil
			}

			err = actions.service.GetAllLiabilityBalancesForSubAccount(balances, userID, account)
			if err != nil {
				log.Error().
					Err(err).
					Str("section", "websocket").
					Str("action", "node:balances").
					Str("channel", fmt.Sprintf("user:balances#%d", userID)).
					Msg("Unable to get all liability balances for subaccount")
				return false, nil
			}
		}

		if err != nil {
			log.Error().
				Err(err).
				Str("section", "websocket").
				Str("action", "node:balances").
				Str("channel", fmt.Sprintf("user:balances#%d", userID)).
				Msg("Unable to get user balances")
			return false, nil
		}

		data, err := json.Marshal(SocketEvent{"channel": channel, "data": balances.GetAll()})
		if err != nil {
			log.Error().
				Err(err).
				Str("section", "websocket").
				Str("action", "node:balances").
				Str("channel", fmt.Sprintf("user:balances#%d", userID)).
				Msg("Unable to marshal user balances")
			return false, nil
		}

		log.Debug().
			Str("section", "websocket").
			Str("action", "node:balances").
			Str("channel", fmt.Sprintf("user:balances#%d", userID)).
			Str("data", string(data)).
			Msg("User balances init message")

		// Increment the counter of balance updates via sockets
		socketBalanceUpdates.Add(1)

		elapsedTime := time.Since(startTime)                                                    // Calculate the duration of the function execution
		apiRequestDuration.WithLabelValues(channel, "websocket").Observe(elapsedTime.Seconds()) // Record the duration in the metric

		apiRequestsTotal.WithLabelValues(channel, "websocket").Inc() // Increment the websocket call counter

		return true, data
	}
	return false, nil
}

// // GetSocketTradeChan get a channel to send new trades
// func GetSocketTradeChan() chan<- model.Trade {
// 	return tradeChan
// }

// // GetSocketOrderChan get a channel to send updates on orders
// func GetSocketOrderChan() chan<- model.Order {
// 	return orderChan
// }

// GetSocketDepositChan get a channel to send new deposits
func GetSocketDepositChan() chan<- model.Transaction {
	return depositChan
}

// GetSocketWithdrawRequestChan get a channel to send new withdraw requests
func GetSocketWithdrawRequestChan() chan<- model.WithdrawRequest {
	return withdrawRequestChan
}

// GetSocketNotificationChan get a channel to send new notification
func GetSocketNotificationChan() chan<- *model.NotificationWithTotalUnread {
	return notificationChan
}

// func (actions *Actions) startTradesPublisher(node *centrifuge.Node) {
// 	go func() {
// 		for trade := range tradeChan {
// 			// publish trade for buyer
// 			bidTrade := model.UserTrade{UserID: trade.BidOwnerID, Trade: trade}
// 			data, _ := json.Marshal(&bidTrade)
// 			_ = node.Publish(fmt.Sprintf("user:trades/%s#%d", trade.MarketID, trade.BidOwnerID), data)
// 			// publish trade for seller
// 			askTrade := model.UserTrade{UserID: trade.AskOwnerID, Trade: trade}
// 			data, _ = json.Marshal(&askTrade)
// 			_ = node.Publish(fmt.Sprintf("user:trades/%s#%d", trade.MarketID, trade.AskOwnerID), data)
// 			actions.PublishBalanceUpdate(node, trade.AskOwnerID)
// 			actions.PublishBalanceUpdate(node, trade.BidOwnerID)
// 		}
// 	}()
// }

// func (actions *Actions) startOrdersPublisher(node *centrifuge.Node) {
// 	go func() {
// 		for order := range orderChan {
// 			// publish order
// 			data, _ := json.Marshal(&order)
// 			log.Info().Str("section", "websocket").Str("action", "node:order-publish").Str("channel", fmt.Sprintf("user:orders/%s#%d", order.MarketID, order.OwnerID)).Uint64("order id", order.ID).Msg("Publishing order update")
// 			_ = node.Publish(fmt.Sprintf("user:orders/%s#%d", order.MarketID, order.OwnerID), data)
// 			actions.PublishBalanceUpdate(node, order.OwnerID)
// 		}
// 	}()
// }

// PublishBalanceUpdate publish a message with the user balances
func (actions *Actions) PublishBalanceUpdate(userID uint64, account *model.SubAccount) {
	balances := service.NewBalances()
	err := actions.service.GetAllLiabilityBalancesForSubAccount(balances, userID, account)
	if err != nil {
		return
	}
	data, err := json.Marshal(balances.GetAll())

	chanName := fmt.Sprintf("user:balances#%d", userID)
	if err != nil {
		log.Error().
			Err(err).
			Str("section", "websocket").
			Str("action", "PublishBalanceUpdate").
			Str("channel", chanName).
			Msg("Unable to marshal user balances")
		return
	}
	_, err = actions.service.WsNode.Publish(chanName, data)
	if err != nil {
		log.Error().
			Err(err).
			Str("section", "websocket").
			Str("action", "PublishBalanceUpdate").
			Str("channel", chanName).
			Msg("Unable to publish user balances")
	}
}

func (actions *Actions) PublishTransferBtwSubAccountsUpdate(userID uint64) {
	balances := service.NewBalances()
	err := actions.service.GetAllLiabilityBalances(balances, userID)
	if err != nil {
		return
	}
	data, err := json.Marshal(balances.GetAll())

	chanName := fmt.Sprintf("user:balances#%d", userID)
	if err != nil {
		log.Error().
			Err(err).
			Str("section", "websocket").
			Str("action", "PublishTransferBtwSubAccountsUpdate").
			Str("channel", chanName).
			Msg("Unable to marshal user balances")
		return
	}

	_, err = actions.service.WsNode.Publish(chanName, data)
	if err != nil {
		log.Error().
			Err(err).
			Str("section", "websocket").
			Str("action", "PublishTransferBtwSubAccountsUpdate").
			Str("channel", chanName).
			Msg("Unable to publish user balances")
	}
}

// PublishNotification publish notification to user
func (actions *Actions) PublishNotification(notification *model.NotificationWithTotalUnread) {

	data, err := json.Marshal(map[string]interface{}{
		"id":                       notification.ID,
		"notification_status":      notification.Status,
		"related_object_type":      notification.RelatedObjectType,
		"related_object_id":        notification.RelatedObjectID,
		"type":                     notification.Type,
		"title":                    notification.Title,
		"message":                  notification.Message,
		"totalUnreadNotifications": notification.TotalUnreadNotifications,
	})

	chanName := fmt.Sprintf("user:notifications#%d", notification.UserID)
	if err != nil {
		log.Error().
			Err(err).
			Str("section", "websocket").
			Str("action", "PublishNotification").
			Str("channel", chanName).
			Msg("Unable to marshal user notifications")
		return
	}
	_, err = actions.service.WsNode.Publish(chanName, data)
	if err != nil {
		log.Error().
			Err(err).
			Str("section", "websocket").
			Str("action", "PublishNotification").
			Str("channel", chanName).
			Msg("Unable to publish user notifications")
	}
}

func (actions *Actions) startReloadBalancePublisher(ctx context.Context, wait *sync.WaitGroup) {
	log.Info().Str("worker", "reload_balance_publisher").Str("action", "start").Msg("Reload balance publisher - started")
	for {
		select {
		case tx := <-depositChan:
			account, _ := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, tx.UserID, model.AccountGroupMain)
			actions.PublishBalanceUpdate(tx.UserID, account)
			cache.SetWithPublish(tx.UserID, account.ID)
			monitor.DepositCount.WithLabelValues().Desc()
			actions.publishDeposit(tx)
		case req := <-withdrawRequestChan:
			account, _ := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, req.UserID, model.AccountGroupMain)
			actions.PublishBalanceUpdate(req.UserID, account)
			cache.SetWithPublish(req.UserID, account.ID)
			monitor.WithdrawCount.WithLabelValues().Desc()
		case notification := <-notificationChan:
			actions.PublishNotification(notification)
		case <-ctx.Done():
			log.Info().Str("worker", "reload_balance_publisher").Str("action", "stop").Msg("18 => Reload balance publisher - stopped")
			wait.Done()
			return
		}
	}
}

func (actions *Actions) publishDeposit(tx model.Transaction) {

	var event model.WalletEvent

	event.Symbol = tx.CoinSymbol
	event.Op = "add"
	event.Amount = tx.Amount.V
	event.LockedAmount = zero
	event.InOrder = zero
	event.InBTC = zero
	// publish update
	data, err := json.Marshal(&event)
	monitor.DepositQueued.WithLabelValues().Desc()
	if err != nil {
		log.Error().
			Err(err).
			Str("section", "websocket").
			Str("action", "startDepositPublisher").
			Str("channel", fmt.Sprintf("user:balances#%d", tx.UserID)).
			Interface("data", data).
			Msg("Unable to marshal new deposit")
		return
	}

	_, err = actions.service.WsNode.Publish(fmt.Sprintf("user:balances#%d", tx.UserID), data)
	if err != nil {
		log.Error().
			Err(err).
			Str("section", "websocket").
			Str("action", "startDepositPublisher").
			Str("channel", fmt.Sprintf("user:balances#%d", tx.UserID)).
			Interface("data", data).
			Msg("Unable to publish new deposit")
	}
}

func (actions *Actions) PublishOrderUpdate(userId uint64, subAccountId int64, order *model.Order) {
	data, err := json.Marshal(order)
	if err != nil {
		log.Error().
			Err(err).
			Str("section", "websocket").
			Str("action", "PublishOrderUpdate").
			Str("channel", "public:order-update").
			Interface("data", data).
			Msg("Unable to marshal order updates")
	}

	if subAccountId == -1 {
		_, err = actions.service.WsNode.Publish(fmt.Sprintf("user:order-update#%d", userId), data)
	} else {
		_, err = actions.service.WsNode.Publish(fmt.Sprintf("user:order-update#%d#%d", userId, subAccountId), data)
	}
	if err != nil {
		log.Error().
			Err(err).
			Str("section", "websocket").
			Str("action", "PublishOrderUpdate").
			Str("channel", "public:coins-value").
			Interface("data", data).
			Msg("Unable to publish order updates")
	}
}

func (actions *Actions) startRatesPublisher(ctx context.Context, wait *sync.WaitGroup) {
	log.Info().Str("worker", "rates_publisher").Str("action", "start").Msg("Rates publisher - started")
	ticker := time.NewTicker(time.Second)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			log.Info().Str("worker", "rates_publisher").Str("action", "stop").Msg("19 => Rates publisher - stopped")
			wait.Done()
			return
		case <-ticker.C:
			data, err := actions.service.GetCoinsValue()
			if err != nil {
				continue
			}

			b, err := json.Marshal(data)
			if err != nil {
				log.Error().
					Err(err).
					Str("section", "websocket").
					Str("action", "startRatesPublisher").
					Str("channel", "public:coins-value").
					Interface("data", data).
					Msg("Unable to marshal coin values")
				continue
			}

			_, err = actions.service.WsNode.Publish("public:coins-value", b)
			if err != nil {
				log.Error().
					Err(err).
					Str("section", "websocket").
					Str("action", "startRatesPublisher").
					Str("channel", "public:coins-value").
					Interface("data", data).
					Msg("Unable to publish coin values")
				continue
			}
		}

	}
}

func (actions *Actions) startWithdrawRequestPublisher(ctx context.Context, wait *sync.WaitGroup) {
	log.Info().Str("worker", "withdraw_request_publisher").Str("action", "start").Msg("Withdraw request publisher - started")

	var event model.WalletEvent
	for {
		select {
		case req := <-withdrawRequestChan:
			event.Symbol = req.CoinSymbol
			event.Op = "sub"
			event.Amount = new(decimal.Big).Add(req.Amount.V, req.FeeAmount.V)
			event.LockedAmount = zero
			event.InOrder = zero
			event.InBTC = zero
			// publish update
			data, err := json.Marshal(&event)
			monitor.WithdrawQueued.WithLabelValues().Desc()
			if err != nil {
				log.Error().
					Err(err).
					Str("section", "websocket").
					Str("action", "startWithdrawRequestPublisher").
					Str("channel", fmt.Sprintf("user:balances#%d", req.UserID)).
					Interface("data", data).
					Msg("Unable to marshal new withdrawal")
				continue
			}
			_, err = actions.service.WsNode.Publish(fmt.Sprintf("user:balances#%d", req.UserID), data)
			if err != nil {
				log.Error().
					Err(err).
					Str("section", "websocket").
					Str("action", "startWithdrawRequestPublisher").
					Str("channel", fmt.Sprintf("user:balances#%d", req.UserID)).
					Interface("data", data).
					Msg("Unable to publish new withdrawal")
			}
		case <-ctx.Done():
			log.Info().Str("worker", "withdraw_request_publisher").Str("action", "stop").Msg("17 => Withdraw request publisher - stopped")
			wait.Done()
			return
		}
	}
}

var node *centrifuge.Node

// StartNode configure a centrifuge node and start processing messages
func (actions *Actions) StartNode(jwtSecret string) *centrifuge.Node {
	log.Info().Str("worker", "websocket").Str("action", "start").Msg("Websocket worker - started")
	cfg := centrifuge.DefaultConfig

	// Set secret to handle requests with JWT auth too. This is
	// not required if you don't use token authentication and
	// private subscriptions verified by token.

	cfg.LogLevel = centrifuge.LogLevelInfo
	cfg.LogHandler = handleLog

	var err error
	node, err = centrifuge.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to start WebSocket Node")
	}

	node.OnConnecting(func(ctx context.Context, e centrifuge.ConnectEvent) (centrifuge.ConnectReply, error) {
		claims, err := ParseToken(e.Token, jwtSecret)

		if err == nil {
			cred := &centrifuge.Credentials{
				UserID: fmt.Sprintf("%s", claims["sub"]),
			}

			return centrifuge.ConnectReply{
				Credentials: cred,
				Data:        protocol.Raw(`{}`),
			}, nil
		}
		return centrifuge.ConnectReply{
			Data: protocol.Raw(`{}`),
			Subscriptions: map[string]centrifuge.SubscribeOptions{
				"public": {},
				"user":   {},
			},
			Credentials: &centrifuge.Credentials{
				UserID: "",
			},
		}, nil
	})

	node.OnConnect(func(client *centrifuge.Client) {
		client.OnSubscribe(func(e centrifuge.SubscribeEvent, callback centrifuge.SubscribeCallback) {
			authUserId, err := strconv.ParseUint(client.UserID(), 10, 64)

			if err != nil {
				claims, err := ParseToken(e.Token, jwtSecret)
				if err == nil {
					if claims["sub"] != nil {
						authUserId, err = strconv.ParseUint(claims["sub"].(string), 10, 64)
						if err != nil {
							log.Error().Err(err).Str("section", "websocket").Str("action", "node:subscribe").Msgf("Unable to parse channel information in channel %s", e.Channel)

						}
					}
				}
			}

			channelUserId, channelSubAccountId, err := actions.getUserIdAndSubAccountFromChannel(e.Channel)
			if err != nil {
				log.Error().Err(err).Str("section", "websocket").Str("action", "node:subscribe").Msgf("Unable to parse channel information in channel %s", e.Channel)
			}

			if channelUserId != 0 && authUserId != channelUserId {
				if err := client.Unsubscribe(e.Channel); err != nil {
					log.Error().Err(err).Str("section", "websocket").Str("action", "node:subscribe").Msgf("Unable to send initial message to %s", e.Channel)
				}
				return
			}
			log.Debug().Str("section", "websocket").Str("action", "node:subscribe").Msgf("Subscribing to channel %s", e.Channel)

			if send, data := actions.GetInitialReplyForChannel(e.Channel, channelUserId, channelSubAccountId); send {
				if err := client.Send(data); err != nil {
					log.Error().Err(err).Str("section", "websocket").Str("action", "node:subscribe").Msgf("Unable to send initial message to %s", e.Channel)
				}
			}
			channelPrefix := actions.GetChannelPrefix(e.Channel)
			if channelPrefix == "user:order-update" {
				order_cache.SetSubscribe(authUserId, channelSubAccountId)
			}
			callback(centrifuge.SubscribeReply{Options: centrifuge.SubscribeOptions{ExpireAt: time.Now().Unix() + 24*60*60}}, nil)
		})

		client.OnSubRefresh(func(e centrifuge.SubRefreshEvent, callback centrifuge.SubRefreshCallback) {
			callback(centrifuge.SubRefreshReply{
				ExpireAt: time.Now().Unix() + 24*60*60,
			}, nil)
		})

		client.OnUnsubscribe(func(e centrifuge.UnsubscribeEvent) {
			//check for order
			channelUserId, channelSubAccountId, err := actions.getUserIdAndSubAccountFromChannel(e.Channel)
			if err != nil {
				log.Info().Str("section", "websocket").Str("action", "node:Unsubscribe").Msgf("Unsubscribe to channel %s", e.Channel)
			}
			channelPrefix := actions.GetChannelPrefix(e.Channel)
			if channelPrefix == "user:order-update" {
				order_cache.DeleteSubscribe(channelUserId, channelSubAccountId)
				if channelSubAccountId == -1 {
					order_cache.EraseAllOrderHolder(channelUserId)
				} else {
					order_cache.EraseSubAccountOrderHolder(channelUserId, uint64(channelSubAccountId))
				}
			}
			log.Debug().Str("section", "websocket").Str("action", "node:Unsubscribe").Msgf("Unsubscribe to channel %s", e.Channel)
		})

		client.OnDisconnect(func(e centrifuge.DisconnectEvent) {
		})

		client.OnPublish(func(e centrifuge.PublishEvent, publishCallback centrifuge.PublishCallback) {
			log.Info().Str("section", "websocket").Str("action", "node:publish").Msgf("Publishing to channel %s", e.Channel)
			publishCallback(centrifuge.PublishReply{}, nil)
		})

		client.OnRefresh(func(e centrifuge.RefreshEvent, callback centrifuge.RefreshCallback) {
			callback(centrifuge.RefreshReply{
				ExpireAt: time.Now().Unix() + 60*60,
			}, nil)
		})
	})

	engine, err := centrifuge.NewMemoryBroker(node, centrifuge.MemoryBrokerConfig{})

	if err != nil {
		log.Fatal().Err(err).
			Str("section", "websocket").
			Str("action", "node:init").
			Msg("Unable to set memory engine")
	}
	node.SetBroker(engine)
	if err := node.Run(); err != nil {
		log.Fatal().Err(err).
			Str("section", "websocket").
			Str("action", "node:init").
			Msg("Unable to start websocket server node")
	}

	gostop.GetInstance().Go("websocket_ticket_stats_publisher", actions.getTickerStatsPublisher(node), true)   // public:market/24h_tick
	gostop.GetInstance().Go("websocket_market_level2_publisher", actions.getMarketLevel2Publisher(node), true) // public:market-depth/%s
	gostop.GetInstance().Go("websocket_public_trades_publisher", actions.getPublicTradesPublisher(node), true) // public:trades/%s

	// actions.startTradesPublisher(node)
	// actions.startOrdersPublisher(node)
	gostop.GetInstance().Go("websocket_withdraw_request_publisher", actions.startWithdrawRequestPublisher, true)
	gostop.GetInstance().Go("websocket_reload_balance_publisher", actions.startReloadBalancePublisher, true)
	gostop.GetInstance().Go("websocket_rates_publisher", actions.startRatesPublisher, true)

	return node
}

func (actions *Actions) StopNode(ctx context.Context) error {
	log.Info().Str("worker", "websocket").Str("action", "start").Msg("Websocket worker - stopped")
	return node.Shutdown(ctx)
}

func handleLog(e centrifuge.LogEntry) {
	lev := log.
		With().
		Str("section", "websocket").
		Str("action", "event:log").
		Str("ws_log", fmt.Sprintf("%#v", e.Fields)).
		Logger()
	switch e.Level {
	case centrifuge.LogLevelDebug:
		lev.Debug().Msg(e.Message)
	case centrifuge.LogLevelInfo:
		lev.Info().Msg(e.Message)
	case centrifuge.LogLevelError:
		lev.Error().Msg(e.Message)
	default:
		return
	}
}

func (actions *Actions) AuthMiddleware(h http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		cred := &centrifuge.Credentials{
			UserID: "",
		}
		newCtx := centrifuge.SetCredentials(c, cred)
		c.Request = c.Request.WithContext(newCtx)
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func (actions *Actions) NewWebsocketHandler() http.Handler {
	return centrifuge.NewWebsocketHandler(node, centrifuge.WebsocketConfig{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	})
}

func (actions *Actions) NewSockjsHandler(prefix string) http.Handler {
	return centrifuge.NewSockjsHandler(node, centrifuge.SockjsConfig{
		URL:           "https://cdn.jsdelivr.net/npm/sockjs-client@1/dist/sockjs.min.js",
		HandlerPrefix: prefix,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		WebsocketCheckOrigin: func(r *http.Request) bool {
			return true
		},
	})
}
