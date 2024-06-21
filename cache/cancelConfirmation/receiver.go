package cancelConfirmation

import (
	"errors"
	"net/http"
	"sync"

	"github.com/rs/zerolog/log"
	kafkaGo "github.com/segmentio/kafka-go"
	occ "gitlab.com/paramountdax-exchange/exchange_api_v2/data/order_canceled_confirmation"
)

type OrderCancelConfirmationResponse struct {
	Status      occ.OrderCanceledStatus
	ErrorMsg    string
	IsCancelled bool
}

func (msg OrderCancelConfirmationResponse) GetHttpCode() (code int) {

	switch msg.Status {
	case occ.OrderCanceledStatus_AlreadyFilled:
		code = http.StatusAccepted
	case occ.OrderCanceledStatus_AlreadyCancelled:
		code = http.StatusAlreadyReported
	case occ.OrderCanceledStatus_OK:
		code = http.StatusOK
	default:
		code = http.StatusInternalServerError
	}

	return code
}

type cancelQueue struct {
	l sync.Mutex
	c map[uint64][]chan OrderCancelConfirmationResponse
}

var receiver cancelQueue

func Init() {
	receiver = cancelQueue{
		l: sync.Mutex{},
		c: make(map[uint64][]chan OrderCancelConfirmationResponse),
	}
}

func WaitOrder(orderId uint64) (chan OrderCancelConfirmationResponse, error) {
	receiver.l.Lock()
	defer receiver.l.Unlock()

	if receiver.c[orderId] != nil {
		return nil, errors.New("canceling already in progress")
	}

	channel := make(chan OrderCancelConfirmationResponse, 1)
	receiver.c[orderId] = append(receiver.c[orderId], channel)

	return channel, nil
}

func ReceiveUpdate(orderId uint64, status occ.OrderCanceledStatus, msg string, isCancelled bool) {
	receiver.l.Lock()
	defer receiver.l.Unlock()

	if receiver.c != nil {
		if _, ok := receiver.c[orderId]; ok {
			for _, channel := range receiver.c[orderId] {
				channel <- OrderCancelConfirmationResponse{
					Status:      status,
					ErrorMsg:    msg,
					IsCancelled: isCancelled,
				}
				close(channel)
			}

			delete(receiver.c, orderId)
		}
	}
}

func CancelWaiting(orderId uint64) {
	receiver.l.Lock()
	defer receiver.l.Unlock()

	if receiver.c != nil {
		if _, ok := receiver.c[orderId]; ok {
			for _, channel := range receiver.c[orderId] {
				close(channel)
			}
		}
		delete(receiver.c, orderId)
	}
}

func Process(msg *kafkaGo.Message) error {

	event := occ.OrderCanceledConfirmation{}
	err := event.FromBinary(msg.Value)
	if err != nil {
		log.Error().
			Err(err).
			Str("method", "Process").
			Str("section", "cancelConfirmation").
			Msg("Unable to decode message from the kafka")
		return err
	}

	ReceiveUpdate(event.OrderId, event.Status, event.ErrorMsg, event.IsCancelled())

	return nil
}
