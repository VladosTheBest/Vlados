package data

import (
	proto "github.com/golang/protobuf/proto"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// ***************************
// SyncEvent Implementations
// ***************************

// FromBinary loads an event from a byte array
func (event *SyncEvent) FromBinary(msg []byte) error {
	return proto.Unmarshal(msg, event)
}

// ToBinary converts an event to a byte string
func (event *SyncEvent) ToBinary() ([]byte, error) {
	return proto.Marshal(event)
}

func (g *GenericType) GetValueAsInterface() interface{} {
	switch g.Value.(type) {
	case *GenericType_StrValue:
		return g.GetStrValue()
	case *GenericType_BigDecimalValue:
		return DecimalFromString(g.GetBigDecimalValue().Value)
	case *GenericType_Uint64Value:
		return g.GetUint64Value()
	case *GenericType_Uint32Value:
		return g.GetUint32Value()
	case *GenericType_Int64Value:
		return g.GetInt64Value()
	case *GenericType_Int32Value:
		return g.GetInt32Value()
	case *GenericType_BoolValue:
		return g.GetBoolValue()
	case *GenericType_TimestampValue:
		return ToTime(g.GetTimestampValue().Value)
	}
	return nil
}

func NewSaveDataEvent(enc Encoder, model string, item interface{}) *SyncEvent {
	return &SyncEvent{
		Model:   model,
		Command: EventCommandType_Save,
		Payload: enc.ToPayload(item),
	}
}

func NewUpdateOrderDataEvent(enc Encoder, order *model.Order) *SyncEvent {
	payload := enc.ToPayloadFields(order, []string{"id", "filled_amount", "fee_amount", "used_funds", "status", "updated_at"})
	return &SyncEvent{
		Model:   "orders",
		Command: EventCommandType_Update,
		Payload: payload,
	}
}
