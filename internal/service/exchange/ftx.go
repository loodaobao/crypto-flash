/*
// Ftx implements exchange API for FTX exchange.
// Input: real exchange, trader
// Output: real exchange, signal provider or indicator
// TODO:
// 1. getHistoryCandles: if candles >= 5000, request many times and concat result
// 3. getPosition
// 4. make conditional order
// 5. exchange interface
*/
package exchange

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	util "crypto-flash/internal/service/util"
)

const (
	host                     string = "https://ftx.com"
	marketAPI                string = "/api/markets"
	walletAPI                string = "/api/wallet/balances"
	orderAPI                 string = "/api/orders"
	condOrderAPI             string = "/api/conditional_orders"
	positionAPI              string = "/api/positions"
	futureAPI                string = "/api/futures"
	fundingRateAPI           string = "/api/funding_rates"
	spotMarginBorrowRatesAPI string = "/api/spot_margin/borrow_rates"
)

type FTX struct {
	tag               string
	key               string
	subAccount        string
	secret            string
	Fee               float64
	CollaterableSpots map[string]float64
	// save all candles data from different resolutions and markets
	candleData map[string][]*util.Candle
	candleSubs map[string][]chan<- *util.Candle
	restClient *util.RestClient
}

func NewFTX(key, secret, subAccount string) *FTX {
	return &FTX{
		key:        key,
		secret:     secret,
		subAccount: subAccount,
		Fee:        0.0007,
		// https://help.ftx.com/hc/en-us/articles/360031149632-Non-USD-Collateral
		CollaterableSpots: map[string]float64{
			"USD":             0.98,
			"NonUSDFiat":      0.98,
			"TokenizedStocks": 0.8,
			"USDT":            0.95,
			"BTC":             0.95,
			"ETH":             0.9,
			"BNB":             0.9,
			"PAXG":            0.9,
			"XAUT":            0.9,
			"KNC":             0.9,
			"BCH":             0.85,
			"LTC":             0.85,
			"TRYB":            0.85,
			"LINK":            0.85,
			"TRX":             0.85,
			"CUSDT":           0.85,
			"XRP":             0.85,
			"SOL":             0.85,
			"BVOL":            0.8,
			"IBVOL":           0.8,
			"MKR":             0.8,
			"SUSHI":           0.8,
			"SNX":             0.8,
			"YFI":             0.8,
			"SXP":             0.8,
			"BTMX":            0.8,
			"OMG":             0.8,
			"TOMO":            0.8,
			"AAVE":            0.8,
			"OKB":             0.8,
			"HT":              0.8,
			"MATIC":           0.8,
			"LEO":             0.8,
			"SRM":             0.85,
			"UNI":             0.8,
			"MOB":             0.8,
			"1INCH":           0.85,
			"CEL":             0.8,
			"GRT":             0.85,
			"RUNE":            0.8,
			"RSR":             0.8,
			"FIDA":            0.8,
			"HXRO":            0.75,
			"HOLY":            0.85,
			"SECO":            0.85,
			"DAI":             0.85,
			"DOGE":            0.8,
			"FTT":             0.95,
		},
		tag:        "FTX",
		candleData: make(map[string][]*util.Candle),
		candleSubs: make(map[string][]chan<- *util.Candle),
		restClient: util.NewRestClient(),
	}
}

// depth 1 ~ 100
func (ftx *FTX) GetOrderbook(market string, depth int) *util.Orderbook {
	type orderbookRes struct {
		Asks [][2]float64
		Bids [][2]float64
	}
	type res struct {
		Success bool
		Result  orderbookRes
	}
	url := host + marketAPI +
		fmt.Sprintf("/%s/orderbook?depth=%d", market, depth)
	var resObj res
	ftx.restClient.Get(url, nil, nil, &resObj)
	orderbook := &util.Orderbook{}
	for _, row := range resObj.Result.Asks {
		orderbook.Add("ask", row[0], row[1])
	}
	for _, row := range resObj.Result.Bids {
		orderbook.Add("bid", row[0], row[1])
	}
	return orderbook
}
func (ftx *FTX) GetHistoryCandles(market string, resolution int,
	startTime int64, endTime int64) []*util.Candle {
	type candleRes struct {
		Close     float64
		High      float64
		Low       float64
		Open      float64
		StartTime string
		Volume    float64
	}
	type historyRes struct {
		Success bool
		Result  []candleRes
	}
	var candles []*util.Candle
	maxReqInterval := int64(resolution * 5000)
	for curStartTime := startTime; curStartTime < endTime; curStartTime += maxReqInterval {
		curEndTime := curStartTime + maxReqInterval
		if curEndTime > endTime {
			curEndTime = endTime
		}
		url := host + marketAPI + fmt.Sprintf(
			"/%s/candles?resolution=%d&start_time=%d&end_time=%d&limit=5000",
			market, resolution, curStartTime, curEndTime)
		var resObj historyRes
		ftx.restClient.Get(url, nil, nil, &resObj)
		for _, c := range resObj.Result {
			candles = append(candles, util.NewCandle(
				c.Open, c.High, c.Low, c.Close, c.Volume, c.StartTime))
		}
	}
	return candles
}
func sleepToNextCandle(resolution int64) {
	timeToNextCandle := resolution - time.Now().Unix()%resolution
	sleepDuration := util.Duration{Second: timeToNextCandle + 1}
	time.Sleep(sleepDuration.GetTimeDuration())
}

// resolution can be 15, 60, 300, 900, 3600, 14400, 86400
func (ftx *FTX) SubCandle(
	market string, resolution int, c chan<- *util.Candle) {
	dataID := fmt.Sprintf("%s-%d", market, resolution)
	if _, exist := ftx.candleData[dataID]; exist {
		// someone already sub this data
		ftx.candleSubs[dataID] = append(ftx.candleSubs[dataID], c)
		return
	}
	ftx.candleData[dataID] = []*util.Candle{}
	ftx.candleSubs[dataID] = []chan<- *util.Candle{}
	ftx.candleSubs[dataID] = append(ftx.candleSubs[dataID], c)
	resolution64 := int64(resolution)
	// sleep to the next candle
	sleepToNextCandle(resolution64)
	for {
		now := time.Now().Unix()
		startTime := now - resolution64*2 + 1
		endTime := now - resolution64
		candles := ftx.GetHistoryCandles(
			"BTC-PERP", resolution, startTime, endTime)
		for _, c := range ftx.candleSubs[dataID] {
			c <- candles[0]
		}
		ftx.candleData[dataID] = append(ftx.candleData[dataID], candles...)
		sleepToNextCandle(resolution64)
	}
}
func (ftx *FTX) genAuthHeader(method, path, body string) *http.Header {
	header := http.Header(make(map[string][]string))
	header.Add("FTX-KEY", ftx.key)
	ts := fmt.Sprintf("%d", time.Now().UnixNano()/1000000)
	header.Add("FTX-TS", ts)
	payload := ts + method + path + body
	signature := util.HMac(payload, ftx.secret)
	header.Add("FTX-SIGN", signature)
	if ftx.subAccount != "" {
		header.Add("FTX-SUBACCOUNT", ftx.subAccount)
	}
	return &header
}
func (ftx *FTX) GetWallet() *util.Wallet {
	type coin struct {
		Coin  string
		Free  float64
		Total float64
	}
	type res struct {
		Success bool
		Result  []coin
	}
	url := host + walletAPI
	header := ftx.genAuthHeader("GET", walletAPI, "")
	var resObj res
	ftx.restClient.Get(url, header, nil, &resObj)
	wallet := util.NewWallet()
	for _, coin := range resObj.Result {
		wallet.Increase(coin.Coin, coin.Total)
	}
	return wallet
}
func (ftx *FTX) GetPosition(market string) *util.Position {
	type resPos struct {
		Cost                         float64
		EntryPrice                   float64
		EstimatedLiquidationPrice    float64
		Future                       string
		InitialMarginRequirement     float64
		LongOrderSize                float64
		MaintenanceMarginRequirement float64
		NetSize                      float64
		OpenSize                     float64
		RealizedPnl                  float64
		ShortOrderSize               float64
		Side                         string
		Size                         float64
		UnrealizedPnl                float64
	}
	type res struct {
		Success bool
		Result  []resPos
	}
	url := host + positionAPI
	header := ftx.genAuthHeader("GET", positionAPI, "")
	var resObj res
	ftx.restClient.Get(url, header, nil, &resObj)
	if !resObj.Success {
		fmt.Println(resObj)
		util.Error(ftx.tag, "Cancel all order error")
	}
	for _, pos := range resObj.Result {
		if pos.Future == market && pos.Size != 0 {
			var side string
			if pos.Side == "sell" {
				side = "short"
			} else {
				side = "long"
			}
			return &util.Position{
				Market:    pos.Future,
				Side:      side,
				Size:      pos.Size,
				OpenPrice: pos.EntryPrice,
			}
		}
	}
	return nil
}
func (ftx *FTX) MakeOrder(order *util.Order) int64 {
	type result struct {
		CreatedAt  string
		FilledSize float64
		Future     string
		Id         int64
		Market     string
		Price      float64
		RemainSize float64
		Side       string
		Size       float64
		Status     string
		Type       string
		ReduceOnly bool
		Ioc        bool
		PostOnly   bool
		ClientId   string
		// for conditional order
		TriggerPrice     float64
		OrderPrice       float64
		TriggeredAt      string
		OrderType        string
		RetryUntilFilled bool
	}
	type res struct {
		Success bool
		Result  result
	}
	var api string
	if order.Type == "market" || order.Type == "limit" {
		api = orderAPI
	} else if order.Type == "stop" || order.Type == "trailingStop" ||
		order.Type == "takeProfit" {
		api = condOrderAPI
	}
	url := host + api
	orderStr := util.GetJSONString(order.CreateMap())
	header := ftx.genAuthHeader("POST", api, orderStr)
	var resObj res
	ftx.restClient.Post(url, header,
		util.GetJSONBuffer(order.CreateMap()), &resObj)
	if !resObj.Success {
		fmt.Println(resObj)
		util.Error(ftx.tag, "Make order error")
	}
	return resObj.Result.Id
}
func (ftx *FTX) CancelAllOrder(market string) {
	type req struct {
		Market string
	}
	reqBody := req{
		Market: market,
	}
	type res struct {
		Success bool
		Result  string
	}
	url := host + orderAPI
	header := ftx.genAuthHeader("DELETE", orderAPI, util.GetJSONString(reqBody))
	var resObj res
	ftx.restClient.Delete(url, header, util.GetJSONBuffer(reqBody), &resObj)
	if !resObj.Success {
		fmt.Println(resObj)
		util.Error(ftx.tag, "Cancel all order error")
	}
}
func (ftx *FTX) GetFundingRates(startTime, endTime int64,
	future string) []float64 {
	type result struct {
		Future string
		Rate   float64
		Time   string
	}
	type res struct {
		Success bool
		Result  []result
	}
	url := host + fundingRateAPI
	url += fmt.Sprintf("?start_time=%d&end_time=%d&future=%s",
		startTime, endTime, future)
	req := make(map[string]interface{})
	req["start_time"] = startTime
	req["end_time"] = endTime
	req["future"] = future
	var resObj res
	ftx.restClient.Get(url, nil, nil, &resObj)
	if !resObj.Success {
		fmt.Println(resObj)
		util.Error(ftx.tag, "Get funding rates error")
	}
	var rates []float64
	for _, result := range resObj.Result {
		rates = append(rates, result.Rate)
	}
	return rates
}

type futureResult struct {
	Ask   float64
	Bid   float64
	Index float64
}

func (ftx *FTX) GetFuture(future string) (futureResult, error) {
	type res struct {
		Success bool
		Result  futureResult
	}
	url := host + futureAPI + "/" + future
	var resObj res
	ftx.restClient.Get(url, nil, nil, &resObj)
	if !resObj.Success {
		fmt.Println(resObj)
		errorMsg := fmt.Sprintf("Get future %s error", future)
		util.Error(ftx.tag, errorMsg)
		return resObj.Result, errors.New(errorMsg)
	}
	return resObj.Result, nil
}

type futureStatsResult struct {
	NextFundingRate float64
	NextFundingTime string
}

func (ftx *FTX) GetFutureStats(future string) futureStatsResult {
	type res struct {
		Success bool
		Result  futureStatsResult
	}
	url := host + futureAPI + "/" + future + "/stats"
	var resObj res
	ftx.restClient.Get(url, nil, nil, &resObj)
	if !resObj.Success {
		fmt.Println(resObj)
		util.Error(ftx.tag, "Get future stats error")
	}
	return resObj.Result
}

type MarketPairs struct {
	Perps    []string
	Quarters []string
	Spots    []string
}

// Get all markets
func (ftx *FTX) GetMarketPairs() (*MarketPairs, error) {
	type markets struct {
		Name string
		Type string
	}

	type res struct {
		Success bool
		Result  []markets
	}

	var resObj res
	url := host + marketAPI
	ftx.restClient.Get(url, nil, nil, &resObj)

	if !resObj.Success {
		fmt.Println(resObj)
		errorMsg := fmt.Sprintf("Get all markets error")
		util.Error(ftx.tag, errorMsg)
		return nil, errors.New(errorMsg)
	}

	var marketPairs MarketPairs
	for _, market := range resObj.Result {
		switch marketType := market.Type; marketType {
		case "future":
			existIndex := strings.Index(market.Name, "-PERP")
			if existIndex >= 0 {
				marketPairs.Perps = append(marketPairs.Perps, market.Name)
			} else {
				marketPairs.Quarters = append(marketPairs.Quarters, market.Name)
			}
		case "spot":
			marketPairs.Spots = append(marketPairs.Spots, market.Name)
		}
	}

	return &marketPairs, nil
}

type SpotMarginBorrowRate struct {
	Coin     string  `json:"coin"`
	Estimate float64 `json:"estimate"`
	Previous float64 `json:"previous"`
}

// Get spot margin borrow rates
func (ftx *FTX) GetspotMarginBorrowRates() (*[]SpotMarginBorrowRate, error) {
	type res struct {
		Success bool
		Result  []SpotMarginBorrowRate
	}

	var resObj res
	url := host + spotMarginBorrowRatesAPI
	header := ftx.genAuthHeader("GET", spotMarginBorrowRatesAPI, "")
	ftx.restClient.Get(url, header, nil, &resObj)

	if !resObj.Success {
		fmt.Println(resObj)
		errorMsg := fmt.Sprintf("Get spot margin borrow rates error")
		util.Error(ftx.tag, errorMsg)
		return nil, errors.New(errorMsg)
	}

	return &resObj.Result, nil
}
