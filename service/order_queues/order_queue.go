package order_queues

/**
 * Queue orders creation before sending them for processing to ensure no double spending
 */

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Unleash/unleash-client-go/v3"
	"github.com/ericlagergren/decimal"
	"github.com/rs/zerolog/log"
	coins "gitlab.com/paramountdax-exchange/exchange_api_v2/apps/coinsLocal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/fms"

	marketCache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/markets"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/logger"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/monitor"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/ops"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

var ErrInsufficientFunds = errors.New("Insufficient Funds")

type order_queues struct {
	repo                  *queries.Repo
	BonusAccountRiskLevel *decimal.Big
	ops                   *ops.Ops
	FundsEngine           *fms.FundsEngine
	coinValues            *coins.App
	lock                  *sync.Mutex
}

var orderQueues = new(order_queues)

func init() {
	orderQueues.lock = &sync.Mutex{}
}

// SetRepo for order queues
func SetRepo(repo *queries.Repo) {
	orderQueues.repo = repo
}

func SetCurrentRiskLevel(riskLevel *decimal.Big) {
	orderQueues.BonusAccountRiskLevel = riskLevel
}

// SetOps sets the ops
func SetOps(ops *ops.Ops) {
	orderQueues.ops = ops
}

func SetFundsEngine(fmsInstance *fms.FundsEngine) {
	orderQueues.FundsEngine = fmsInstance
}

func SetCoinsRates(c *coins.App) {
	orderQueues.coinValues = c
}

// CreateOrder - process an order one by one and add it to the exchange based on available funds
func CreateOrder(ctx context.Context, order *model.Order, marketPrice *decimal.Big) error {
	monitor.QueuedOrderRequests.WithLabelValues(order.MarketID).Inc()
	err := orderQueues.create_order(ctx, order, marketPrice)
	monitor.QueuedOrderRequests.WithLabelValues(order.MarketID).Dec()
	if err == nil {
		monitor.OrdersCreatedCount.WithLabelValues(order.MarketID).Inc()
	}
	return err
}

func (queue *order_queues) create_order(ctx context.Context, order *model.Order, marketPrice *decimal.Big) error {
	// get the user from order
	// user := queue.repo.GetUserByID(order.OwnerID)
	market, err := marketCache.Get(order.MarketID)
	if err != nil {
		return err
	}

	var symbol string
	symbol = market.QuoteCoinSymbol
	if order.Side == model.MarketSide_Sell {
		symbol = market.MarketCoinSymbol
	}

	_, err = market.GetCrossMinMarket()
	if err != nil {
		return err
	}

	_, err = market.GetCrossMinQuote()
	if err != nil {
		return err
	}

	// -> lock the process to only process one order at a time
	// orderQueues.lock.Lock()
	// defer orderQueues.lock.Unlock()

	logger.LogTimestamp(ctx, "queue_process_start", time.Now())

	// -> lock to get user subaccount
	account, err := subAccounts.GetUserSubAccountByID(model.MarketTypeSpot, order.OwnerID, order.SubAccount)
	if err != nil {
		return err
	}

	// calculate
	if order.CanCalculateFundsWithoutBalance() {
		order.CalculateFundsForSellOrLimit()
	} else {

		// -> lock twice to get user account balance
		accountBalances, err := queue.FundsEngine.GetAccountBalances(order.OwnerID, account.ID)
		if err != nil {
			return err
		}

		// -> lock account balances
		accountBalances.LockAccount()

		balance, err := accountBalances.GetAvailableBalanceForCoin(symbol)
		if err != nil {
			accountBalances.UnlockAccount()
			return err
		}

		var oppositeBalance *decimal.Big
		if order.IsStrangleOrStraddleOrderType() {
			var oppositeSymbol string
			if symbol == market.MarketCoinSymbol {
				oppositeSymbol = market.QuoteCoinSymbol
			} else {
				oppositeSymbol = market.MarketCoinSymbol
			}
			// get the user's balance for that coin
			oppositeBalance, err = accountBalances.GetAvailableBalanceForCoin(oppositeSymbol)
			if err != nil {
				accountBalances.UnlockAccount()
				return err
			}
		}

		if oppositeBalance != nil {
			order.OppositeFunds.V = conv.NewDecimalWithPrecision().Copy(oppositeBalance)
		}
		accountBalances.UnlockAccount()
		order.CalculateFundsForBuyOrMarket(balance, marketPrice)
	}

	// check if the order is valid against the market
	// if err = order.IsValid(minMarketVolume, minQuoteVolume, market); err != nil {
	// 	logger.LogCustomValue(ctx, "user_id", strconv.FormatUint(order.OwnerID, 10))
	// 	return err
	// }

	if err = queue.orderRiskChecks(market, order, account); err != nil {
		return err
	}

	// execute add order operation and subtract from the user's balance the used funds
	_, err = queue.ops.AddOrder(order, symbol, market)

	// add time log
	logger.LogTimestamp(ctx, "queue_order_added", time.Now())

	return err
}

func (queue *order_queues) orderRiskChecks(market *model.Market, order *model.Order, account *model.SubAccount) error {
	if account.AccountGroup == model.AccountGroupBonus {
		if !unleash.IsEnabled(fmt.Sprintf("api.bonus-account.market.%s", market.ID)) {
			return fmt.Errorf("orders not allowed on %s/%s market", market.MarketCoinSymbol, market.QuoteCoinSymbol)
		}

		if unleash.IsEnabled("api.bonus-account.orders.high_risk.checking") {
			riskLevel, err := queue.getCurrentRiskLevel(market, order, account)
			if err != nil {
				return errors.New("unable to check the risk level")
			}

			log.Info().Str("level", riskLevel.String()).Msg("Risk Level")

			if unleash.IsEnabled("api.bonus-account.orders.restriction") {
				if riskLevel.Cmp(queue.BonusAccountRiskLevel) > 0 {
					return errors.New("high risk")
				}
			}
		}
	}

	if unleash.IsEnabled("api.order.create:abnormal_price:checking") {
		if order.Type == model.OrderType_Limit && market.MarketCoinSymbol != "prdx" {
			log.Warn().
				Str("section", "service").
				Str("action", "CreateOrder").
				Str("method", "orderRiskChecks").
				Msg("1. HTTP Call to external service for 'GetLastPriceForTheMarket'")
			currentExternalPrice, err := queue.coinValues.GetLastPriceForTheMarket(market.ID)
			if err != nil {
				log.Error().Err(err).Str("section", "service").Str("action", "CreateOrder").
					Str("method", "GetLastPriceForPair").Msg("Unable to get price limits")
				return err
			}

			log.Warn().
				Str("section", "service").
				Str("action", "CreateOrder").
				Str("method", "orderRiskChecks").
				Msg("1. Call to database for 'GetPriceLimits'")
			minPriceDec, maxPriceDec, err := queue.repo.GetPriceLimits()
			if err != nil {
				log.Error().Err(err).Str("section", "service").Str("action", "CreateOrder").
					Str("method", "GetPriceLimits").Msg("Unable to get price limits")
				return err
			}

			res := conv.NewDecimalWithPrecision().Quo(currentExternalPrice, order.Price.V)
			fmt.Println(res.String())
			diffFrom := res.Cmp(minPriceDec)
			diffTo := res.Cmp(maxPriceDec)

			if diffFrom < 0 || diffTo > 0 {
				log.Info().
					Str("section", "service").
					Str("action", "CreateOrder").
					Str("method", "GetLastPriceForPair").
					Str("price_cryptoCompare", currentExternalPrice.String()).
					Str("price_on_the_order", order.Price.V.String()).
					Str("inPercentage", res.String()).
					Interface("order", order).
					Msg("Abnormal order price")
				if unleash.IsEnabled("api.order.create:abnormal_price:restriction") {
					return errors.New("abnormal order price")
				}
			}
		}
	}

	return nil
}

func (queue *order_queues) getCurrentRiskLevel(market *model.Market, order *model.Order, account *model.SubAccount) (*decimal.Big, error) {
	currentCoin := market.QuoteCoinSymbol
	if order.Side == model.MarketSide_Sell {
		currentCoin = market.MarketCoinSymbol
	}

	rates, err := queue.coinValues.GetAll()

	if err != nil {
		return nil, err
	}

	totalAvailable := conv.NewDecimalWithPrecision()
	totalBonusAmount := conv.NewDecimalWithPrecision()
	totalContractsAmount := conv.NewDecimalWithPrecision()

	log.Warn().
		Str("section", "service").
		Str("action", "CreateOrder").
		Str("method", "getCurrentRiskLevel").
		Msg("1. Call to database for 'GetLiabilityBalances'")
	balances, err := queue.repo.GetLiabilityBalances(order.OwnerID, account)
	if err != nil {
		return nil, err
	}

	for coin, balance := range balances {
		if balance.Available.Cmp(model.ZERO) <= 0 {
			continue
		}

		crossRate, ok := rates[strings.ToUpper(coin)][strings.ToUpper(currentCoin)]
		if !ok {
			continue
		}

		if conv.NewDecimalWithPrecision().CheckNaNs(balance.Available, crossRate) {
			continue
		}

		converted := conv.NewDecimalWithPrecision().Mul(balance.GetTotal(), crossRate)
		totalAvailable = conv.NewDecimalWithPrecision().Add(totalAvailable, converted)
	}

	log.Warn().
		Str("section", "service").
		Str("action", "CreateOrder").
		Str("method", "getCurrentRiskLevel").
		Msg("2. Call to database for 'GetBonusAccountContracts'")
	contracts, err := queue.repo.GetBonusAccountContracts(order.OwnerID, 0, 0, "", 0, []model.BonusAccountContractStatus{
		model.BonusAccountContractStatusActive,
	})

	if err != nil {
		return nil, err
	}

	if len(contracts) == 0 {
		return nil, errors.New("you don't have active contracts")
	}

	for _, contract := range contracts {
		coin := contract.CoinSymbol
		amount := contract.Amount.V

		crossRate, ok := rates[strings.ToUpper(coin)][strings.ToUpper(currentCoin)]
		if !ok {
			continue
		}

		if conv.NewDecimalWithPrecision().CheckNaNs(amount, crossRate) {
			continue
		}

		converted := conv.NewDecimalWithPrecision().Mul(amount, crossRate)
		totalContractsAmount.Add(totalContractsAmount, converted)

		totalBonusConverted := conv.NewDecimalWithPrecision().Mul(contract.GetFullBonusAmount(), crossRate)
		totalBonusAmount.Add(totalBonusAmount, totalBonusConverted)
	}

	// check that client take coins with unmarket price
	take := conv.NewDecimalWithPrecision().Mul(order.Amount.V, rates[strings.ToUpper(market.MarketCoinSymbol)][strings.ToUpper(market.QuoteCoinSymbol)])
	receive := conv.NewDecimalWithPrecision()

	if order.Type == model.OrderType_Market || (order.IsCustomOrderType() && *order.OtoType == model.OrderType_Market) {
		receive.Copy(take)
	} else {
		receive.Mul(order.Amount.V, order.Price.V)
	}

	totalAvailable.Sub(totalAvailable, totalBonusAmount)
	totalAvailable.Sub(totalAvailable, take)
	totalAvailable.Add(totalAvailable, receive)

	riskLevel := conv.NewDecimalWithPrecision().Quo(totalAvailable, totalContractsAmount)
	riskLevel = conv.NewDecimalWithPrecision().Sub(conv.NewDecimalWithPrecision().SetFloat64(1), riskLevel)
	riskLevel = conv.NewDecimalWithPrecision().Abs(riskLevel)
	return riskLevel, nil
}
