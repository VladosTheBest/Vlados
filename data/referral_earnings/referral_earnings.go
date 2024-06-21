package referral_earnings

import (
	"google.golang.org/protobuf/proto"
)

// FromBinary loads an event from a byte array
func (event *AddReferralEarningsFromUser) FromBinary(msg []byte) error {
	return proto.Unmarshal(msg, event)
}

// ToBinary converts an event to a byte string
func (event *AddReferralEarningsFromUser) ToBinary() ([]byte, error) {
	return proto.Marshal(event)
}
