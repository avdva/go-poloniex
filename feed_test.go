package poloniex

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscribeOB(t *testing.T) {
	a := assert.New(t)
	client := New("", "")
	stopChan := make(chan struct{})
	updChan := make(chan MarketUpd)
	go func() {
		for range updChan {
			println("ASD")
		}
	}()
	a.NoError(client.SubscribeOrderBook("USDT_BTC", updChan, stopChan))
}

func TestSubscribeTicker(t *testing.T) {
	a := assert.New(t)
	client := New("", "")
	stopChan := make(chan struct{})
	updChan := make(chan TickerUpd)
	go func() {
		for range updChan {

		}
	}()
	_ = a
	t.Error(client.SubscribeTicker(updChan, stopChan))
}
