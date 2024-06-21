package model

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// CoinsStats Amount owed to a user
type CoinsStats struct {
	LastLiabilityID    uint64
	LiabilityAvailable float64 `sql:"type:decimal(36,18)"`
	LiabilityLocked    float64 `sql:"type:decimal(36,18)"`

	LastAssetsID    uint64
	AssetsAvailable float64 `sql:"type:decimal(36,18)"`
	AssetsLocked    float64 `sql:"type:decimal(36,18)"`

	LastExpensesID    uint64
	ExpensesAvailable float64 `sql:"type:decimal(36,18)"`
	ExpensesLocked    float64 `sql:"type:decimal(36,18)"`

	LastRevenuesID    uint64
	RevenuesAvailable float64 `sql:"type:decimal(36,18)"`
	RevenuesLocked    float64 `sql:"type:decimal(36,18)"`

	CoinSymbol     string
	BTCValue       string
	Fee            float64
	TokenPrecision int
}

// MarshalJSON JSON encoding of a cs entry
func (cs CoinsStats) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{

		"last_liability_id":  cs.LastLiabilityID,
		"liabilities":        strconv.FormatFloat(cs.LiabilityAvailable, 'f', 18, 64),
		"liabilities_locked": strconv.FormatFloat(cs.LiabilityLocked, 'f', 18, 64),

		"last_assets_id": cs.LastAssetsID,
		"assets":         strconv.FormatFloat(cs.AssetsAvailable, 'f', 18, 64),
		"assets_locked":  strconv.FormatFloat(cs.AssetsLocked, 'f', 18, 64),

		"last_expenses_id": cs.LastExpensesID,
		"expenses":         strconv.FormatFloat(cs.ExpensesAvailable, 'f', 18, 64),
		"expenses_locked":  strconv.FormatFloat(cs.ExpensesLocked, 'f', 18, 64),

		"last_revenues_id": cs.LastRevenuesID,
		"profit":           strconv.FormatFloat(cs.RevenuesAvailable, 'f', 18, 64),
		"profit_locked":    strconv.FormatFloat(cs.RevenuesLocked, 'f', 18, 64),

		"coin":      cs.CoinSymbol,
		"btc_value": cs.BTCValue,
		"fee":       fmt.Sprintf(`%v`, cs.TokenPrecision),
	})
}
