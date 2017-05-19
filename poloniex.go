// Package Poloniex is an implementation of the Poloniex API in Golang.
package poloniex

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/avdva/turnpike"
)

const (
	API_BASE                   = "https://poloniex.com/"  // Poloniex API endpoint
	API_WS                     = "wss://api.poloniex.com" // Poloniex WS endpoint
	DEFAULT_HTTPCLIENT_TIMEOUT = 30 * time.Second         // HTTP client timeout
)

// New return a instantiate poloniex struct
func New(apiKey, apiSecret string) *Poloniex {
	client := NewClient(apiKey, apiSecret)
	return &Poloniex{client}
}

// poloniex represent a poloniex client
type Poloniex struct {
	client *client
}

// GetTickers is used to get the ticker for all markets
func (b *Poloniex) GetTickers() (tickers map[string]Ticker, err error) {
	r, err := b.client.do("GET", "public?command=returnTicker", "", false)
	if err != nil {
		return
	}
	if err = json.Unmarshal(r, &tickers); err != nil {
		return
	}
	return
}

// GetVolumes is used to get the volume for all markets
func (b *Poloniex) GetVolumes() (vc VolumeCollection, err error) {
	r, err := b.client.do("GET", "public?command=return24hVolume", "", false)
	if err != nil {
		return
	}
	if err = json.Unmarshal(r, &vc); err != nil {
		return
	}
	return
}

func (b *Poloniex) GetCurrencies() (currencies Currencies, err error) {
	r, err := b.client.do("GET", "public?command=returnCurrencies", "", false)
	if err != nil {
		return
	}
	if err = json.Unmarshal(r, &currencies.Pair); err != nil {
		return
	}
	return
}

// GetOrderBook is used to get retrieve the orderbook for a given market
// market: a string literal for the market (ex: BTC_NXT). 'all' not implemented.
// cat: bid, ask or both to identify the type of orderbook to return.
// depth: how deep of an order book to retrieve
func (b *Poloniex) GetOrderBook(market, cat string, depth int) (orderBook OrderBook, err error) {
	// not implemented
	if cat != "bid" && cat != "ask" && cat != "both" {
		cat = "both"
	}
	if depth > 100 {
		depth = 100
	}
	if depth < 1 {
		depth = 1
	}

	r, err := b.client.do("GET", fmt.Sprintf("public?command=returnOrderBook&currencyPair=%s&depth=%d", strings.ToUpper(market), depth), "", false)
	if err != nil {
		return
	}
	if err = json.Unmarshal(r, &orderBook); err != nil {
		return
	}
	if orderBook.Error != "" {
		err = errors.New(orderBook.Error)
		return
	}
	return
}

// Returns candlestick chart data. Required GET parameters are "currencyPair",
// "period" (candlestick period in seconds; valid values are 300, 900, 1800,
// 7200, 14400, and 86400), "start", and "end". "Start" and "end" are given in
// UNIX timestamp format and used to specify the date range for the data
// returned.
func (b *Poloniex) ChartData(currencyPair string, period int, start, end time.Time) (candles []*CandleStick, err error) {
	r, err := b.client.do("GET", fmt.Sprintf(
		"/public?command=returnChartData&currencyPair=%s&period=%d&start=%d&end=%d",
		strings.ToUpper(currencyPair),
		period,
		start.Unix(),
		end.Unix(),
	), "", false)
	if err != nil {
		return
	}

	if err = json.Unmarshal(r, &candles); err != nil {
		return
	}

	return
}

// SubscribeOrderBook subscribes for trades and order book updates via WAMP.
//	symbol - a symbol you are interested in.
//	updatesCh - a channel for market updates.
//	stopCh - a channel to cancel or reset ws subscribtion.
//		close it or send 'true' to stop subscribtion.
//		send 'false' to reconnect. May be useful, if updates were stalled.
func (b *Poloniex) SubscribeOrderBook(symbol string, updatesCh chan<- MarketUpd, stopCh <-chan bool) error {
	tmDialer := func(network, addr string) (net.Conn, error) {
		return net.DialTimeout(network, addr, DEFAULT_HTTPCLIENT_TIMEOUT)
	}
	f := func() (cont bool, err error) {
		var client *turnpike.Client
		client, err = turnpike.NewWebsocketClient(turnpike.JSONNUMBER, API_WS, nil, tmDialer)
		if err != nil {
			return
		}
		defer func() {
			go client.Close()
		}()
		if _, err = client.JoinRealm("realm1", nil); err != nil {
			return
		}
		if err = client.Subscribe(symbol, nil, makeOBookSubHandler(updatesCh)); err != nil {
			return
		}
		val, ok := <-stopCh
		if val || !ok {
			return
		}
		return true, nil
	}
	for {
		if cont, err := f(); !cont {
			return err
		}
	}
}

// SubscribeTicker subscribes for ticker via WAMP.
// Send to, or close stopCh to cancel subscribtion.
//	updatesCh - a channel for ticker updates.
//	stopCh - a channel to cancel or reset ws subscribtion.
//		close it or send 'true' to stop subscribtion.
//		send 'false' to reconnect. May be useful, if updates were stalled.
func (b *Poloniex) SubscribeTicker(updatesCh chan<- TickerUpd, stopCh <-chan bool) error {
	tmDialer := func(network, addr string) (net.Conn, error) {
		return net.DialTimeout(network, addr, DEFAULT_HTTPCLIENT_TIMEOUT)
	}
	f := func() (cont bool, err error) {
		var client *turnpike.Client
		client, err = turnpike.NewWebsocketClient(turnpike.JSONNUMBER, API_WS, nil, tmDialer)
		if err != nil {
			return
		}
		defer func() {
			go client.Close()
		}()
		if _, err = client.JoinRealm("realm1", nil); err != nil {
			return
		}
		if err = client.Subscribe("ticker", nil, makeTickerSubHandler(updatesCh)); err != nil {
			return
		}
		val, ok := <-stopCh
		if val || !ok {
			return
		}
		return true, nil
	}
	for {
		if cont, err := f(); !cont {
			return err
		}
	}
}
