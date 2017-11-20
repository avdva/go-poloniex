package main

import (
	"github.com/avdva/go-poloniex"
)

const (
	API_KEY    = ""
	API_SECRET = ""
)

func main() {
	// Poloniex client
	client := poloniex.New(API_KEY, API_SECRET)

	// Get Ticker (BTC-VTC)
	/*
		ticker, err := client.GetTicker("BTC-DRK")
		fmt.Println(err, ticker)
	*/

	// Get Tickers
	/*
		tickers, err := client.GetTickers()
		if err != nil {
			fmt.Println("Error:", err)
		} else {
			for key, ticker := range tickers {
				fmt.Printf("Ticker: %s, Last: %.8f\n", key, ticker.Last)
			}
		}
		tickerName := "BTC_FLO"
		ticker, ok := tickers[tickerName]
		if ok {
			fmt.Printf("BTC_FLO Last: %.8f\n", ticker.Last)
		} else {
			fmt.Println("ticker not found - ", tickerName)
		}
	*/

	// Get Volumes
	/*
		volumes, err := client.GetVolumes()
		if err != nil {
			fmt.Println("Error:", err)
		} else {
			for key, volume := range volumes.Volumes {
				fmt.Printf("Ticker: %s Value: %#v\n", key, volume["BTC"])
			}
		}
	*/

	// Get CandleStick chart data ( OHLCV )
	/*
		candles, err := client.ChartData("BTC_XMR", 300, time.Now().Add(-time.Hour), time.Now())
		if err != nil {
			panic(err)
		}
		for _, candle := range candles {
			fmt.Printf("BTC_XMR %s\tOpened at: %f\tClosed at: %f\n", candle.Date, candle.Open, candle.Close)
		}
	*/

	// Get markets
	/*
		markets, err := client.GetMarkets()
		fmt.Println(err, markets)
	*/

	// Get orders book
	/*
		orderBook, err := client.GetOrderBook("BTC-DRK", "both", 100)
		fmt.Println(err, orderBook)
	*/

	// Market history
	/*
		marketHistory, err := client.GetMarketHistory("BTC-DRK", 100)
		for _, trade := range marketHistory {
			fmt.Println(err, trade.Timestamp.String(), trade.Quantity, trade.Price)
		}
	*/

	// Market

	// BuyLimit
	/*
		uuid, err := client.BuyLimit("BTC-DOGE", 1000, 0.00000102)
		fmt.Println(err, uuid)
	*/

	// BuyMarket
	/*
		uuid, err := client.BuyLimit("BTC-DOGE", 1000)
		fmt.Println(err, uuid)
	*/

	// Sell limit
	/*
		uuid, err := client.SellLimit("BTC-DOGE", 1000, 0.00000115)
		fmt.Println(err, uuid)
	*/

	// Cancel Order
	/*
		err := client.CancelOrder("e3b4b704-2aca-4b8c-8272-50fada7de474")
		fmt.Println(err)
	*/

	// Get open orders
	/*
		orders, err := client.GetOpenOrders("BTC-DOGE")
		fmt.Println(err, orders)
	*/

	// Account
	// Get balances
	/*
		balances, err := client.GetBalances()
		fmt.Println(err, balances)
	*/

	// Get balance
	/*
		balance, err := client.GetBalance("DOGE")
		fmt.Println(err, balance)
	*/

	// Get address
	/*
		address, err := client.GetDepositAddress("QBC")
		fmt.Println(err, address)
	*/

	// WithDraw
	/*
		whitdrawUuid, err := client.Withdraw("QYQeWgSnxwtTuW744z7Bs1xsgszWaFueQc", "QBC", 1.1)
		fmt.Println(err, whitdrawUuid)
	*/

	// Get order history
	/*
		orderHistory, err := client.GetOrderHistory("BTC-DOGE", 10)
		fmt.Println(err, orderHistory)
	*/

	// Get getwithdrawal history
	/*
		withdrawalHistory, err := client.GetWithdrawalHistory("all", 0)
		fmt.Println(err, withdrawalHistory)
	*/

	// Get deposit history
	/*
		deposits, err := client.GetDepositHistory("all", 0)
		fmt.Println(err, deposits)
	*/

	// Get order book feed.
	/*
		updChan := make(chan poloniex.MarketUpd, 128)
		stopChan := make(chan bool)
		go func() {
			for upd := range updChan {
				fmt.Println(upd)
			}
		}()
		for {
			err := client.SubscribeOrderBook("USDT_BTC", updChan, stopChan)
			if err == nil {
				break
			}
			log.Errorf("client: sub error: %v", err)
			time.Sleep(time.Second)
		}
	*/
}
