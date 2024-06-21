package data

import (
	"reflect"
	"time"

	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/rs/zerolog/log"
)

// Encoder represents an interface which provides basic methods to convert any type to wire format
type Encoder interface {
	// ToPayload converts instance of any struct to Payload
	// the key in the payload will be the tag name of attribute
	ToPayload(item interface{}) map[string]*GenericType

	// ToPayloadFields converts instance of any struct to Payload and it will
	// only save the fields specified in the fieldNames
	// the key in the payload will be the tag name of attribute
	ToPayloadFields(item interface{}, fieldNames []string) map[string]*GenericType
}

type wireEncoder struct {
	tagName string
}

// NewWireEncoder instantiates new instance of Encoder
func NewWireEncoder(tag string) Encoder {
	return &wireEncoder{
		tagName: tag,
	}
}

func (e *wireEncoder) ToPayload(item interface{}) map[string]*GenericType {
	if reflect.TypeOf(item).Kind() == reflect.Ptr {
		item = reflect.ValueOf(item).Elem().Interface()
	}
	fields := reflect.TypeOf(item)
	values := reflect.ValueOf(item)

	logger := log.With().
		Str("section", "data").
		Str("method", "ToPayload").
		Logger()

	num := values.NumField()
	payload := make(map[string]*GenericType, num)

	for i := 0; i < num; i++ {
		field := fields.Field(i)
		value := values.Field(i)
		if value.Kind() == reflect.Ptr && value.IsNil() {
			continue
		}

		tagValue := field.Tag.Get(e.tagName)
		if tagValue == "" {
			continue
		}

		genericValue := e.fieldToGenericType(value)
		if genericValue == nil {
			if value.Type().String() != "*postgres.Decimal" {
				logger.Warn().Str("field", field.Name).Str("type", value.Type().String()).Msg("invalid type to convert")
			}
			continue
		}

		payload[tagValue] = genericValue
	}
	return payload
}

func (e *wireEncoder) ToPayloadFields(item interface{}, fieldNames []string) map[string]*GenericType {
	if reflect.TypeOf(item).Kind() == reflect.Ptr {
		item = reflect.ValueOf(item).Elem().Interface()
	}
	fields := reflect.TypeOf(item)
	values := reflect.ValueOf(item)

	logger := log.With().
		Str("section", "data").
		Str("method", "ToPayload").
		Logger()

	fieldNameMap := make(map[string]int, len(fieldNames))
	for _, fieldName := range fieldNames {
		fieldNameMap[fieldName] = 1
	}
	num := values.NumField()
	payload := make(map[string]*GenericType, num)

	for i := 0; i < num; i++ {
		field := fields.Field(i)
		value := values.Field(i)
		if value.Kind() == reflect.Ptr && value.IsNil() {
			continue
		}
		tagValue := field.Tag.Get(e.tagName)
		if tagValue == "" {
			continue
		}
		if _, ok := fieldNameMap[tagValue]; !ok {
			continue
		}

		genericValue := e.fieldToGenericType(value)
		if genericValue == nil {
			if value.Type().String() != "*postgres.Decimal" {
				logger.Warn().Str("field", field.Name).Str("type", value.Type().String()).Msg("invalid type to convert")
			}
			continue
		}
		payload[tagValue] = genericValue
	}
	return payload
}

func (e *wireEncoder) fieldToGenericType(value reflect.Value) *GenericType {
	fieldType := value.Type().String()
	switch fieldType {
	case "*postgres.Decimal":
		if value.Interface().(*postgres.Decimal).V == nil {
			return nil
		}
		return &GenericType{Value: &GenericType_BigDecimalValue{
			BigDecimalValue: &BigDecimal{Value: DecimalToString(value.Interface().(*postgres.Decimal))},
		}}
	case "uint64":
		return &GenericType{Value: &GenericType_Uint64Value{Uint64Value: value.Uint()}}
	case "uint32":
		return &GenericType{Value: &GenericType_Uint32Value{Uint32Value: uint32(value.Uint())}}
	case "int64":
		return &GenericType{Value: &GenericType_Int64Value{Int64Value: value.Int()}}
	case "int32":
		return &GenericType{Value: &GenericType_Int32Value{Int32Value: int32(value.Int())}}
	case "bool":
		return &GenericType{Value: &GenericType_BoolValue{BoolValue: value.Bool()}}
	case "time.Time":
		return &GenericType{Value: &GenericType_TimestampValue{
			TimestampValue: &Timestamp{Value: FromTime(value.Interface().(time.Time))},
		}}
	case "*uint64":
		return &GenericType{Value: &GenericType_Uint64Value{Uint64Value: value.Elem().Uint()}}
	case "*uint32":
		return &GenericType{Value: &GenericType_Uint32Value{Uint32Value: uint32(value.Elem().Uint())}}
	case "*int64":
		return &GenericType{Value: &GenericType_Int64Value{Int64Value: value.Elem().Int()}}
	case "*int32":
		return &GenericType{Value: &GenericType_Int32Value{Int32Value: int32(value.Elem().Int())}}
	case "*bool":
		return &GenericType{Value: &GenericType_BoolValue{BoolValue: value.Elem().Bool()}}
	case "*time.Time":
		return &GenericType{Value: &GenericType_TimestampValue{
			TimestampValue: &Timestamp{Value: FromTime(value.Elem().Interface().(time.Time))},
		}}
	default:
		if value.Kind() == reflect.String {
			return &GenericType{Value: &GenericType_StrValue{StrValue: value.String()}}
		} else if value.Kind() == reflect.Ptr && value.Elem().Kind() == reflect.String {
			return &GenericType{Value: &GenericType_StrValue{StrValue: value.Elem().String()}}
		} else if value.Kind() == reflect.Int {
			return &GenericType{Value: &GenericType_Int64Value{Int64Value: value.Int()}}
		} else if value.Kind() == reflect.Uint {
			return &GenericType{Value: &GenericType_Uint64Value{Uint64Value: value.Uint()}}
		} else if value.Kind() == reflect.Ptr && value.Elem().Kind() == reflect.Int {
			return &GenericType{Value: &GenericType_Int64Value{Int64Value: value.Elem().Int()}}
		} else if value.Kind() == reflect.Ptr && value.Elem().Kind() == reflect.Uint {
			return &GenericType{Value: &GenericType_Uint64Value{Uint64Value: value.Elem().Uint()}}
		}
		return nil
	}
}
