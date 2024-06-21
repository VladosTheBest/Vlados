package wallet

import (
	"fmt"
	"github.com/rs/zerolog/log"
	kafkaGo "github.com/segmentio/kafka-go"
	data "gitlab.com/paramountdax-exchange/exchange_api_v2/data/wallet"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/gostop"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/ops"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

type transactionConfirmed interface {
	SendEmailForDepositConfirmed(transaction *model.Transaction, language string) error
	SendEmailForWithdrawConfirmed(transaction *model.Transaction, language string) error

	SendNotification(userID uint64, notificationType model.NotificationType, title,
		message string, relatedObjectType model.RelatedObjectType,
		relatedObjectID string) (*model.NotificationWithTotalUnread, error)
}

// App type
// Handle incomming events from any configured wallet service and update
// the database with any received deposits
type App struct {
	repo                 *queries.Repo
	ops                  *ops.Ops
	withdrawQueue        chan WithdrawRequest
	depositQueue         chan<- model.Transaction
	withdrawRequestQueue chan<- model.WithdrawRequest
}

// NewApp create a new application that will process incomming wallet events
func NewApp(repo *queries.Repo, ops *ops.Ops, depositQueue chan<- model.Transaction, withdrawRequestQueue chan<- model.WithdrawRequest) *App {
	return &App{
		repo:                 repo,
		ops:                  ops,
		withdrawQueue:        make(chan WithdrawRequest, 100),
		depositQueue:         depositQueue,
		withdrawRequestQueue: withdrawRequestQueue,
	}
}

// Init the app
func (app *App) Init() error {
	gostop.GetInstance().Go("worker_withdraw_request_loop", app.StartWithdrawLoop, true)
	return nil
}

// Process a new kafka message
func (app *App) Process(msg kafkaGo.Message, d transactionConfirmed) (*model.Transaction, error) {
	// decode the event
	event := data.Event{}
	err := event.FromBinary(msg.Value)
	if err != nil {
		return nil, err
	}
	return app.distributeEvent(&event, d)
}

func (app *App) distributeEvent(event *data.Event, d transactionConfirmed) (*model.Transaction, error) {

	switch data.EventType(event.Event) {
	case data.EventType_CreateAddress:
		return nil, app.CreateAddress(event)
	case data.EventType_Deposit:
		return app.Deposit(event, d)
	case data.EventType_WithdrawCompleted:
		return nil, app.WithdrawCompleted(event)
	case data.EventType_Withdraw:
		return app.Withdraw(event, d)
	default:
		err := fmt.Errorf("Invalid event type received: %v", event.Event)
		log.Error().Err(err).Str("event", fmt.Sprintf("%#v", event)).Msg("Ignored invalid event received from bitgo wallet.")
		return nil, nil
	}
}
