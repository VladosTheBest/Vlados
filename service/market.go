package service

import (
	"fmt"
	"github.com/ericlagergren/decimal"
	marketCache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/markets"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"strings"
)

// GetMarketsByStatus - get all markets by status
func (service *Service) GetMarketsByStatus(status model.MarketStatus) ([]*model.Market, error) {
	return service.GetCachedMarketsByStatus(status), nil
}

// GetMarket - get single market
func (service *Service) GetMarket(id string) (*model.Market, error) {
	return marketCache.Get(id)
}

// ListMarkets - get all markets added to the database
func (service *Service) ListMarkets() ([]*model.Market, error) {
	return marketCache.GetAllActive(), nil
}

// GetMarketsDetailed - get markets paged
func (service *Service) GetMarketsDetailed(limit, page int, query string) (*model.MarketList, error) {
	markets := make([]model.Market, 0)
	var rowCount int64

	db := service.repo.Conn.Table("markets")

	if len(query) > 0 {
		squery := "%" + query + "%"
		db = db.Where("name ILIKE ? OR market_coin_symbol ILIKE ? OR quote_coin_symbol ILIKE ?", squery, squery, squery)
	}

	dbc := db.Count(&rowCount)

	if dbc.Error != nil {
		return nil, dbc.Error
	}

	db = db.Order("created_at DESC")
	if limit == 0 {
		db = db.Find(&markets)
	} else {
		db = db.Limit(limit).Offset((page - 1) * limit).Find(&markets)
	}

	marketsList := model.MarketList{
		Markets: markets,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: make(map[string]interface{})},
	}

	return &marketsList, db.Error
}

// CreateMarket - save a new market based in the given markets and configuration
func (service *Service) CreateMarket(
	name, marketSymbol, quoteSymbol string,
	status model.MarketStatus,
	mPrec, qPrec, mPrecFormat, qPrecFormat int,
	minMVol, minQVol, maxMPrice, maxQPrice, maxUSDTSpendLimit *decimal.Big,
) (*model.Market, error) {
	market := model.NewMarket(strings.ToLower(marketSymbol+quoteSymbol), name, marketSymbol, quoteSymbol, status, mPrec, qPrec, mPrecFormat, qPrecFormat, minMVol, minQVol, maxMPrice, maxQPrice, maxUSDTSpendLimit)
	err := service.repo.Create(market)

	return market, err
}

// UpdateMarket - Update information about a market
func (service *Service) UpdateMarket(
	market *model.Market,
	name string,
	status model.MarketStatus,
	mPrec, qPrec, mPrecFormat, qPrecFormat int,
	minMVol, minQVol, maxMPrice, maxQPrice, maxUSDTSpendLimit *decimal.Big) (*model.Market, error) {
	market.Name = name
	market.MarketPrecisionFormat = mPrecFormat
	market.QuotePrecisionFormat = qPrecFormat
	market.MinMarketVolume.V = minMVol
	market.MinQuoteVolume.V = minQVol
	market.MaxMarketPrice.V = maxMPrice
	market.MaxQuotePrice.V = maxQPrice
	market.Status = status
	market.MaxUSDTSpendLimit.V = maxUSDTSpendLimit

	err := service.repo.Update(market)
	if err != nil {
		return nil, err
	}

	return market, nil
}

// DeleteMarket - removes a market from the database
func (service *Service) DeleteMarket(market *model.Market) error {
	market.Status = model.MarketStatusDisabled

	db := service.repo.Conn.Model(market).Update("status", model.MarketStatusDisabled)

	return db.Error
}

// GetMarketByID - get market by id
func (service *Service) GetMarketByID(id string) (*model.Market, error) {
	return marketCache.Get(id)
}

// SetMarketPairFavorite - set or unset favorite market pair
func (service *Service) SetMarketPairFavorite(id uint64, pair string) (*model.UserFavoriteMarketPairs, error) {
	favoritePair := model.UserFavoriteMarketPairs{}
	db := service.repo.Conn.First(&favoritePair, "user_id = ? AND pair = ?", id, pair)
	if db.Error != nil { //no record found, create it
		favPair := model.NewUserFavoriteMarketPairs(id, pair)
		err := service.repo.Create(favPair)
		return favPair, err
	}

	//when found, remove
	db = service.repo.Conn.Delete(favoritePair)
	return &favoritePair, db.Error
}

// GetMarketPairFavorites - get user's favorite market pairs
func (service *Service) GetMarketPairFavorites(id uint64) ([]model.UserFavoriteMarketPairs, error) {
	favoritePairs := make([]model.UserFavoriteMarketPairs, 0)
	db := service.repo.ConnReader.Find(&favoritePairs, "user_id = ?", id)

	return favoritePairs, db.Error
}

// LoadMarketIDs return the list of market ids
func (service *Service) LoadMarketIDs() ([]string, error) {
	markets := marketCache.GetAll()
	ids := make([]string, 0)
	for _, market := range markets {
		ids = append(ids, market.ID)
	}
	return ids, nil
}

func (service *Service) LoadMarketIDsByCoin(marketCoinSymbol, quoteCoinSymbol string) ([]string, error) {
	allMarkets := marketCache.GetAll()
	ids := make([]string, 0)

	for _, market := range allMarkets {
		// Фильтрация по символам монет
		if (marketCoinSymbol == "all" || market.MarketCoinSymbol == marketCoinSymbol) &&
			(quoteCoinSymbol == "all" || market.QuoteCoinSymbol == quoteCoinSymbol) {
			ids = append(ids, market.ID)
		}
	}

	return ids, nil
}

// LoadActiveMarketIDs return the list of active market ids
func (service *Service) LoadActiveMarketIDs() ([]string, error) {
	markets := service.GetCachedMarketsByStatus(model.MarketStatusActive)
	ids := make([]string, 0, len(markets))
	for _, market := range markets {
		ids = append(ids, market.ID)
	}
	return ids, nil
}

// GetCachedMarketsByStatus godoc
// Load cached markets by status
func (service *Service) GetCachedMarketsByStatus(status model.MarketStatus) []*model.Market {
	markets := marketCache.GetAll()
	filtered := make([]*model.Market, 0, len(markets))
	for i := range markets {
		if markets[i].Status == status {
			filtered = append(filtered, markets[i])
		}
	}
	return filtered
}

func (service *Service) SetMarketHighlight(market *model.Market, switcher string) error {

	var highlight bool
	if switcher == "on" {
		highlight = true
	}

	if ok := marketCache.SetHighlight(market.ID, highlight); !ok {
		return fmt.Errorf("unable to get market with id %s", market.ID)
	}

	err := service.repo.Conn.Model(market).Update("highlight", highlight).Error

	return err
}
