package poloniex

import (
	"reflect"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

var (
	decType         = reflect.TypeOf(decimal.Decimal{})
	obookDecodeHook = mapstructure.ComposeDecodeHookFunc(decimalDecodeHook)
)

const (
	Sell = OpType(0)
	Buy  = OpType(1)
)

// OpType is either buy or sell.
type OpType int

// OrderBookUpd is a single order book update.
type OrderBookUpd struct {
	// Type is either bid or ask.
	Type OpType
	// Price of an asset.
	Price decimal.Decimal
	// Size can be zero, if the order was removed.
	Size decimal.Decimal
}

// TradeUpd contains single trade information.
type TradeUpd struct {
	// TradeID - unique trade ID.
	TradeID string
	// Type is either buy or sell
	Type OpType
	// Price is an asset price.
	Price decimal.Decimal
	// Size is a trade amount.
	Size decimal.Decimal
	// Date is a trade's unix timestamp.
	Date int64
}

// MarketUpd is a message from Poloniex exchange.
type MarketUpd struct {
	// Seq is constantly increasing number.
	Seq int64
	// Obooks - updates of an order book.
	Obooks []OrderBookUpd
	// Trades - new trades.
	Trades []TradeUpd
}

// TickerUpd is a ticker update message.
type TickerUpd struct {
	Pair string
	Ticker
}

func decimalDecodeHook(from reflect.Type, to reflect.Type, v interface{}) (interface{}, error) {
	if to == decType {
		if str, ok := v.(string); ok {
			return decimal.NewFromString(str)
		}
		return nil, errors.Errorf("cannot decode %s to decimal", from.String())
	}
	return v, nil
}
