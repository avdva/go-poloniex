package poloniex

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/avdva/sorgo/async"
	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

type wsClient struct {
	writeChan chan wsCmd
	m         sync.Mutex
	cv        *sync.Cond
	conn      *websocket.Conn
	wsErr     error
	wsChan    chan bool
	subs      map[int]wsSub
}

type wsMessage struct {
	channelID json.Number
	seq       *int64
	data      [][]interface{}
}

type wsSub struct {
	handler func(interface{})
	errChan chan error
}

type wsCmd struct {
	message  interface{}
	resultCh chan error
}

func newWsClient() *wsClient {
	result := &wsClient{
		writeChan: make(chan wsCmd, 16),
		wsChan:    make(chan bool, 1),
		subs:      make(map[int]wsSub),
	}
	result.cv = sync.NewCond(&result.m)
	go result.writeLoop()
	go result.wsConnLoop()
	return result
}

func (c *wsClient) shutdown() error {
	c.m.Lock()
	if c.wsChan != nil {
		close(c.wsChan)
		c.wsChan = nil
	}
	c.m.Unlock()
	return c.closeConnWithErr(nil)
}

func (c *wsClient) closeConnWithErr(err error) error {
	c.m.Lock()
	conn := c.conn
	c.conn = nil
	for _, sub := range c.subs {
		asyncErr(sub.errChan, err)
	}
	c.m.Unlock()
	if conn != nil {
		async.DoWithTimeout(func() error {
			return conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		}, func(e error) {
			log.Warnf("timeout writing ws close message")
		}, 2*time.Second)
		return conn.Close()
	}
	return nil
}

func (c *wsClient) cmd(command string, channel int) error {
	cmd := wsCmd{
		message: map[string]interface{}{
			"command": command,
			"channel": channel,
		},
		resultCh: make(chan error, 1),
	}
	c.writeChan <- cmd
	return <-cmd.resultCh
}

func (c *wsClient) makeWsClient() (*websocket.Conn, error) {
	cl, _, err := websocket.DefaultDialer.Dial(API_WS, nil)
	return cl, err
}

func (c *wsClient) wsConnLoop() {
	for range c.wsChan {
		c.m.Lock()
		cl := c.conn
		c.m.Unlock()
		if cl != nil {
			continue
		}
		cl, err := c.makeWsClient()
		c.m.Lock()
		c.conn = cl
		c.wsErr = err
		c.m.Unlock()
		// notify all clients about connect attempt, successful or not.
		c.cv.Broadcast()
	}
}

func (c *wsClient) checkWsClient() (*websocket.Conn, error) {
	c.m.Lock()
	defer c.m.Unlock()
	// do not wait for connection forever, return an error on bad attempt.
	if c.conn == nil {
		if c.wsChan == nil {
			return nil, errors.New("client closed")
		}
		select {
		case c.wsChan <- true:
		default:
		}
		c.cv.Wait()
		if c.conn == nil {
			err := c.wsErr
			if err == nil {
				err = errors.New("connection error")
			}
			return nil, err
		} else {
			go c.readLoop(c.conn)
		}
	}
	return c.conn, nil
}

func (c *wsClient) writeLoop() {
	for cmd := range c.writeChan {
		conn, err := c.checkWsClient()
		if err != nil {
			asyncErr(cmd.resultCh, err)
			continue
		}
		asyncErr(cmd.resultCh, conn.WriteJSON(cmd.message))
	}
}

func (c *wsClient) wsTimeout(hbChan chan struct{}) {
	const hbTimeout = 5 * time.Second
	for {
		select {
		case _, ok := <-hbChan:
			if !ok {
				return
			}
		case <-time.After(hbTimeout):
			c.closeConnWithErr(errors.New("ws timeout"))
			return
		}
	}
}

func (c *wsClient) readLoop(conn *websocket.Conn) {
	hbChan := make(chan struct{})
	defer close(hbChan)
	go c.wsTimeout(hbChan)
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			c.closeConnWithErr(err)
			return
		}
		hbChan <- struct{}{}
		if err := c.handleMessage(message); err != nil {
			log.Errorf("message handling error: %v", err)
		}
	}
}

func (c *wsClient) handleMessage(message []byte) error {
	var msg wsMessage
	arr := []interface{}{&msg.channelID, &msg.seq, &msg.data}
	if err := json.Unmarshal(message, &arr); err != nil {
		return err
	}
	return c.parseMessage(msg)
}

func (c *wsClient) parseMessage(msg wsMessage) error {
	id, err := msg.channelID.Int64()
	if err != nil {
		return err
	}
	var mu MarketUpd
	for _, el := range msg.data {
		if len(el) < 2 {
			continue
		}
		typ, ok := el[0].(string)
		if !ok {
			continue
		}
		switch typ {
		case "i", "o", "t": // order book updates, or new trades
			if msg.seq != nil {
				if err := c.handleObookOrTrades(typ, el, &mu); err != nil {
					log.Errorf("message parsing error: %v", err)
				}
			}
		}
	}
	if len(mu.Obooks)+len(mu.Trades) == 0 {
		return nil
	}
	c.m.Lock()
	handler := c.subs[int(id)].handler
	c.m.Unlock()
	if handler != nil {
		handler(mu)
	}
	return nil
}

func (c *wsClient) handleObookOrTrades(typ string, i []interface{}, mu *MarketUpd) error {
	switch typ {
	case "i":
		updates, err := c.parseObookInitial(i[1])
		if err == nil {
			mu.Initial = true
			mu.Obooks = updates
		}
		return err
	case "o":
		update, err := c.parseObookUpdate(i)
		if err == nil {
			mu.Obooks = append(mu.Obooks, update)
		}
		return err
	case "t":
		update, err := c.parseTradeUpdate(i)
		if err == nil {
			mu.Trades = append(mu.Trades, update)
		}
		return err
	}
	return nil
}

func (c *wsClient) parseObookUpdate(arr []interface{}) (OrderBookUpd, error) {
	var oBookUpd OrderBookUpd
	var typ string
	toDecode := []interface{}{&typ, &oBookUpd.Type, &oBookUpd.Price, &oBookUpd.Size}
	obookDec, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:     &toDecode,
		DecodeHook: obookDecodeHook,
	})
	return oBookUpd, obookDec.Decode(arr)
}

func (c *wsClient) parseTradeUpdate(arr []interface{}) (TradeUpd, error) {
	var tradeUpd TradeUpd
	var typ string
	toDecode := []interface{}{&typ, &tradeUpd.TradeID, &tradeUpd.Type, &tradeUpd.Size, &tradeUpd.Price, &tradeUpd.Date}
	obookDec, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:     &toDecode,
		DecodeHook: obookDecodeHook,
	})
	return tradeUpd, obookDec.Decode(arr)
}

func (c *wsClient) parseObookInitial(i interface{}) ([]OrderBookUpd, error) {
	oBookUpd := struct {
		CurrencyPair string
		OrderBook    []map[decimal.Decimal]decimal.Decimal
	}{}
	obookDec, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:     &oBookUpd,
		DecodeHook: obookDecodeHook,
	})
	if err := obookDec.Decode(i); err != nil {
		return nil, err
	}
	if len(oBookUpd.OrderBook) != 2 {
		return nil, errors.New("invalid order book structure")
	}
	var updates []OrderBookUpd
	fill := func(m map[decimal.Decimal]decimal.Decimal, typ OpType) {
		for price, size := range m {
			updates = append(updates, OrderBookUpd{
				Type:  typ,
				Price: price,
				Size:  size,
			})
		}
	}
	fill(oBookUpd.OrderBook[0], Sell)
	fill(oBookUpd.OrderBook[1], Buy)
	return updates, nil
}

func (c *wsClient) makeMarektUpdateHandler(updatesCh chan<- MarketUpd) func(interface{}) {
	return func(i interface{}) {
		mu, ok := i.(MarketUpd)
		if !ok {
			return
		}
		select {
		case updatesCh <- mu:
		default:
		}
	}
}

func (c *wsClient) unsubscribeChan(id int) {
	err := async.DoWithTimeout(func() error {
		return c.cmd("unsubscribe", id)
	}, func(err error) {
		c.m.Lock()
		sub, found := c.subs[id]
		if found {
			asyncErr(sub.errChan, errors.New("subscription error"))
		}
		c.m.Unlock()
		log.Warnf("timeout writing ws unsubscribe message. the last error was %v", err)
	}, 2*time.Second)
	if err != nil {
		log.Errorf("unsubscribe error: %v", err)
	}
	c.m.Lock()
	delete(c.subs, id)
	c.m.Unlock()
}

func (c *wsClient) subscribeMarketUpdates(id int, updatesCh chan<- MarketUpd, stopCh <-chan struct{}) error {
	c.m.Lock()
	if _, found := c.subs[id]; found {
		c.m.Unlock()
		return errors.New("already subscribed")
	}
	errChan := make(chan error, 1)
	c.subs[id] = wsSub{
		handler: c.makeMarektUpdateHandler(updatesCh),
		errChan: errChan,
	}
	c.m.Unlock()
	defer c.unsubscribeChan(id)
	if err := c.cmd("subscribe", id); err != nil {
		return err
	}
	select {
	case <-stopCh:
		return nil
	case err := <-errChan:
		return err
	}
}

func asyncErr(ch chan<- error, err error) {
	select {
	case ch <- err:
	default:
	}
}
