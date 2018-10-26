package poloniex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSubscribeOB(t *testing.T) {
	client := New("", "")
	stopChan := make(chan struct{})
	updChan := make(chan MarketUpd)
	ticks, err := client.GetTickers()
	require.NoError(t, err)
	cur, found := ticks["USDT_BTC"]
	require.NotZero(t, found)
	errCh := make(chan error)
	defer func() {
		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(3 * time.Second):
			t.Error("unsubscribe timeout")
		}
	}()
	go func() {
		err := client.SubscribeOrderBook(cur.ID, updChan, stopChan)
		require.NoError(t, err)
		errCh <- err
	}()
	tm := time.After(time.Second * 30)
	msgCount := 0
	for loop := true; loop; {
		select {
		case <-updChan:
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

func TestSubscribeOB2(t *testing.T) {
	client := New("", "")
	stopChan := make(chan struct{})
	updChan := make(chan MarketUpd)
	ticks, err := client.GetTickers()
	require.NoError(t, err)
	cur, found := ticks["USDT_BTC"]
	require.NotZero(t, found)
	errCh := make(chan error)
	defer func() {
		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(3 * time.Second):
			t.Error("unsubscribe timeout")
		}
	}()
	go func() {
		err := client.SubscribeOrderBook(cur.ID, updChan, stopChan)
		require.NoError(t, err)
		errCh <- err
	}()
	tm := time.After(time.Second * 30)
	msgCount := 0
	for loop := true; loop; {
		select {
		case <-updChan:
			msgCount++
			loop = false
		case <-tm:
			loop = false
		}
	}
	if msgCount < 1 {
		t.Errorf("got less, than 1 messages: %d", msgCount)
	}
	require.NoError(t, client.UnsubscribeAll())
}
