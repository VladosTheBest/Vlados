package cancelConfirmation

import (
	"context"
	"errors"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/gostop"
	"sync"

	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/data"
	occ "gitlab.com/paramountdax-exchange/exchange_api_v2/data/order_canceled_confirmation"
)

var ErrCancelAlreadyInProgress = errors.New("canceling already in progress")

type ListenerMap struct {
	market   string
	channels map[uint64]chan<- OrderCancelConfirmationResponse
	inputs   chan []data.Event
	lock     sync.Mutex
}

type InternalReceiver struct {
	lock    sync.Mutex
	markets map[string]*ListenerMap
}

var internalReceiver *InternalReceiver = &InternalReceiver{
	markets: map[string]*ListenerMap{},
	lock:    sync.Mutex{},
}

func GetInstance() *InternalReceiver {
	return internalReceiver
}

func (receiver *InternalReceiver) InitMarket(market_id string) *ListenerMap {
	marketListeners := &ListenerMap{
		channels: make(map[uint64]chan<- OrderCancelConfirmationResponse),
		inputs:   make(chan []data.Event, 1000),
		lock:     sync.Mutex{},
		market:   market_id,
	}
	receiver.lock.Lock()
	receiver.markets[market_id] = marketListeners
	receiver.lock.Unlock()
	return marketListeners
}

func (receiver *InternalReceiver) GetListenerMap(market_id string) *ListenerMap {
	receiver.lock.Lock()
	defer receiver.lock.Unlock()
	return receiver.markets[market_id]
}

func (lmap *ListenerMap) GetInputChan() chan<- []data.Event {
	return lmap.inputs
}

func (lmap *ListenerMap) StartReceiveLoop() {
	gostop.GetInstance().Go("cancel_confirmation_internal_receive_loop", func(ctx context.Context, wait *sync.WaitGroup) {
		inputs := lmap.inputs
		log.Info().Str("worker", "cancel_confirmation_internal").Str("action", "start").
			Str("market", lmap.market).Msg("Cancel confirmation internal - started")
		var order_id uint64
		// watch for new generated events over the input channel
		for {
			select {
			case events := <-inputs:
				// iterate over each event group received
				for _, event := range events {
					var cStatus occ.OrderCanceledStatus
					// filter only status change events
					switch event.Type {
					case data.EventType_OrderStatusChange:
						orderStatus := event.GetOrderStatus()
						order_id = orderStatus.ID
						// filter only cancel and filled events
						switch orderStatus.Status {
						case data.OrderStatus_Filled:
							cStatus = occ.OrderCanceledStatus_AlreadyFilled
						case data.OrderStatus_Cancelled:
							// call cancel listener
							cStatus = occ.OrderCanceledStatus_OK
						default:
							continue
						}
					case data.EventType_Error:
						orderError := event.GetError()
						switch orderError.Code {
						case data.ErrorCode_CancelFailed:
							cStatus = occ.OrderCanceledStatus_CancelFailedFromME
						default:
							continue
						}
					default:
						continue
					}

					// notify registered listeners
					lmap.lock.Lock()
					listener, ok := lmap.channels[order_id]
					delete(lmap.channels, order_id)
					lmap.lock.Unlock()

					log.Debug().Uint64("order_id", order_id).Str("status", cStatus.String()).Msg("Internal cancellation event received")

					if ok {
						listener <- OrderCancelConfirmationResponse{
							Status:      cStatus,
							ErrorMsg:    "",
							IsCancelled: true,
						}
						close(listener)
					}
				}
			case <-ctx.Done():
				log.Info().Str("worker", "cancel_confirmation_internal").Str("action", "stop").
					Str("market", lmap.market).Msg("22 => Cancel confirmation internal - stopped")
				wait.Done()
				return
			}
		}
	}, false)
}

func (lmap *ListenerMap) GetListener(order_id uint64) (<-chan OrderCancelConfirmationResponse, error) {
	lmap.lock.Lock()
	if lmap.channels[order_id] != nil {
		lmap.lock.Unlock()
		return nil, ErrCancelAlreadyInProgress
	}
	listener := make(chan OrderCancelConfirmationResponse, 1)
	lmap.channels[order_id] = listener
	lmap.lock.Unlock()
	return listener, nil
}

func (lmap *ListenerMap) RemoveListener(order_id uint64) {
	lmap.lock.Lock()
	listener, ok := lmap.channels[order_id]
	delete(lmap.channels, order_id)
	lmap.lock.Unlock()
	if ok {
		close(listener)
	}
}
