package wallet

import (
	"errors"
	"fmt"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"strconv"
	"strings"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/monitor"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"github.com/ericlagergren/decimal"
	data "gitlab.com/paramountdax-exchange/exchange_api_v2/data/wallet"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

func (app *App) Deposit(event *data.Event, d transactionConfirmed) (*model.Transaction, error) {
	switch event.System {
	case "advcash", "clear_junction":
		return app.DepositCash(event, d)
	default:
		return app.DepositCrypto(event, d)
	}
}

// Deposit handler
func (app *App) DepositCrypto(event *data.Event, d transactionConfirmed) (*model.Transaction, error) {
	var coin model.Coin
	db := app.repo.ConnReader.Where("symbol = ?", strings.ToLower(event.Coin)).First(&coin)
	if db.Error != nil {
		log.Error().Err(db.Error).Str("section", "app:wallet").Str("action", "deposit").Str("coin_symbol", event.Coin).Msg("Coin not found with symbol. Skipping processing deposit")
		return nil, nil
	}

	amount, isAmountSettled := conv.NewDecimalWithPrecision().SetString(event.Payload["amount"])
	if !isAmountSettled {
		log.Error().
			Str("section", "app:wallet").
			Str("action", "Deposit").
			Msg("Unable to set amount")
		return nil, errors.New("amount not set")
	}
	confirmations, err := strconv.Atoi(event.Payload["confirmations"])
	if err != nil {
		log.Error().Err(err).
			Str("section", "app:wallet").
			Str("action", "Deposit").
			Msg("Unable to convert confirmation field")
		return nil, err
	}

	eventStatus := event.Payload["status"]
	status := model.TxStatus_Unconfirmed

	if eventStatus == "confirmed" {
		status = model.TxStatus_Confirmed
	}

	isNew := false
	tx := app.repo.Conn.Begin()

	deposit, err := app.repo.GetTransaction(event.ID)
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		deposit = model.NewDeposit(
			event.ID,
			event.UserID,
			status,
			coin.Symbol,
			event.Payload["txid"],
			event.Payload["address"],
			amount,
			confirmations,
			model.TransactionExternalSystem(event.System),
		)
		isNew = true
		// try to create the record
		db = tx.Create(&deposit)
		if db.Error != nil {
			tx.Rollback()
			// otherwise log the error and skip the message
			log.Error().Err(db.Error).Str("section", "app:wallet").Str("action", "deposit").Str("txid", deposit.ID).Msg("Unable to save new deposit")
			return deposit, db.Error
		}
	}

	// 1. if the transaction is not new then check status
	// 1.1. if the status is confirmed return since the deposit was already processed
	// 1.2. if the status is unconfirmed and the new status is unconfirmed then return since there is nothing to do
	// 1.3. if the status is unconfirmed and the new status is confirmed them confirm the deposit and move balances from locked to unlocked
	// 2. if the transaction is new then check the new status
	// 2.1. if the status is unconfirmed add deposit and update locked balances
	// 2.2. if the status is confirmed then update deposit status and confirmations and create available funds

	// ignore transaction since it was already confirmed or the tx is already saved as unconfirmed
	if !isNew && (deposit.Status == model.TxStatus_Confirmed || (deposit.Status == model.TxStatus_Unconfirmed && status == model.TxStatus_Unconfirmed)) {
		tx.Rollback()
		return deposit, nil
	}

	var zero *decimal.Big = conv.NewDecimalWithPrecision()

	// existing tx: confirm and move funds from locked to unlocked
	if !isNew && deposit.Status == model.TxStatus_Unconfirmed && status == model.TxStatus_Confirmed {
		deposit.Status = status
		// move from locked to unlocked funds
		_, err = app.ops.Deposit(tx, deposit, zero, zero, zero, amount, model.OperationType_Deposit)
		if err != nil {
			tx.Rollback()
			log.Error().Err(err).Str("section", "app:wallet").Str("action", "deposit").Str("txid", deposit.ID).Msg("Unable to save deposit operation")
			return deposit, err
		}
		app.depositQueue <- *deposit
		monitor.DepositCount.WithLabelValues().Inc()
		monitor.DepositQueued.WithLabelValues().Inc()
		return deposit, nil
	}

	// new unconfirmed tx: add locked funds until confirmation
	if isNew && status == model.TxStatus_Unconfirmed {
		deposit.Status = status
		_, err = app.ops.Deposit(tx, deposit, zero, amount, zero, zero, model.OperationType_Deposit)
		if err != nil {
			tx.Rollback()
			log.Error().Err(err).Str("section", "app:wallet").Str("action", "deposit").Str("txid", deposit.ID).Msg("Unable to save deposit operation")
			return deposit, err
		}
		return deposit, nil
	}

	// new confirmed tx: add funds and and confirm tx
	if isNew && status == model.TxStatus_Confirmed {
		deposit.Status = status
		_, err = app.ops.Deposit(tx, deposit, zero, zero, zero, amount, model.OperationType_Deposit)
		if err != nil {
			tx.Rollback()
			log.Error().Err(err).Str("section", "app:wallet").Str("action", "deposit").Str("txid", deposit.ID).Msg("Unable to save deposit operation")
			return deposit, nil
		}

		userDetails, err := app.repo.GetUserDetails(deposit.UserID)
		if err != nil {
			return deposit, err
		}

		depositAmount := deposit.Amount.V.Quantize(coin.TokenPrecision)
		if featureflags.IsEnabled("api.wallets.send-email-for-confirmed-deposit") {
			if deposit != nil && deposit.TxType == model.TxType_Deposit && deposit.Status == model.TxStatus_Confirmed {
				if err = d.SendEmailForDepositConfirmed(deposit, userDetails.Language.String()); err != nil {
					return deposit, err
				}
				_, err = d.SendNotification(deposit.UserID, model.NotificationType_System,
					model.NotificationTitle_Deposit.String(),
					fmt.Sprintf(model.NotificationMessage_Deposit.String(), depositAmount),
					model.Notification_Deposit_Crypto, deposit.ID)

				if err != nil {
					return deposit, err
				}
			}
		}

		app.depositQueue <- *deposit
		monitor.DepositCount.WithLabelValues().Inc()
		monitor.DepositQueued.WithLabelValues().Inc()
		return deposit, nil
	}

	return nil, nil
}

func (app *App) DepositCash(event *data.Event, d transactionConfirmed) (*model.Transaction, error) {
	var coin model.Coin
	db := app.repo.ConnReader.Where("symbol = ?", strings.ToLower(event.Coin)).First(&coin)
	if db.Error != nil {
		log.Error().Err(db.Error).Str("section", "app:wallet").Str("action", "depositCash").Str("coin_symbol", event.Coin).Msg("Coin not found with symbol. Skipping processing deposit")
		return nil, db.Error
	}

	precision := decimal.New(1, coin.TokenPrecision)
	amount, isAmountSet := conv.NewDecimalWithPrecision().SetString(event.Payload["amount"])
	if !isAmountSet {
		log.Error().
			Str("section", "app:wallet").
			Str("action", "Deposit").
			Msg("Unable to set amount")
		return nil, errors.New("amount not set")
	}
	amount = conv.NewDecimalWithPrecision().Mul(amount, precision)

	existingDeposit, _ := app.repo.GetTransaction(event.ID)

	if existingDeposit != nil {
		return existingDeposit, nil
	}

	deposit := model.NewDeposit(
		event.ID,
		event.UserID,
		model.TxStatus_Confirmed,
		coin.Symbol,
		event.Payload["txid"],
		event.Payload["address"],
		amount,
		1,
		model.TransactionExternalSystem(event.System),
	)

	var zero *decimal.Big = conv.NewDecimalWithPrecision()

	userDetails := model.UserDetails{}
	err := app.repo.ConnReader.First(&userDetails, "user_id = ?", deposit.UserID).Error
	if err != nil {
		return deposit, err
	}

	tx := app.repo.Conn.Begin()
	_, err = app.ops.Deposit(tx, deposit, zero, zero, zero, amount, model.OperationType_Deposit)
	if err != nil {
		tx.Rollback()
		log.Error().Err(err).Str("section", "app:wallet").Str("action", "depositCash").Str("txid", deposit.ID).Msg("Unable to save deposit operation")
		return nil, err
	}

	depositAmount := deposit.Amount.V.Quantize(coin.TokenPrecision)
	if featureflags.IsEnabled("api.wallets.send-email-for-confirmed-deposit") {
		if deposit != nil && deposit.TxType == model.TxType_Deposit && deposit.Status == model.TxStatus_Confirmed {
			if err = d.SendEmailForDepositConfirmed(deposit, userDetails.Language.String()); err != nil {
				return deposit, err
			}
			_, err = d.SendNotification(deposit.UserID, model.NotificationType_System,
				model.NotificationTitle_Deposit.String(),
				fmt.Sprintf(model.NotificationMessage_Deposit.String(), depositAmount),
				model.Notification_Deposit_Fiat, deposit.ID)

			if err != nil {
				return deposit, err
			}
		}
	}

	app.depositQueue <- *deposit
	monitor.DepositCount.WithLabelValues().Inc()
	monitor.DepositQueued.WithLabelValues().Inc()

	return deposit, nil
}
