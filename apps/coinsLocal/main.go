package coinsLocal

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ericlagergren/decimal"
	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/httpagent"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type CoinValueData map[string]map[string]*decimal.Big
type CoinValueDecimal map[string]*decimal.Big

// App structure hold information about the active markets
type App struct {
	coinValues    CoinValueData
	urlCoinsValue string
	urlLastPrices string
	lock          *sync.RWMutex
}

// NewApp create a new application that can hold information about the active markets
func NewApp(urlCoinsValue, urlLastPrices string, ctx context.Context) *App {
	var app = &App{
		urlCoinsValue: urlCoinsValue,
		urlLastPrices: urlLastPrices,
		lock:          &sync.RWMutex{},
	}

	return app
}

func (app *App) Start(ctx context.Context, wait *sync.WaitGroup) {
	log.Info().Str("cron", "coin_values").Str("action", "start").Msg("Coin values worker - started")
	ticker := time.NewTicker(200 * time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			log.Info().Str("cron", "coin_values").Str("action", "stop").Msg("9 => Coin values worker - stopped")
			wait.Done()
			return
		case <-ticker.C:
			if err := app.update(); err != nil {
				log.Error().Err(err).Str("section", "service").Str("action", "CoinsLocal").
					Str("method", "update").Msg("Unable to get cross rates from CoinValuesAPI")
			}
		}
	}
}

// Update all list of the coins values
func (app *App) update() error {
	code, body, err := httpagent.Get(app.urlCoinsValue)
	if err != nil {
		return err
	}

	if code != http.StatusOK {
		return fmt.Errorf("invalid status code: %d (%s)", code, http.StatusText(code))
	}
	coins := CoinValueData{}
	jsonErr := json.Unmarshal(body, &coins)
	if jsonErr != nil {
		return jsonErr
	}
	app.lock.Lock()
	app.coinValues = coins
	app.lock.Unlock()
	return nil
}

// GetAll return a list of all markets
func (app *App) GetAll() (CoinValueData, error) {
	app.lock.RLock()
	if app.coinValues == nil {
		app.lock.RUnlock()
		if err := app.update(); err != nil {
			return nil, err
		}
	} else {
		app.lock.RUnlock()
	}
	coins := CoinValueData{}
	app.lock.RLock()
	for k, row := range app.coinValues {
		coins[k] = CoinValueDecimal{}
		for i, v := range row {
			newV := &decimal.Big{}
			newV.Copy(v)
			coins[k][i] = newV
		}
	}
	app.lock.RUnlock()
	return coins, nil
}

func (app *App) GetLastPriceForTheMarket(marketID string) (*decimal.Big, error) {
	code, body, err := httpagent.Get(fmt.Sprintf("%s/%s", app.urlLastPrices, marketID))
	if err != nil {
		log.Error().Err(err).Str("section", "service").Str("action", "CreateOrder").
			Str("method", "GetLastPriceForPair").Msg("Unable to get current price from CoinValuesAPI")
		return nil, err
	}

	if code != http.StatusOK {
		return nil, fmt.Errorf("invalid status code: %d (%s)", code, http.StatusText(code))
	}

	var resp decimal.Big
	jsonErr := json.Unmarshal(body, &resp)
	if jsonErr != nil {
		return nil, jsonErr
	}

	return &resp, nil
}
