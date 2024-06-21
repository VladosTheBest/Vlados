package wallet

import (
	"google.golang.org/protobuf/proto"
)

type EventType string

const (
	EventType_CreateAddress     EventType = "create_address"
	EventType_Deposit           EventType = "deposit"
	EventType_WithdrawCompleted EventType = "withdraw_completed"
	EventType_Withdraw          EventType = "withdraw"
)

// FromBinary loads a event from a byte array
func (event *Event) FromBinary(msg []byte) error {
	return proto.Unmarshal(msg, event)
}

// ToBinary converts a event to a byte string
func (event *Event) ToBinary() ([]byte, error) {
	return proto.Marshal(event)
}
