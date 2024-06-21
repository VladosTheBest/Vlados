package conv

import "github.com/ericlagergren/decimal"

var zeroRounded decimal.Big

func init() {
	zeroRounded = decimal.Big{}
	zeroRounded.Context = decimal.Context128
	zeroRounded.Context.RoundingMode = decimal.ToZero
	zeroRounded.Quantize(8)
}

// ToUnits converts the given price to uint64 units used by the trading engine
func ToUnits(amounts string, precision uint8) uint64 {
	bytes := []byte(amounts)
	size := len(bytes)
	start := false
	pointPos := 0
	var dec uint64
	i := 0
	for i = 0; i < size && (!start || (start && i-pointPos <= int(precision))); i++ {
		if !start && bytes[i] == '.' {
			start = true
			pointPos = i
		} else {
			dec = 10*dec + uint64(bytes[i]-48) // ascii char for 0
		}
	}
	if !start {
		i = 1
	}
	for i-pointPos <= int(precision) {
		dec *= 10
		i++
	}
	return dec
}

// FromUnits converts the given price to uint64 units used by the trading engine
func FromUnits(number uint64, precision uint8) string {
	bytes := []byte{48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48}
	i := 0
	for (number != 0 || i < int(precision)) && i <= 28 {
		add := uint8(number % 10)
		number /= 10
		bytes[28-i] = 48 + add
		if i == int(precision)-1 {
			i++
			bytes[28-i] = 46 // . char
		}
		i++
	}
	i--
	if bytes[28-i] == 46 {
		return string(bytes[28-i-1:])
	}

	return string(bytes[28-i:])
}

func CloneToPrecision(devAmount *decimal.Big) *decimal.Big {
	dec := &decimal.Big{}
	dec.Context = decimal.Context128
	dec.Context.RoundingMode = decimal.ToZero
	dec.Copy(devAmount)
	dec.Quantize(8)
	return dec
}

func RoundToPrecision(decAmount *decimal.Big) *decimal.Big {
	decAmount.Context = decimal.Context128
	decAmount.Context.RoundingMode = decimal.ToZero
	decAmount.Quantize(8)

	return decAmount
}

func NewDecimalWithPrecision() *decimal.Big {
	z := zeroRounded
	return &z
}
