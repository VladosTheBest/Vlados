package wallet

import (
	"errors"
	"fmt"
	"github.com/ericlagergren/decimal"
	"github.com/rs/zerolog/log"
	data "gitlab.com/paramountdax-exchange/exchange_api_v2/data/wallet"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/monitor"
	"strconv"
	"strings"
)

func (app *App) Withdraw(event *data.Event, w transactionConfirmed) (*model.Transaction, error) {
	switch event.System {
	case "advcash", "clear_junction":
		return app.WithdrawCash(event, w)
	default:
		return app.WithdrawCrypto(event, w)
	}
}

func (app *App) WithdrawCrypto(event *data.Event, w transactionConfirmed) (*model.Transaction, error) {
	var coin model.Coin
	db := app.repo.ConnReader.
		Where("symbol = ?", strings.ToLower(event.Coin)).
		First(&coin)
	if db.Error != nil {
		log.Error().Err(db.Error).
			Str("section", "app:wallet").Str("action", "withdraw").
			Str("coin_symbol", event.Coin).
			Msg("Coin not found with symbol. Skipping processing withdraw")
		return nil, nil
	}

	amount, isAmountSetted := new(decimal.Big).SetString(event.Payload["amount"])
	if !isAmountSetted {
		log.Error().
			Str("section", "app:wallet").
			Str("action", "Withdraw").
			Msg("Unable to set amount")
		return nil, errors.New("amount not set")
	}

	feeAmount, isFeeAmountSetted := new(decimal.Big).SetString(event.Payload["fee"])
	if !isFeeAmountSetted {
		log.Error().
			Str("section", "app:wallet").
			Str("action", "Withdraw").
			Msg("Unable to set fee amount")
		return nil, errors.New("fee amount not set")
	}

	confirmations, err := strconv.Atoi(event.Payload["confirmations"])
	if err != nil {
		log.Error().Err(err).
			Str("section", "app:wallet").
			Str("action", "Withdraw").
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

	var withdraw *model.Transaction
	db = app.repo.Conn.
		Table("transactions").
		Where("transactions.id = ?", event.ID).
		First(&withdraw)
	if db.Error != nil {
		withdraw = model.NewWithdraw(
			event.ID,
			event.UserID,
			status,
			coin.Symbol,
			coin.Symbol,
			event.Payload["txId"],
			event.Payload["address"],
			amount,
			feeAmount,
			confirmations,
			model.TransactionExternalSystem(event.System),
		)

		isNew = true

		db = app.repo.Conn.
			Table("transactions").
			Create(&withdraw)
		if db.Error != nil {
			db.Rollback()

			log.Error().Err(db.Error).
				Str("section", "app:wallet").
				Str("action", "withdraw").
				Str("txId", withdraw.ID).
				Msg("Unable to save new withdraw")
		}
	}

	// 1. if the transaction is not new then check status
	// 1.1. if the status is confirmed return since withdraw was already processed
	// 1.2. if the status is unconfirmed and the new status is unconfirmed then return since there is nothing to do
	// 1.3. if the status is unconfirmed and the new status is confirmed them confirm withdraw and move balances from locked to unlocked
	// 2. if the transaction is new then check the new status
	// 2.1. if the status is unconfirmed add withdraw and update locked balances
	// 2.2. if the status is confirmed then update withdraw status and confirmations and create available funds

	// ignore transaction since it was already confirmed or the tx is already saved as unconfirmed
	if !isNew && (withdraw.Status == model.TxStatus_Confirmed || (withdraw.Status == model.TxStatus_Unconfirmed && status == model.TxStatus_Unconfirmed)) {
		tx.Rollback()
		return withdraw, nil
	}

	var zero *decimal.Big = new(decimal.Big)

	// existing tx: confirm and move funds from locked to unlocked to get debited
	if !isNew && withdraw.Status == model.TxStatus_Unconfirmed && status == model.TxStatus_Confirmed {
		withdraw.Status = status
		// move from locked to unlocked funds
		_, err = app.ops.Deposit(tx, withdraw, zero, zero, amount, zero, model.OperationType_Withdraw)
		if err != nil {
			tx.Rollback()
			log.Error().Err(err).Str("section", "app:wallet").Str("action", "withdraw").Str("txid", withdraw.ID).Msg("Unable to save withdraw operation")
			return withdraw, err
		}
		app.depositQueue <- *withdraw
		monitor.WithdrawCount.WithLabelValues().Inc()
		monitor.WithdrawQueued.WithLabelValues().Inc()
		return withdraw, nil
	}

	// new unconfirmed tx: add locked funds until confirmation
	if isNew && status == model.TxStatus_Unconfirmed {
		withdraw.Status = status
		_, err = app.ops.Deposit(tx, withdraw, amount, zero, zero, zero, model.OperationType_Withdraw)
		if err != nil {
			tx.Rollback()
			log.Error().Err(err).Str("section", "app:wallet").Str("action", "withdraw").Str("txid", withdraw.ID).Msg("Unable to save withdraw operation")
			return withdraw, err
		}
		return withdraw, nil
	}

	// new confirmed tx: confirm and make withdraw request
	if isNew && status == model.TxStatus_Confirmed {
		withdraw.Status = status

		tx := app.repo.Conn.Begin()
		_, err = app.ops.Deposit(tx, withdraw, zero, zero, amount, zero, model.OperationType_Withdraw)
		if err != nil {
			tx.Rollback()
			log.Error().Err(err).Str("section", "app:wallet").Str("action", "withdraw").Str("txid", withdraw.ID).Msg("Unable to save withdraw operation")
			return withdraw, err
		}

		userDetails := model.UserDetails{}
		err := app.repo.ConnReader.First(&userDetails, "user_id = ?", withdraw.UserID).Error
		if err != nil {
			return withdraw, err
		}

		withdrawAmount := withdraw.Amount.V.Quantize(coin.TokenPrecision)
		if featureflags.IsEnabled("api.wallets.send-email-for-confirmed-withdraw") {
			if withdraw != nil && withdraw.TxType == model.TxType_Withdraw && withdraw.Status == model.TxStatus_Confirmed {
				if err = w.SendEmailForWithdrawConfirmed(withdraw, userDetails.Language.String()); err != nil {
					return withdraw, err
				}
				_, err = w.SendNotification(withdraw.UserID, model.NotificationType_System,
					model.NotificationTitle_Withdraw.String(),
					fmt.Sprintf(model.NotificationMessage_Withdraw.String(), withdrawAmount),
					model.Notification_Deposit_Fiat, withdraw.ID)

				if err != nil {
					return withdraw, err
				}
			}
		}

		app.depositQueue <- *withdraw
		monitor.WithdrawCount.WithLabelValues().Inc()
		monitor.WithdrawQueued.WithLabelValues().Inc()
		return withdraw, nil
	}

	return nil, nil
}

func (app *App) WithdrawCash(event *data.Event, w transactionConfirmed) (*model.Transaction, error) {
	var coin model.Coin
	db := app.repo.ConnReader.Where("symbol = ?", strings.ToLower(event.Coin)).First(&coin)
	if db.Error != nil {
		log.Error().Err(db.Error).Str("section", "app:wallet").Str("action", "withdrawCash").Str("coin_symbol", event.Coin).Msg("Coin not found with symbol. Skipping processing withdraw")
		return nil, db.Error
	}

	precision := decimal.New(1, coin.TokenPrecision)
	amount, isAmountSet := new(decimal.Big).SetString(event.Payload["amount"])
	if !isAmountSet {
		log.Error().
			Str("section", "app:wallet").
			Str("action", "Deposit").
			Msg("Unable to set amount")
		return nil, errors.New("amount not set")
	}
	amount = new(decimal.Big).Mul(amount, precision)

	existingWithdraw, _ := app.repo.GetTransaction(event.ID)

	if existingWithdraw != nil {
		return existingWithdraw, nil
	}

	var zero *decimal.Big = new(decimal.Big)

	withdraw := model.NewWithdraw(
		event.ID,
		event.UserID,
		model.TxStatus_Confirmed,
		coin.Symbol,
		coin.Symbol,
		event.Payload["txId"],
		event.Payload["address"],
		amount,
		zero,
		1,
		model.TransactionExternalSystem(event.System),
	)

	userDetails := model.UserDetails{}
	err := app.repo.ConnReader.First(&userDetails, "user_id = ?", withdraw.UserID).Error
	if err != nil {
		return withdraw, err
	}

	tx := app.repo.Conn.Begin()
	_, err = app.ops.Deposit(tx, withdraw, zero, zero, amount, zero, model.OperationType_Withdraw)
	if err != nil {
		tx.Rollback()
		log.Error().Err(err).Str("section", "app:wallet").Str("action", "withdrawCash").Str("txid", withdraw.ID).Msg("Unable to save withdraw operation")
		return nil, err
	}

	depositAmount := withdraw.Amount.V.Quantize(coin.TokenPrecision)
	if featureflags.IsEnabled("api.wallets.send-email-for-confirmed-withdraw") {
		if withdraw != nil && withdraw.TxType == model.TxType_Withdraw && withdraw.Status == model.TxStatus_Confirmed {
			if err = w.SendEmailForWithdrawConfirmed(withdraw, userDetails.Language.String()); err != nil {
				return withdraw, err
			}
			_, err = w.SendNotification(withdraw.UserID, model.NotificationType_System,
				model.NotificationTitle_Deposit.String(),
				fmt.Sprintf(model.NotificationMessage_Deposit.String(), depositAmount),
				model.Notification_Deposit_Fiat, withdraw.ID)

			if err != nil {
				return withdraw, err
			}
		}
	}

	app.depositQueue <- *withdraw
	monitor.WithdrawCount.WithLabelValues().Inc()
	monitor.WithdrawQueued.WithLabelValues().Inc()

	return withdraw, nil
}
