package model

type MarketDepthLevel1 struct {
	Timestamp  int64  `json:"timestamp"`
	BidPrice   string `json:"bid_price"`
	BidVolume  string `json:"bid_volume"`
	AskPrice   string `json:"ask_price"`
	AskVolume  string `json:"ask_volume"`
	LastPrice  string `json:"last_price"`
	LastVolume string `json:"last_volume"`
}

type MarketDepthLevel2 struct {
	Timestamp  int64       `json:"timestamp"`
	LastPrice  string      `json:"last_price"`
	LastVolume string      `json:"last_volume"`
	Bids       [][2]string `json:"bids"`
	Asks       [][2]string `json:"asks"`
}

type MarketDepthLevel2CGK struct {
	TickerID  string      `json:"ticker_id"`
	Timestamp int64       `json:"timestamp"`
	Bids      [][2]string `json:"bids"`
	Asks      [][2]string `json:"asks"`
}
