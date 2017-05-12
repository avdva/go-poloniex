package poloniex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSubscribeOB(t *testing.T) {
	a := assert.New(t)
	client := New("", "")
	stopChan := make(chan bool)
	updChan := make(chan MarketUpd)
	go func() {
		a.NoError(client.SubscribeOrderBook("USDT_BTC", updChan, stopChan))
	}()
	tm := time.After(time.Second * 30)
	msgCount := 0
	for loop := true; loop; {
		select {
		case <-updChan:
			if msgCount == 0 {
				stopChan <- false
			}
			msgCount++
			if msgCount >= 3 {
				loop = false
			}
		case <-tm:
			loop = false
		}
	}
	if msgCount < 3 {
		t.Errorf("got less, than 3 messages: %d", msgCount)
	}
	close(stopChan)
}

func TestSubscribeTicker(t *testing.T) {
	a := assert.New(t)
	client := New("", "")
	stopChan := make(chan bool)
	updChan := make(chan TickerUpd)
	go func() {
		a.NoError(client.SubscribeTicker(updChan, stopChan))
	}()
	tm := time.After(time.Second * 30)
	msgCount := 0
	for loop := true; loop; {
		select {
		case <-updChan:
			if msgCount == 0 {
				stopChan <- false
			}
			msgCount++
			if msgCount >= 3 {
				loop = false
			}
		case <-tm:
			loop = false
		}
	}
	if msgCount < 3 {
		t.Errorf("got less, than 3 messages: %d", msgCount)
	}
	close(stopChan)
}
