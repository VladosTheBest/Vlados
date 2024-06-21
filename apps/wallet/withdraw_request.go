package wallet

import (
	"context"
	"errors"
	"sync"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/fms"

	"github.com/ericlagergren/decimal"
	gouuid "github.com/nu7hatch/gouuid"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/monitor"
)

var ErrInsufficientFunds = errors.New("Insufficient Funds")
var ErrUnknownAmountFormat = errors.New("Unknown format for amount")

// WithdrawRequest struct
type WithdrawRequest struct {
	RespChan chan<- WithdrawResponse
	Data     *model.WithdrawRequest
}

// WithdrawResponse struct
type WithdrawResponse struct {
	Status string
	Error  error
	Data   *model.WithdrawRequest
}

// AddWithdrawRequest add a new withdraw request for a user
func (app *App) AddWithdrawRequest(userID uint64, coin string, amount, fee *decimal.Big, to string, fromAccount *model.SubAccount, externalSystem model.WithdrawExternalSystem, data string, accountBalances *fms.AccountBalances) (*model.WithdrawRequest, error) {
	// get balances for the current user
	logger := log.With().Str("section", "app:wallet").
		Str("action", "AddWithdrawRequest").
		Uint64("user_id", userID).
		Uint64("sub_account", fromAccount.ID).
		Logger()

	if !fromAccount.WithdrawalAllowed {
		logger.Error().
			Msg("Unable to withdraw from current account. Restricted by account")

		return nil, errors.New("withdraws from this account restricted")
	}

	balance, err := accountBalances.GetAvailableBalanceForCoin(coin)
	if err != nil {
		logger.Error().Err(err).
			Msg("Unable to create WithdrawRequest - get user liability")

		return nil, err
	}

	if balance == nil {
		logger.Error().
			Msg("Insufficient funds - balance")

		return nil, ErrInsufficientFunds
	}

	// check if the user has the necessary balances to make the withdraw
	totalAmount := (&decimal.Big{}).Add(amount, fee)
	if totalAmount.Cmp(balance) == 1 {
		logger.Error().
			Msg("Insufficient funds - not enough amount")

		return nil, ErrInsufficientFunds
	}

	request := model.NewWithdrawRequest(userID, coin, amount, fee, to, data, "", externalSystem)
	u, _ := gouuid.NewV4()
	request.ID = u.String()

	// try to create the withdraw request
	err = app.repo.Create(&request)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("Unable to create WithdrawRequest")

		return nil, err
	}

	return request, nil
}

// StartWithdrawLoop - create a goroutine for processing withdraw requests in the background
func (app *App) StartWithdrawLoop(ctx context.Context, wait *sync.WaitGroup) {
	app.processWithdrawRequests(ctx, wait)
}

func (app *App) AcceptWithdrawRequest(withdrawRequest *model.WithdrawRequest) (*model.WithdrawRequest, error) {
	respChan := make(chan WithdrawResponse)
	app.withdrawQueue <- WithdrawRequest{
		RespChan: respChan,
		Data:     withdrawRequest}

	resp := <-respChan
	return resp.Data, resp.Error
}

// Read new withdraw requests from the queue and process them one by one
func (app *App) processWithdrawRequests(ctx context.Context, wait *sync.WaitGroup) {
	log.Info().Str("worker", "withdraw_requests").Str("action", "start").Msg("Withdraw requests processor - started")
	for {
		select {
		case wr := <-app.withdrawQueue:
			app.processWithdrawRequest(wr)
		case <-ctx.Done():
			log.Info().Str("worker", "withdraw_requests").Str("action", "stop").Msg("21 => Withdraw requests processor - stopped")
			wait.Done()
			return
		}
	}
}

// process a new withdraw request
func (app *App) processWithdrawRequest(wr WithdrawRequest) {
	response := WithdrawResponse{Error: nil, Data: nil}
	respChan := wr.RespChan
	request := wr.Data
	// get balances for the current user

	account, err := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, wr.Data.UserID, model.AccountGroupMain)
	if err != nil {
		response.Error = err
		log.Error().Err(err).
			Str("section", "app:wallet").
			Str("action", "processWithdrawRequest").
			Uint64("user_id", request.UserID).
			Str("amount", request.GetAmount().String()).
			Str("fee", request.GetFeeAmount().String()).
			Msg("Unable to get user account")
		respChan <- response
		return
	}

	err = app.ops.FundsEngine.DoNewWithdraw(request.UserID, account.ID, request.CoinSymbol, request.GetAmount(), request.GetFeeAmount())
	if err != nil {
		log.Error().Err(err).
			Str("section", "app:wallet").
			Str("action", "processWithdrawRequest").
			Uint64("user_id", request.UserID).
			Str("amount", request.GetAmount().String()).
			Str("fee", request.GetFeeAmount().String()).
			// Str("locked_balance", balance.String()).
			Msg("Insufficient funds")
		response.Error = err
		respChan <- response
	}
	_, err = app.ops.AcceptWithdrawRequest(request, request.CoinSymbol, request.UserID)
	if err != nil {
		log.Error().Err(err).
			Str("section", "app:wallet").
			Str("action", "processWithdrawRequest").
			Uint64("user_id", request.UserID).
			Msg("Unable to create WithdrawRequest")
		// revert withdraw request if withdrawal failed
		_ = app.ops.FundsEngine.DoRevertWithdraw(request.UserID, account.ID, request.CoinSymbol, request.GetAmount(), request.GetFeeAmount())
		response.Error = err
		respChan <- response
		return
	}

	response.Data = request
	respChan <- response
	app.withdrawRequestQueue <- *request
	monitor.WithdrawCount.WithLabelValues().Inc()
	monitor.WithdrawQueued.WithLabelValues().Inc()
}
