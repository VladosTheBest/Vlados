package order_canceled_confirmation

import (
	"google.golang.org/protobuf/proto"
)

// FromBinary loads an event from a byte array
func (event *OrderCanceledConfirmation) FromBinary(msg []byte) error {
	return proto.Unmarshal(msg, event)
}

// ToBinary converts an event to a byte string
// func (event *OrderCanceledConfirmation) ToBinary() ([]byte, error) {
// 	 return proto.MarshalOptions{UseCachedSize: true, Deterministic: true}.Marshal(event)
// }

func (event *OrderCanceledConfirmation) IsCancelled() bool {
	switch event.Status {
	case OrderCanceledStatus_AlreadyFilled,
		OrderCanceledStatus_AlreadyCancelled,
		OrderCanceledStatus_OK:
		return true
	default:
		return false
	}
}
