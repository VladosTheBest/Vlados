package data

import "google.golang.org/protobuf/proto"

/*
*
MarketDepthLevel1
=================

Return the top level market depth for bids, asks and last trade:
- bid_price
- bid_volume
- ask_price
- ask_volume
- last_price
- last_volume
*/
type MarketDepthLevel1 struct {
	BidPrice   string `json:"bid_price"`
	BidVolume  string `json:"bid_volume"`
	AskPrice   string `json:"ask_price"`
	AskVolume  string `json:"ask_volume"`
	LastPrice  string `json:"last_price"`
	LastVolume string `json:"last_volume"`
}

/*
*
MarketDepthLevel2
=================

Returns a list of all top depth data for bids and asks for a market.
*/
type MarketDepthLevel2 struct {
	Bid [][2]string `json:"bids"`
	Ask [][2]string `json:"asks"`
}

// FromBinary loads a market backup from a byte array
func (m *MarketDepth) FromBinary(msg []byte) error {
	return proto.Unmarshal(msg, m)
}

// ToBinary converts a market backup to a byte string
func (m *MarketDepth) ToBinary() ([]byte, error) {
	return proto.Marshal(m)
}
