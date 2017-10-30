package poloniex

import (
	"errors"
	"net"

	"github.com/avdva/turnpike"
)

func (c *client) close() error {
	c.m.Lock()
	if c.wsChan != nil {
		close(c.wsChan)
		c.wsChan = nil
	}
	c.m.Unlock()
	return c.wsReset()
}

func (c *client) makeWsClient() (*turnpike.Client, error) {
	cl, err := turnpike.NewWebsocketClient(turnpike.JSONNUMBER, API_WS, nil,
		func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, c.httpTimeout)
		},
	)
	if err != nil {
		return nil, err
	}
	if _, err = cl.JoinRealm("realm1", nil); err != nil {
		cl.Close()
		return nil, err
	}
	cl.ReceiveDone = make(chan bool)
	go func() {
		<-cl.ReceiveDone
		c.m.Lock()
		if c.wsClient == cl {
			c.wsClient = nil
		}
		c.m.Unlock()
	}()
	return cl, nil
}

func (c *client) wsConnLoop() {
	for range c.wsChan {
		c.m.Lock()
		cl := c.wsClient
		c.m.Unlock()
		if cl != nil {
			continue
		}
		if cl, err := c.makeWsClient(); err == nil {
			c.m.Lock()
			c.wsClient = cl
			c.m.Unlock()
		}
		// notify all clients about connect attempt, successful or not.
		c.cv.Broadcast()
	}
}

func (c *client) checkWsClient() (*turnpike.Client, error) {
	c.m.Lock()
	defer c.m.Unlock()
	// do not wait for connection forever, return an error on bad attempt.
	if c.wsClient == nil {
		if c.wsChan == nil {
			return nil, errors.New("client closed")
		}
		select {
		case c.wsChan <- true:
		default:
		}
		c.cv.Wait()
		if c.wsClient == nil {
			return nil, errors.New("connection error")
		}
	}
	return c.wsClient, nil
}

func (c *client) wsReset() error {
	c.m.Lock()
	cl := c.wsClient
	c.wsClient = nil
	c.m.Unlock()
	if cl != nil {
		return cl.Close()
	}
	return nil
}

func (c *client) wsConnect(topic string, handler turnpike.EventHandler, stopCh <-chan bool) (cont bool, err error) {
	client, err := c.checkWsClient()
	if err != nil {
		return
	}
	ch := make(chan error, 1)
	defer func() {
		if err != nil {
			go client.Unsubscribe(topic)
		}
	}()
	go func() {
		ch <- client.Subscribe(topic, nil, handler)
	}()
	for {
		select {
		case err = <-ch:
			if err != nil {
				return
			}
			ch = nil
		case <-client.ReceiveDone:
			err = errors.New("client closed")
			return
		case val, ok := <-stopCh:
			cont = ok && !val
			return
		}
	}
}
