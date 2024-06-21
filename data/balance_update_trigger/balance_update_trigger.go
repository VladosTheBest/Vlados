package balance_update_trigger

import (
	"google.golang.org/protobuf/proto"
)

// FromBinary loads an event from a byte array
func (event *BalanceUpdateEvent) FromBinary(msg []byte) error {
	return proto.Unmarshal(msg, event)
}

// ToBinary converts an event to a byte string
func (event *BalanceUpdateEvent) ToBinary() ([]byte, error) {
	return proto.MarshalOptions{UseCachedSize: true, Deterministic: true}.Marshal(event)
}
