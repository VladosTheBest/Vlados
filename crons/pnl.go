package crons

import (
	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/rs/zerolog/log"
	coins "gitlab.com/paramountdax-exchange/exchange_api_v2/apps/coinsLocal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
	"strings"
	"time"
)

func CronUpdateBalance24h(repo *queries.Repo, coin *coins.App) {
	logger := log.With().
		Str("section", "crons").
		Str("method", "CronUpdateBalance24h").
		Logger()
	balances, err := repo.GetAllBalances()
	if err != nil {
		logger.Error().Err(err).Msg("Unable to get balances")
		return
	}

	crossRates, err := coin.GetAll()
	if err != nil {
		logger.Error().Err(err).Msg("Unable to get cross rates")
		return
	}

	yesterdayBalance24h, err := repo.GetBalances24hOneDayAbove()
	if err != nil {
		logger.Error().Err(err).Msg("Unable to get all yesterday's balances24h")
		return
	}

	var balances24h []model.Balance24h
	for _, balance := range balances {
		balance24h := model.Balance24h{
			UserID:     balance.UserID,
			SubAccount: balance.SubAccount,
			CoinSymbol: balance.CoinSymbol,
			Total:      &postgres.Decimal{V: conv.NewDecimalWithPrecision().Add(balance.Available.V, balance.Locked.V)},
			CreatedAt:  time.Now().Truncate(24 * time.Hour),
			UpdatedAt:  time.Now(),
			Percent:    &postgres.Decimal{V: conv.NewDecimalWithPrecision()},
		}
		if balance24h.Total.V != nil && crossRates[strings.ToUpper(balance.CoinSymbol)]["USDT"] != nil {
			totalCross := conv.NewDecimalWithPrecision().Mul(crossRates[strings.ToUpper(balance.CoinSymbol)]["USDT"], balance24h.Total.V)
			balance24h.TotalCross = &postgres.Decimal{V: totalCross}
		} else {
			balance24h.TotalCross = &postgres.Decimal{V: model.Zero}
		}

		for _, b24hYesterday := range yesterdayBalance24h {
			if balance24h.CoinSymbol == b24hYesterday.CoinSymbol && balance24h.SubAccount == b24hYesterday.SubAccount && balance24h.UserID == b24hYesterday.UserID {
				if b24hYesterday.TotalCross != nil || conv.NewDecimalWithPrecision().CheckNaNs(b24hYesterday.TotalCross.V, nil) {
					difference := conv.NewDecimalWithPrecision().Sub(balance24h.TotalCross.V, b24hYesterday.TotalCross.V)
					percent := conv.NewDecimalWithPrecision().Quo(difference, b24hYesterday.TotalCross.V)
					percent = percent.Mul(percent, conv.NewDecimalWithPrecision().SetUint64(100)).RoundToInt()
					balance24h.Percent = &postgres.Decimal{V: percent}
				} else {
					balance24h.Percent = &postgres.Decimal{V: model.ZERO}
				}
			}
		}
		balances24h = append(balances24h, balance24h)
	}

	if len(balances24h) > 0 {
		db := repo.Conn.Begin()
		for _, balance24h := range balances24h {
			if err := db.Table("balances_24h").Create(&balance24h).Error; err != nil {
				logger.Error().Err(err).Msg("Unable to create balances_24h")
				db.Rollback()
				return
			}
		}
		if err := db.Commit().Error; err != nil {
			logger.Error().Err(err).Msg("Unable to insert in to balances_24h")
		}
	}
}
