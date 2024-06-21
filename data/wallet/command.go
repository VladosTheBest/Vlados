package wallet

import (
	"google.golang.org/protobuf/proto"
)

// FromBinary loads a command from a byte array
func (command *Command) FromBinary(msg []byte) error {
	return proto.Unmarshal(msg, command)
}

// ToBinary converts a command to a byte string
func (command *Command) ToBinary() ([]byte, error) {
	return proto.Marshal(command)
}
