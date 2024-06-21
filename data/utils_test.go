package data

import (
	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/go-playground/assert/v2"
	"testing"
	"time"
)

func TestDecimalConversion(t *testing.T) {
	testCases := []*postgres.Decimal{
		DecimalFromString("123.123"),
		DecimalFromString("123456.0123456789"),
		DecimalFromString("123456789.0000123456"),
		nil,
	}
	for _, test := range testCases {
		s := DecimalToString(test)
		d := DecimalFromString(s)
		if test == nil {
			assert.Equal(t, d, nil)
			continue
		}
		assert.Equal(t, test.V.Cmp(d.V), 0)
	}
}

func TestTimeConversion(t *testing.T) {
	testCases := []time.Time{
		time.Now(),
		time.Now().Add(time.Hour * 1241234),
		time.Now().Add(time.Minute * 12813499),
	}
	for _, test := range testCases {
		assert.Equal(t, test.Equal(ToTime(FromTime(test))), true)
	}
}
