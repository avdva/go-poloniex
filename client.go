package poloniex

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/avdva/turnpike"
)

type client struct {
	apiKey     string
	apiSecret  string
	httpClient *http.Client
	throttle   <-chan time.Time

	m        sync.RWMutex
	wsClient *turnpike.Client
}

var (
	// Technically 6 req/s allowed, but we're being nice / playing it safe.
	reqInterval = 200 * time.Millisecond
)

// NewClient return a new Poloniex HTTP client
func NewClient(apiKey, apiSecret string) (c *client) {
	return &client{apiKey: apiKey, apiSecret: apiSecret, httpClient: &http.Client{}, throttle: time.Tick(reqInterval)}
}

// doTimeoutRequest do a HTTP request with timeout
func (c *client) doTimeoutRequest(timer *time.Timer, req *http.Request) (*http.Response, error) {
	// Do the request in the background so we can check the timeout
	type result struct {
		resp *http.Response
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := c.httpClient.Do(req)
		done <- result{resp, err}
	}()
	// Wait for the read or the timeout
	select {
	case r := <-done:
		return r.resp, r.err
	case <-timer.C:
		return nil, errors.New("timeout on reading data from Poloniex API")
	}
}

func (c *client) makeReq(method, resource, payload string, authNeeded bool, respCh chan<- []byte, errCh chan<- error) {
	body := []byte{}
	connectTimer := time.NewTimer(DEFAULT_HTTPCLIENT_TIMEOUT)

	var rawurl string
	if strings.HasPrefix(resource, "http") {
		rawurl = resource
	} else {
		rawurl = fmt.Sprintf("%s/%s", API_BASE, resource)
	}

	req, err := http.NewRequest(method, rawurl, strings.NewReader(payload))
	if err != nil {
		respCh <- body
		errCh <- errors.New("You need to set API Key and API Secret to call this method")
		return
	}
	if method == "POST" || method == "PUT" {
		req.Header.Add("Content-Type", "application/json;charset=utf-8")
	}
	req.Header.Add("Accept", "application/json")

	// Auth
	if authNeeded {
		if len(c.apiKey) == 0 || len(c.apiSecret) == 0 {
			respCh <- body
			errCh <- errors.New("You need to set API Key and API Secret to call this method")
			return
		}
		nonce := time.Now().UnixNano()
		q := req.URL.Query()
		q.Set("apikey", c.apiKey)
		q.Set("nonce", fmt.Sprintf("%d", nonce))
		req.URL.RawQuery = q.Encode()
		mac := hmac.New(sha512.New, []byte(c.apiSecret))
		_, err = mac.Write([]byte(req.URL.String()))
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Add("apisign", sig)
	}

	resp, err := c.doTimeoutRequest(connectTimer, req)
	if err != nil {
		respCh <- body
		errCh <- err
		return
	}

	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		respCh <- body
		errCh <- err
		return
	}
	if resp.StatusCode != 200 {
		respCh <- body
		errCh <- errors.New(resp.Status)
		return
	}

	respCh <- body
	errCh <- nil
	close(respCh)
	close(errCh)
}

// do prepare and process HTTP request to Poloniex API
func (c *client) do(method, resource, payload string, authNeeded bool) (response []byte, err error) {
	respCh := make(chan []byte)
	errCh := make(chan error)
	<-c.throttle
	go c.makeReq(method, resource, payload, authNeeded, respCh, errCh)
	response = <-respCh
	err = <-errCh
	return
}

func tmDialer(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, DEFAULT_HTTPCLIENT_TIMEOUT)
}

func (c *client) checkWsClient() (*turnpike.Client, error) {
	c.m.RLock()
	cl := c.wsClient
	c.m.RUnlock()
	if cl != nil {
		return cl, nil
	}
	var err error
	cl, err = turnpike.NewWebsocketClient(turnpike.JSONNUMBER, API_WS, nil, tmDialer)
	if err != nil {
		return nil, err
	}
	doneCh := make(chan bool)
	cl.ReceiveDone = doneCh
	if _, err = cl.JoinRealm("realm1", nil); err != nil {
		cl.Close()
		return nil, err
	}
	c.m.Lock()
	if c.wsClient == nil {
		c.wsClient = cl
		c.m.Unlock()
		go func() {
			<-doneCh
			c.m.Lock()
			if c.wsClient == cl {
				c.wsClient = nil
			}
			c.m.Unlock()
		}()
	} else {
		cl = c.wsClient
		c.m.Unlock()
		cl.Close()
	}
	return cl, nil
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
