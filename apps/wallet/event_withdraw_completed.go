package wallet

import (
	"github.com/ericlagergren/decimal"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	data "gitlab.com/paramountdax-exchange/exchange_api_v2/data/wallet"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"strconv"
	"strings"
)

// WithdrawCompleted handler
func (app *App) WithdrawCompleted(event *data.Event) error {
	// get coin and fee coin
	var coin, feeCoin model.Coin
	db := app.repo.Conn.Where("symbol = ?", strings.ToLower(event.Coin)).First(&coin)
	if db.Error != nil {
		log.Error().Err(db.Error).
			Str("section", "app:wallet").Str("action", "withdraw_complete").
			Str("coin_symbol", strings.ToLower(event.Coin)).
			Msg("Unable to find coin used for a withdraw event")
		return db.Error
	}
	db = app.repo.Conn.Where("symbol = ?", strings.ToLower(coin.CostSymbol)).First(&feeCoin)
	if db.Error != nil {
		log.Error().Err(db.Error).
			Str("section", "app:wallet").Str("action", "withdraw_complete").
			Str("cost_symbol", strings.ToLower(coin.CostSymbol)).
			Msg("Unable to find cost coin using for a withdraw event")
		return db.Error
	}

	// load info from event payload
	status := event.Payload["status"]
	txid := event.Payload["txid"]
	withdrawRequestID := event.Payload["withdraw_request_id"]
	precision := decimal.New(1, coin.TokenPrecision)
	precisionFee := decimal.New(1, feeCoin.TokenPrecision)
	amount, _ := conv.NewDecimalWithPrecision().SetString(event.Payload["amount"])
	amount = conv.NewDecimalWithPrecision().Mul(amount, precision)
	fee, _ := conv.NewDecimalWithPrecision().SetString(event.Payload["fee"])
	fee = conv.NewDecimalWithPrecision().Mul(fee, precisionFee)
	confirmations, _ := strconv.Atoi(event.Payload["confirmations"])

	// update status of withdrawal
	var withdrawRequest *model.WithdrawRequest
	var err error
	if withdrawRequestID != "" {
		withdrawRequest, err = app.repo.GetWithdrawRequest(withdrawRequestID)
		if err != nil {
			return err
		}
		withdrawRequest.TxID = txid
		if status == "confirmed" {
			withdrawRequest.Status = model.WithdrawStatus_Completed
		} else {
			withdrawRequest.Status = model.WithdrawStatus_InProgress
		}
	} else {
		log.Error().
			Str("section", "app:wallet").Str("action", "withdraw_complete").
			Str("event_id", event.ID).
			Msg("Unknown withdraw event. Possible hack")
		return nil
	}

	_, err = app.repo.GetTransaction(event.ID)
	if err == nil {
		log.Warn().
			Str("section", "app:wallet").Str("action", "withdraw_complete").
			Str("event_id", event.ID).
			Msg("Withdraw transaction already exists. Skipping")
		return nil
	}

	address := withdrawRequest.To
	tx := model.NewWithdraw(
		event.ID,
		event.UserID,
		model.TxStatus_Confirmed,
		coin.Symbol, feeCoin.Symbol, txid, address,
		amount,
		fee,
		confirmations,
		model.TransactionExternalSystem(event.System),
	)

	err = app.ops.Withdraw(withdrawRequest, tx)
	return err
}
