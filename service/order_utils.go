package service

import (
	"fmt"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/data"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

func ConvertOrderFromModelToData(order model.Order, quotePrecision, marketPrecision uint8) data.Order {
	priceInUnits := conv.ToUnits(utils.Fmt(order.Price.V), quotePrecision)

	// @todo subtract the used amount from the order when it's loaded from the db
	amountDec := conv.NewDecimalWithPrecision()
	amountDec = amountDec.Sub(order.Amount.V, order.FilledAmount.V)
	amountInUnits := conv.ToUnits(utils.Fmt(amountDec), marketPrecision)
	stopPriceInUnits := conv.ToUnits(utils.Fmt(order.StopPrice.V), quotePrecision)
	tpPriceValue := conv.ToUnits(utils.Fmt(order.TPPrice.V), quotePrecision)
	slPriceValue := conv.ToUnits(utils.Fmt(order.SLPrice.V), quotePrecision)
	tsActivationPriceInUnits := conv.ToUnits(utils.Fmt(order.TrailingStopActivationPrice.V), quotePrecision)
	tsPriceInUnits := conv.ToUnits(utils.Fmt(order.TrailingStopPrice.V), quotePrecision)

	// @done subtract the used funds from the order when it's loaded from the db
	fundsInUnits := uint64(0)
	fundsDec := conv.NewDecimalWithPrecision()
	fundsDec = fundsDec.Sub(order.LockedFunds.V, order.UsedFunds.V)
	if order.Side == model.MarketSide_Buy {
		fundsInUnits = conv.ToUnits(fmt.Sprintf("%f", fundsDec), quotePrecision)
	} else {
		fundsInUnits = conv.ToUnits(fmt.Sprintf("%f", fundsDec), marketPrecision)
	}

	eventType := data.CommandType_NewOrder

	orderEvent := data.Order{
		ID:              order.ID,
		EventType:       eventType,
		Side:            toDataSide(order.Side),
		Type:            toDataOrderType(order.Type),
		Stop:            toDataStop(order.Stop),
		Market:          order.MarketID,
		OwnerID:         order.OwnerID,
		Amount:          amountInUnits,
		Price:           priceInUnits,
		StopPrice:       stopPriceInUnits,
		Funds:           fundsInUnits,
		TakeProfitPrice: tpPriceValue,
		StopLossPrice:   slPriceValue,
		SubAccount:      order.SubAccount,
	}

	if order.OtoType != nil {
		orderEvent.OtoType = toDataOrderType(*order.OtoType)
	}
	if order.TrailingStopPriceType != nil {
		orderEvent.TrailingStopPriceType = toDataTSStop(*order.TrailingStopPriceType)
		orderEvent.TrailingStopActivationPrice = tsActivationPriceInUnits
		orderEvent.TrailingStopPrice = tsPriceInUnits
		//trt
	}
	return orderEvent
}
