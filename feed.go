package poloniex

type OrderBookUpd struct {
	OpType string
	Rate   string
	Amount string
	Type   string
}

type TradeUpd struct {
	OpType  string
	TradeID string
	Rate    string
	Amount  string
	Date    string
	Total   string
	Type    string
}

type MarketUpd struct {
	Seq    int64
	Obooks []OrderBookUpd
	Trades []TradeUpd
}
