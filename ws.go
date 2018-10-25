package poloniex

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

type wsClient struct {
	m      sync.Mutex
	cv     *sync.Cond
	conn   *websocket.Conn
	wsErr  error
	wsChan chan bool
	subs   map[string]struct{}
}

type wsMessage struct {
	channelID json.Number
	seq       *int64
	data      [][]interface{}
}

func newWsClient() *wsClient {
	result := &wsClient{
		wsChan: make(chan bool, 1),
	}
	result.cv = sync.NewCond(&result.m)
	go result.wsConnLoop()
	return result
}

func (c *wsClient) close() error {
	c.m.Lock()
	if c.wsChan != nil {
		close(c.wsChan)
		c.wsChan = nil
	}
	c.m.Unlock()
	return c.wsReset()
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

func (c *wsClient) readLoop(conn *websocket.Conn) {
	defer c.wsReset()
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
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
	c.parseMessage(msg)
	return nil
}

func (c *wsClient) parseMessage(msg wsMessage) {
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
}

func (c *wsClient) handleObookOrTrades(typ string, i []interface{}, mu *MarketUpd) error {
	switch typ {
	case "i":
		updates, err := c.parseObookInitial(i[1])
		if err == nil {
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
				Type:  Sell,
				Price: price,
				Size:  size,
			})
		}
	}
	fill(oBookUpd.OrderBook[0], Sell)
	fill(oBookUpd.OrderBook[1], Buy)
	return updates, nil
}

func (c *wsClient) wsReset() error {
	c.m.Lock()
	conn := c.conn
	c.conn = nil
	c.m.Unlock()
	if conn != nil {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		return conn.Close()
	}
	return nil
}

func (c *wsClient) cmd(command, channel string) error {
	client, err := c.checkWsClient()
	if err != nil {
		return err
	}
	m := map[string]string{
		"command": command,
		"channel": channel,
	}
	return client.WriteJSON(m)
}

func (c *wsClient) subscribe(channel string) error {
	c.cmd("subscribe", "BTC_BTS")
	c.cmd("subscribe", "14")
	return nil
}
