package data

import (
	"github.com/ericlagergren/decimal/sql/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"time"
)

func DecimalFromString(s string) *postgres.Decimal {
	if s == "" {
		return nil
	}
	d, _ := conv.NewDecimalWithPrecision().SetString(s)
	return &postgres.Decimal{V: d}
}

func DecimalToString(d *postgres.Decimal) string {
	if d == nil {
		return ""
	}
	return d.V.String()
}

func ToTime(t int64) time.Time {
	return time.Unix(0, t).UTC()
}

func FromTime(t time.Time) int64 {
	return t.UTC().UnixNano()
}
