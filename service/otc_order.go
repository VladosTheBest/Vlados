package service

import (
	"fmt"
	"github.com/ericlagergren/decimal"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/otc_desk"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"strconv"
)

func (service *Service) CreateOTCOrderQuote(primary, secondary, side, amountStr, commissionStr string) (*otc_desk.QuoteResponse, error) {
	logger := log.With().Str("method", "CreateOTCOrderQuote").Logger()

	orderSide := model.MarketSide(side)
	if !orderSide.IsValid() {
		err := fmt.Errorf("invalid order side")
		logger.Error().Str("side", side).Msg(err.Error())
		return nil, err
	}
	commission, err := strconv.ParseFloat(commissionStr, 64)
	if err != nil {
		logger.Error().Err(err).Str("commission", commissionStr).Msg("unable secondary parse commission percentage")
		return nil, err
	}
	if commission > 5.0 {
		err := fmt.Errorf("OTC order commission should be less than or equal secondary 5")
		logger.Error().Str("commission", commissionStr).Msg(err.Error())
		return nil, err
	}

	primaryCoin, err := service.GetCoin(primary)
	if err != nil {
		logger.Error().Err(err).Str("coin", primary).Msg("unable secondary get coin")
		return nil, err
	}

	_, err = service.GetCoin(secondary)
	if err != nil {
		logger.Error().Err(err).Str("coin", secondary).Msg("unable secondary get coin")
		return nil, err
	}

	coinPrecision := uint8(primaryCoin.TokenPrecision)
	amountInUnits := conv.ToUnits(amountStr, coinPrecision)
	amountAsDecimal := new(decimal.Big)
	amountAsDecimal.SetString(conv.FromUnits(amountInUnits, coinPrecision))

	req := otc_desk.NewQuoteRequest(primary, secondary, amountAsDecimal, commission, orderSide)
	return service.otcDesk.RequestForQuote(req)
}
