package poloniex

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/avdva/turnpike"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/siddontang/go/log"
)

var (
	decType  = reflect.TypeOf(decimal.Decimal{})
	timeType = reflect.TypeOf(time.Time{})

	obookDecodeHook = mapstructure.ComposeDecodeHookFunc(decimalDecodeHook, timeDecodeHook)
)

// OrderBookUpd is a single order book update.
type OrderBookUpd struct {
	// OpType is either orderBookModify or orderBookRemove.
	OpType string
	// Type is either bid or ask.
	Type string
	// Rate is a price.
	Rate decimal.Decimal
	// Amount can be zero, if OpType == orderBookRemove.
	Amount decimal.Decimal
}

// TradeUpd contains single trade information.
type TradeUpd struct {
	// OpType = newTrade
	OpType string
	// TradeID - unique trade ID.
	TradeID string
	// Rate is a price.
	Rate decimal.Decimal
	// Amount is a trade amount.
	Amount decimal.Decimal
	// Date is a trade's timestamp.
	Date time.Time
	// Total is the remaning amount(?).
	Total decimal.Decimal
	// Type	is either buy or sell.
	Type string
}

// MarketUpd is a message from Poloniex exchange
type MarketUpd struct {
	// Seq is constantly increasing number.
	Seq int64
	// Obooks - updates or order book.
	Obooks []OrderBookUpd
	// Trades - new trades.
	Trades []TradeUpd
}

func makeOBookSubHandler(updatesCh chan<- MarketUpd) turnpike.EventHandler {
	return func(args []interface{}, kwargs map[string]interface{}) {
		var (
			upd      MarketUpd
			oBookUpd OrderBookUpd
			tradeUpd TradeUpd
		)
		obookDec, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Result:     &oBookUpd,
			DecodeHook: obookDecodeHook,
		})
		tradeDec, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Result:     &tradeUpd,
			DecodeHook: obookDecodeHook,
		})
		if arg, found := kwargs["seq"]; found {
			if jVal, ok := arg.(json.Number); ok {
				if intVal, err := jVal.Int64(); err == nil {
					upd.Seq = intVal
				} else {
					log.Errorf("poloniex: seq parse error: %v", err)
				}
			}
		}
		if upd.Seq == 0 {
			log.Errorf("poloniex: invalid seq; %v", kwargs)
		}
		for _, iface := range args {
			m, ok := iface.(map[string]interface{})
			if !ok {
				log.Errorf("poloniex: invalid message type: %v", iface)
				continue
			}
			typ, ok := m["type"].(string)
			if !ok {
				log.Errorf("poloniex: invalid message type: %v", m["type"])
				continue
			}
			switch typ {
			case "orderBookModify":
				fallthrough
			case "orderBookRemove":
				oBookUpd.OpType = typ
				if err := obookDec.Decode(m["data"]); err == nil {
					upd.Obooks = append(upd.Obooks, oBookUpd)
				} else {
					log.Errorf("poloniex: order book update decode error: %v", err)
				}
			case "newTrade":
				tradeUpd.OpType = typ
				if err := tradeDec.Decode(m["data"]); err == nil {
					upd.Trades = append(upd.Trades, tradeUpd)
				} else {
					log.Errorf("poloniex: trade update decode error: %v", err)
				}
			default:
				log.Errorf("poloniex: unknown message type: %s", typ)
			}
		}
		updatesCh <- upd
	}
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

func timeDecodeHook(from reflect.Type, to reflect.Type, v interface{}) (interface{}, error) {
	if to == timeType {
		if str, ok := v.(string); ok {
			return time.ParseInLocation("2006-01-02 15:04:05", str, time.UTC)
		}
		return nil, errors.Errorf("cannot decode %s to time.Time", from.String())
	}
	return v, nil
}
