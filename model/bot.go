package model

import (
	"encoding/json"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	postgresDialects "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/lib/pq"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

type BotType string
type BotStatus string
type BotOrder string
type BotOrderBy string

const (
	BotTypeNone  BotType = ""
	BotTypeGrid  BotType = "grid"
	BotTypeTrend BotType = "trend"

	BotStatusActive                 BotStatus = "active"
	BotStatusStopped                BotStatus = "stopped"
	BotStatusStoppedBySystemTrigger BotStatus = "stopped_by_system_trigger"
	BotStatusLiquidated             BotStatus = "liquidated"
	BotStatusArchived               BotStatus = "archived"

	BotAsc  BotOrder = "asc"
	BotDesc BotOrder = "desc"

	BotOrderByCreate BotOrderBy = "create"
	BotOrderByExpire BotOrderBy = "expire"
)

func (t BotType) IsTurnedOn() bool {
	return t.IsValid() && t != BotTypeNone
}

func (t BotType) String() string {
	return string(t)
}

func (t BotType) IsValid() bool {
	switch t {
	case BotTypeGrid, BotTypeTrend, BotTypeNone:
		return true
	default:
		return false
	}
}

func (fromStatus BotStatus) IsValid() bool {
	switch fromStatus {
	case BotStatusActive,
		BotStatusStopped,
		BotStatusArchived,
		BotStatusStoppedBySystemTrigger:
		return true
	default:
		return false
	}
}

func (b BotOrder) IsValid() bool {
	switch b {
	case BotAsc,
		BotDesc,
		"":
		return true
	default:
		return false
	}
}

func (b BotOrderBy) IsValid() bool {
	switch b {
	case BotOrderByCreate,
		BotOrderByExpire,
		"":
		return true
	default:
		return false
	}
}

func (fromStatus BotStatus) IsStatusAllowed(toStatus BotStatus, isSystem bool) bool {
	switch fromStatus {
	case BotStatusActive:
		switch toStatus {
		case BotStatusStoppedBySystemTrigger:
			return isSystem
		case BotStatusArchived, BotStatusStopped, BotStatusLiquidated:
			return true
		default:
			return false
		}
	case BotStatusArchived:
		switch toStatus {
		case BotStatusStoppedBySystemTrigger:
			return isSystem
		case BotStatusStopped, BotStatusLiquidated:
			return true
		default:
			return false
		}
	case BotStatusStopped:
		switch toStatus {
		case BotStatusStoppedBySystemTrigger:
			return isSystem
		case BotStatusActive, BotStatusArchived, BotStatusLiquidated:
			return true
		default:
			return false
		}
	case BotStatusStoppedBySystemTrigger:
		return isSystem
	case BotStatusLiquidated:
		return false
	default:
		return false
	}
}

type Bot struct {
	ID         uint64            `json:"id,omitempty"`
	UserId     uint64            `json:"user_id"`
	Status     BotStatus         `json:"status"`
	Type       BotType           `json:"type"`
	SubAccount uint64            `json:"sub_account"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	RefID      string            `gorm:"column:ref_id" json:"-"`
	Amount     *postgres.Decimal `json:"amount"`
	CoinSymbol string            `gorm:"column:coin_symbol" json:"coin_symbol"`
}

type BotVersion struct {
	ID          uint64                 `json:"id,omitempty"`
	BotId       uint64                 `json:"-"`
	BotSystemID string                 `json:"bot_system_id"`
	Settings    postgresDialects.Jsonb `json:"settings"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type BotSettingsGrid struct {
	BotId         uint64       `json:"botId"`
	UserAccountId string       `json:"userAccountId"`
	CurrencyPair  string       `json:"currencyPair"`
	MinPrice      *decimal.Big `json:"minPrice"`
	CurrentPrice  *decimal.Big `json:"currentPrice"`
	MaxPrice      *decimal.Big `json:"maxPrice"`
	Amount        *decimal.Big `json:"amount"`
	NoOfGrids     int          `json:"noOfGrids"`
	ProfitPercent *decimal.Big `json:"profitPercent"`
	Coin          string       `json:"coin"`
}

type BotSettingsTrend struct {
	BotId                  uint64       `json:"botId"`
	UserAccountId          string       `json:"userAccountId"`
	CurrencyPair           string       `json:"currencyPair"`
	CurrentPrice           *decimal.Big `json:"currentPrice"`
	Amount                 *decimal.Big `json:"amount"`
	Coin                   string       `json:"coin"`
	BuyEntryDistance       *decimal.Big `json:"buyEntryDistance"`
	BuyTakeProfitDistance  *decimal.Big `json:"buyTakeProfitDistance"`
	BuySaveLossesDistance  *decimal.Big `json:"buySaveLossesDistance"`
	SellEntryDistance      *decimal.Big `json:"sellEntryDistance"`
	SellTakeProfitDistance *decimal.Big `json:"sellTakeProfitDistance"`
	SellSaveLossesDistance *decimal.Big `json:"sellSaveLossesDistance"`
	EntryPriceUpdateRateS  int          `json:"entryPriceUpdateRateS,omitempty"`
	OrderCount             int          `json:"orderCount"`
}

type BotWithVersions struct {
	*Bot        `json:",inline"`
	UserEmail   string        `json:"user_email" gorm:"column:email"`
	ContractID  uint64        `json:"contract_id"`
	BotVersions []*BotVersion `json:"versions" gorm:"-"`
}

type BotWithVersionsWithContract struct {
	*Bot        `json:",inline"`
	UserEmail   string        `json:"user_email" gorm:"column:email"`
	ContractID  uint64        `json:"contract_id"`
	ExpiredAt   time.Time     `json:"expired_at"`
	BotVersions []*BotVersion `json:"versions" gorm:"-"`
}

type BotPnl struct {
	BotID             uint64            `json:"bot_id"`
	BotVersionsID     uint64            `json:"versions,omitempty" gorm:"column:id"`
	InitialDeposit    *postgres.Decimal `json:"initial_deposit"`
	CoinSymbolDeposit string            `json:"coin_symbol_deposit"`
	Profit            *postgres.Decimal `json:"profit" gorm:"profit"`
	ProfitCoin        string            `json:"profit_coin" gorm:"profit_coin"`
	BotProfit         *postgres.Decimal `json:"bot_profit"`
}

type BotWithTotalActiveBotsAndLockFunds struct {
	BotWithVersion     []*BotWithVersions `json:"bots"`
	TotalBots          int                `json:"total_bots"`
	TotalOfLockedFunds *decimal.Big       `json:"total_of_locked_funds"`
}

type BotWithVersionWithMeta struct {
	BotWithVersion []*BotWithVersions `json:"bots"`
	Meta           PagingMeta
}

type BotWithVersionWithContractWithMeta struct {
	BotWithVersionWithContract []*BotWithVersionsWithContract `json:"bots"`
	Meta                       PagingMeta
}

type BotWithRangePrice struct {
	BotID      uint64    `json:"bot_id"`
	Email      string    `json:"email"`
	Status     string    `json:"status"`
	ContractID uint64    `json:"contract_id"`
	Pair       string    `json:"pair"`
	MinPrice   float32   `json:"min_price"`
	MaxPrice   float32   `json:"max_price"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiredAT  time.Time `json:"expired_at"`
}

type BotWithStatistics struct {
	Average      float32                         `json:"average"`
	Total        int                             `json:"total"`
	BotRange     float32                         `json:"bot_range"`
	BotWithPrice map[string][]*BotWithRangePrice `json:"bots"`
}

type BotTrendStatistic struct {
	BotID         uint64            `json:"bot_id"`
	Email         string            `json:"email"`
	ContractID    uint64            `json:"contract_id"`
	ExpiredAT     time.Time         `json:"expired_at"`
	Pair          string            `json:"pair"`
	Profit        *postgres.Decimal `json:"profit"`
	ProfitPercent *postgres.Decimal `json:"profit_percent"`
}

type BotWithStatisticsWithMeta struct {
	BotWithStatistics []BotWithStatistics
	Meta              PagingMeta
}

type BotWithTrendStatisticsWithMeta struct {
	BotWithStatistics []BotTrendStatistic
	Meta              PagingMeta
}

type BotCreateUpdateRequest struct {
	ID       uint64    `json:"id,omitempty"`
	Settings string    `json:"settings"`
	Type     BotType   `json:"type"`
	Status   BotStatus `json:"status,omitempty"`
}

type BotStatusChangeRequest struct {
	ID     uint64
	Status BotStatus
}

type BotAnalytics struct {
	ID            uint64            `json:"id,omitempty"`
	BotId         uint64            `json:"bot_id"`
	BotSystemID   string            `json:"bot_system_id"`
	Type          BotType           `json:"type"`
	Orders        pq.Int64Array     `gorm:"type:integer[]" json:"orders"`
	ProfitPercent *postgres.Decimal `gorm:"profit_percent" sql:"type:decimal(36,18)" json:"profitPercent"`
	Profit        *postgres.Decimal `gorm:"profit" sql:"type:decimal(36,18)" json:"profit"`
	ProfitCoin    string            `gorm:"profit_coin" json:"profitCoin"`
	Version       uint64            `json:"version"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

func (b *BotWithTotalActiveBotsAndLockFunds) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"bots": b.BotWithVersion,
	})
}

func (b *BotAnalytics) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":             b.ID,
		"bot_id":         b.BotId,
		"bot_system_id":  b.BotSystemID,
		"type":           b.Type,
		"orders":         b.Orders,
		"profit_percent": utils.FmtDecimalWithPrecision(b.ProfitPercent, 4),
		"profit":         utils.FmtDecimal(b.Profit),
		"profit_coin":    b.ProfitCoin,
		"version":        b.Version,
		"created_at":     b.CreatedAt,
	})
}

func (b *BotWithRangePrice) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"bot_id":      b.BotID,
		"email":       b.Email,
		"status":      b.Status,
		"contract_id": b.ContractID,
		"pair":        b.Pair,
		"min_price":   b.MinPrice,
		"max_price":   b.MaxPrice,
		"created_at":  b.CreatedAt,
		"expired_at":  b.ExpiredAT,
	})
}

type BotAnalyticsRequest struct {
	ID            uint64        `json:"id,omitempty"`
	BotId         uint64        `json:"bot_id"`
	BotSystemID   string        `json:"botSystemID"`
	Type          BotType       `json:"type"`
	Orders        pq.Int64Array `gorm:"type:integer[]" json:"orders"`
	ProfitPercent *decimal.Big  `gorm:"profit_percent" sql:"type:decimal(36,18)" json:"profitPercent"`
	Profit        *decimal.Big  `gorm:"profit" sql:"type:decimal(36,18)" json:"profit"`
	ProfitCoin    string        `gorm:"profit_coin" json:"profitCoin"`
	Version       uint64        `json:"version"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

func (b *BotPnl) MarshalJSON() ([]byte, error) {
	precision := 4
	return json.Marshal(map[string]interface{}{
		"bot_id":              b.BotID,
		"bot_version_id":      b.BotVersionsID,
		"initial_deposit":     utils.FmtDecimalWithPrecision(b.InitialDeposit, precision),
		"coin_symbol_deposit": b.CoinSymbolDeposit,
		"profit":              utils.FmtDecimalWithPrecision(b.Profit, precision),
		"profit_coin":         b.ProfitCoin,
		"bot_profit":          utils.FmtDecimalWithPrecision(b.BotProfit, precision),
	})
}

func (b *BotTrendStatistic) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"bot_id":         b.BotID,
		"email":          b.Email,
		"contract_id":    b.ContractID,
		"expired_at":     b.ExpiredAT,
		"pair":           b.Pair,
		"profit":         utils.FmtDecimal(b.Profit),
		"profit_percent": utils.FmtDecimal(b.ProfitPercent),
	})
}

type BotRebalanceEvents struct {
	ID             uint64            `json:"id,omitempty"`
	BotID          uint64            `gorm:"bot_id" json:"bot_id"`
	FromSubAccount uint64            `json:"from_sub_account"`
	FromCoinSymbol string            `gorm:"from_coin_symbol" json:"from_coin_symbol"`
	FromAmount     *postgres.Decimal `gorm:"from_amount" sql:"type:decimal(36,18)" json:"from_amount"`
	ToSubAccount   uint64            `json:"to_sub_account"`
	ToAmount       *postgres.Decimal `gorm:"to_amount" sql:"type:decimal(36,18)" json:"to_amount"`
	ToCoinSymbol   string            `gorm:"to_coin_symbol" json:"to_coin_symbol"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}
