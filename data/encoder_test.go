package data

import (
	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/go-playground/assert/v2"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"testing"
	"time"
)

func TestTypeToPayloadFields(t *testing.T) {
	testData := getTestModel()
	enc := NewWireEncoder("wire")
	payload := enc.ToPayloadFields(&testData, []string{"id", "data_int_small", "data_bool", "data_enum", "data_decimal", "data_str_ptr", "data_time_ptr"})

	assert.Equal(t, testData.ID, payload["id"].GetValueAsInterface())
	assert.Equal(t, testData.DataIntSmall, payload["data_int_small"].GetValueAsInterface())
	assert.Equal(t, testData.DataBool, payload["data_bool"].GetValueAsInterface())
	assert.Equal(t, testData.DataEnum.String(), payload["data_enum"].GetValueAsInterface())
	assert.Equal(t, testData.DataDecimal.V.Cmp(payload["data_decimal"].GetValueAsInterface().(*postgres.Decimal).V), 0)

	assert.Equal(t, testData.DataStrPtr, payload["data_str_ptr"].GetValueAsInterface())
	assert.Equal(t, testData.DataTimePtr.Equal(payload["data_time_ptr"].GetValueAsInterface().(time.Time)), true)

	assert.Equal(t, payload["data_enum_ptr"], nil)
	assert.Equal(t, payload["id_ptr"], nil)
	assert.Equal(t, payload["id_small"], nil)
}

func TestTypeToPayload(t *testing.T) {
	testData := getTestModel()
	enc := NewWireEncoder("wire")
	payload := enc.ToPayload(testData)

	assert.Equal(t, testData.ID, payload["id"].GetValueAsInterface())
	assert.Equal(t, testData.IDSmall, payload["id_small"].GetValueAsInterface())
	assert.Equal(t, testData.DataInt, payload["data_int"].GetValueAsInterface())
	assert.Equal(t, testData.DataIntSmall, payload["data_int_small"].GetValueAsInterface())
	assert.Equal(t, testData.DataBool, payload["data_bool"].GetValueAsInterface())
	assert.Equal(t, testData.DataStr, payload["data_str"].GetValueAsInterface())
	assert.Equal(t, testData.DataEnum.String(), payload["data_enum"].GetValueAsInterface())
	assert.Equal(t, testData.DataDecimal.V.Cmp(payload["data_decimal"].GetValueAsInterface().(*postgres.Decimal).V), 0)
	assert.Equal(t, testData.DataTime.Equal(payload["data_time"].GetValueAsInterface().(time.Time)), true)
	assert.Equal(t, int64(testData.DataEnumInt), payload["data_enum_int"].GetInt64Value())

	assert.Equal(t, testData.IDPtr, payload["id_ptr"])
	assert.Equal(t, testData.IDSmallPtr, payload["id_small_ptr"].GetValueAsInterface())
	assert.Equal(t, testData.DataIntPtr, payload["data_int_ptr"].GetValueAsInterface())
	assert.Equal(t, testData.DataIntSmallPtr, payload["data_int_small_ptr"].GetValueAsInterface())
	assert.Equal(t, testData.DataBoolPtr, payload["data_bool_ptr"].GetValueAsInterface())
	assert.Equal(t, testData.DataStrPtr, payload["data_str_ptr"].GetValueAsInterface())
	assert.Equal(t, testData.DataEnumPtr.String(), payload["data_enum_ptr"].GetValueAsInterface())
	assert.Equal(t, testData.DataTimePtr.Equal(payload["data_time_ptr"].GetValueAsInterface().(time.Time)), true)
	assert.Equal(t, uint64(*testData.DataEnumUintPtr), payload["data_enum_uint_ptr"].GetValueAsInterface())
}

func TestDecimalToPayload(t *testing.T) {
	testData := getTestModel()
	enc := NewWireEncoder("wire")
	testData.DataDecimal = nil

	payload := enc.ToPayload(testData)
	assert.Equal(t, payload["data_decimal"], nil)

	testData.DataDecimal = &postgres.Decimal{V: nil}
	payload = enc.ToPayload(testData)
	assert.Equal(t, payload["data_decimal"], nil)

	testData.DataDecimal = DecimalFromString("123.1223")
	payload = enc.ToPayload(testData)
	assert.Equal(t, testData.DataDecimal.V.Cmp(payload["data_decimal"].GetValueAsInterface().(*postgres.Decimal).V), 0)
}

func TestPayloadWithNilValues(t *testing.T) {
	testData := getTestModel()
	testData.IDPtr = nil
	testData.IDSmallPtr = nil
	testData.DataIntPtr = nil
	testData.DataIntSmallPtr = nil
	testData.DataBoolPtr = nil
	testData.DataStrPtr = nil
	testData.DataEnumPtr = nil
	testData.DataTimePtr = nil
	testData.DataEnumUintPtr = nil
	testData.DataDecimal = nil

	enc := NewWireEncoder("wire")

	ptrFeilds := []string{
		"id_ptr", "id_small_ptr", "data_int_ptr", "data_int_small_ptr",
		"data_bool_ptr", "data_str_ptr", "data_enum_ptr", "data_time_ptr",
		"data_enum_uint_ptr", "data_decimal",
	}
	payload := enc.ToPayloadFields(testData, ptrFeilds)
	assert.Equal(t, len(payload), 0)

	testData.DataDecimal = &postgres.Decimal{V: nil}
	payload = enc.ToPayloadFields(testData, ptrFeilds)
	assert.Equal(t, len(payload), 0)
}

type testEnumInt int

const (
	testEnumIntA testEnumInt = iota
	testEnumIntB
)

type testEnumUint uint

const (
	testEnumUintA testEnumUint = iota
	testEnumUintB
)

type testModel struct {
	ID           uint64            `wire:"id"`
	IDSmall      uint32            `wire:"id_small"`
	DataInt      int64             `wire:"data_int"`
	DataIntSmall int32             `wire:"data_int_small"`
	DataBool     bool              `wire:"data_bool"`
	DataStr      string            `wire:"data_str"`
	DataEnum     model.OrderType   `wire:"data_enum"`
	DataDecimal  *postgres.Decimal `wire:"data_decimal"`
	DataTime     time.Time         `wire:"data_time"`
	DataEnumInt  testEnumInt       `wire:"data_enum_int"`

	IDPtr           *uint64          `wire:"id_ptr"`
	IDSmallPtr      *uint32          `wire:"id_small_ptr"`
	DataIntPtr      *int64           `wire:"data_int_ptr"`
	DataIntSmallPtr *int32           `wire:"data_int_small_ptr"`
	DataBoolPtr     *bool            `wire:"data_bool_ptr"`
	DataStrPtr      *string          `wire:"data_str_ptr"`
	DataEnumPtr     *model.OrderType `wire:"data_enum_ptr"`
	DataTimePtr     *time.Time       `wire:"data_time_ptr"`
	DataEnumUintPtr *testEnumUint    `wire:"data_enum_uint_ptr"`

	DataNonTag string
}

func getTestModel() testModel {
	var (
		idSmallPtr      uint32 = 987
		dataIntPtr      int64  = 789
		dataIntSmallPtr int32  = 987
		dataBoolPtr            = true
		dataStrPtr             = "str ptr data"
		dataEnumPtr            = model.OrderType_Limit
		dataTimePtr            = time.Now()
		dataEnumUintPtr        = testEnumUintB
	)

	return testModel{
		ID:           123,
		IDSmall:      456,
		DataInt:      -123,
		DataIntSmall: -456,
		DataBool:     true,
		DataStr:      "str data",
		DataEnum:     model.OrderType_Market,
		DataDecimal:  DecimalFromString("123456.654321"),
		DataTime:     time.Now(),
		DataEnumInt:  testEnumIntB,

		IDPtr:           nil,
		IDSmallPtr:      &idSmallPtr,
		DataIntPtr:      &dataIntPtr,
		DataIntSmallPtr: &dataIntSmallPtr,
		DataBoolPtr:     &dataBoolPtr,
		DataStrPtr:      &dataStrPtr,
		DataEnumPtr:     &dataEnumPtr,
		DataTimePtr:     &dataTimePtr,
		DataEnumUintPtr: &dataEnumUintPtr,

		DataNonTag: "hello world",
	}
}
