package poloniex

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTickers(t *testing.T) {
	a := assert.New(t)
	client := New("", "")
	tickers, err := client.GetTickers()
	if !a.NoError(err) {
		return
	}
	ticker, found := tickers["USDT_BTC"]
	if !a.True(found) {
		return
	}
	last, _ := ticker.Last.Float64()
	a.True(last > 0)
}
