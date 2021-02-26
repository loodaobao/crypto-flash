package main

// Ref: https://github.com/go-numb/go-ftx/blob/3144aa68f5/realtime/websocket.go

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"sort"
	"time"

	"github.com/buger/jsonparser"
	"github.com/gorilla/websocket"
)

const (
	wsScheme = "wss"
	wsHost   = "ftx.com"
	wsPath   = "/ws"
)

const (
	UNDEFINED = iota
	ERROR
	ORDERBOOK
)

type request struct {
	Op      string `json:"op"`
	Channel string `json:"channel"`
	Market  string `json:"market"`
}

type Response struct {
	Type      int
	Symbol    string
	Orderbook Orderbook
	Results   error
}

type RowAsksList []Row
type RowBidsList []Row
type Row struct {
	Price float64 `json:"price"`
	Size  float64 `json:"size"`
}
type Orderbook struct {
	Bids   [][]float64 `json:"bids"`
	Asks   [][]float64 `json:"asks"`
	Action string      `json:"action"`
}

var ResultAsks map[string]RowAsksList = make(map[string]RowAsksList)
var ResultBids map[string]RowBidsList = make(map[string]RowBidsList)

func subscribe(conn *websocket.Conn, channel string, symbols []string) error {
	if symbols != nil {
		for j := range symbols {
			if err := conn.WriteJSON(&request{
				Op:      "subscribe",
				Channel: channel,
				Market:  symbols[j],
			}); err != nil {
				return err
			}
		}
	} else {
		if err := conn.WriteJSON(&request{
			Op:      "subscribe",
			Channel: channel,
		}); err != nil {
			return err
		}
	}
	return nil
}

func ping(conn *websocket.Conn) (err error) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, []byte(`{"op": "pong"}`)); err != nil {
				goto EXIT
			}
		}
	}
EXIT:
	return err
}

func Connect(ctx context.Context, ch chan Response, channel string, symbols []string, l *log.Logger) error {
	if l == nil {
		l = log.New(os.Stdout, "ftx websocket", log.Llongfile)
	}

	u := url.URL{Scheme: wsScheme, Host: wsHost, Path: wsPath}
	log.Printf("connecting to %s", u.String())
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}

	if err := subscribe(conn, channel, symbols); err != nil {
		return err
	}

	// ping each 15sec for exchange
	go ping(conn)

	go func() {
		defer conn.Close()

	RESTART:
		for {
			var res Response
			_, msg, err := conn.ReadMessage()
			if err != nil {
				l.Printf("[ERROR]: msg error: %+v", err)
				res.Type = ERROR
				res.Results = fmt.Errorf("%v", err)
				break RESTART
			}

			typeMsg, err := jsonparser.GetString(msg, "type")
			if typeMsg == "error" {
				l.Printf("[ERROR]: error: %+v", string(msg))
				res.Type = ERROR
				res.Results = fmt.Errorf("%v", string(msg))
				break RESTART
			}

			channel, err := jsonparser.GetString(msg, "channel")
			if err != nil {
				l.Printf("[ERROR]: channel error: %+v", string(msg))
				res.Type = ERROR
				res.Results = fmt.Errorf("%v", string(msg))
				break RESTART
			}

			market, err := jsonparser.GetString(msg, "market")
			if err != nil {
				l.Printf("[ERROR]: market err: %+v", string(msg))
				res.Type = ERROR
				res.Results = fmt.Errorf("%v", string(msg))
				break RESTART
			}

			res.Symbol = market

			data, _, _, err := jsonparser.Get(msg, "data")
			if err != nil {
				if isSubscribe, _ := jsonparser.GetString(msg, "type"); isSubscribe == "subscribed" {
					l.Printf("[SUCCESS]: %s %+v", isSubscribe, string(msg))
					continue
				} else {
					err = fmt.Errorf("[ERROR]: data err: %v %s", err, string(msg))
					l.Println(err)
					res.Type = ERROR
					res.Results = err
					break RESTART
				}
			}

			switch channel {
			case "orderbook":
				res.Type = ORDERBOOK
				if err := json.Unmarshal(data, &res.Orderbook); err != nil {
					l.Printf("[WARN]: cant unmarshal orderbook %+v", err)
					continue
				}

				ResultBids[res.Symbol] = *sortBids(ResultBids[res.Symbol], res.Orderbook.Bids)
				ResultAsks[res.Symbol] = *sortAsks(ResultAsks[res.Symbol], res.Orderbook.Asks)

				fmt.Println("result bids --> ", ResultBids)
				fmt.Println("result asks --> ", ResultAsks)

			default:
				res.Type = UNDEFINED
				res.Results = fmt.Errorf("%v", string(msg))
			}
		}
	}()

	return nil
}

func (e RowBidsList) Len() int {
	return len(e)
}

func (e RowAsksList) Len() int {
	return len(e)
}

func (e RowBidsList) Less(i, j int) bool {
	return e[i].Price > e[j].Price
}

func (e RowAsksList) Less(i, j int) bool {
	return e[i].Price < e[j].Price
}

func (e RowBidsList) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func (e RowAsksList) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func sortBids(original []Row, new [][]float64) *[]Row {
	var convertNewObj []Row
	for _, bids := range new {
		bidsRow := Row{bids[0], bids[1]}
		convertNewObj = append(convertNewObj, bidsRow)
	}

	original = append(original, convertNewObj...)
	sort.Sort(RowBidsList(original))

	var result []Row
	result = append(result, original[0])
	for i := 1; i < len(original); i++ {
		if result[len(result)-1].Price == original[i].Price {
			result[len(result)-1].Size = original[i].Size
		} else {
			result = append(result, original[i])
		}
	}

	if len(result) > 50 {
		result = result[:50]
	}

	return &result
}

func sortAsks(original []Row, new [][]float64) *[]Row {
	var convertNewObj []Row
	for _, asks := range new {
		asksRow := Row{asks[0], asks[1]}
		convertNewObj = append(convertNewObj, asksRow)
	}

	original = append(original, convertNewObj...)
	sort.Sort(RowAsksList(original))

	var result []Row
	result = append(result, original[0])
	for i := 1; i < len(original); i++ {
		if result[len(result)-1].Price == original[i].Price {
			result[len(result)-1].Size = original[i].Size
		} else {
			result = append(result, original[i])
		}
	}

	if len(result) > 50 {
		result = result[:50]
	}

	return &result
}

func Start(pairs []string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	channel := "orderbook"
	ch := make(chan Response)
	go Connect(ctx, ch, channel, pairs, nil)

	time.Sleep(1 * time.Second)
	fmt.Println("1st Result: ", ResultBids)
	fmt.Println("1st Result: ", ResultAsks)

	time.Sleep(1 * time.Second)
	fmt.Println("2nd Result: ", ResultBids)
	fmt.Println("2nd Result: ", ResultAsks)

}

var OrderbookResult OrderbookStruct

type OrderbookStruct struct {
	Bids map[string]RowBidsList `json:"bids"`
	Asks map[string]RowAsksList `json:"asks"`
}

func test() {
	pairs := []string{"BTC-PERP", "ETH-PERP"}
	for _, val := range pairs {
		ResultBids[val] = RowBidsList{}
		ResultAsks[val] = RowAsksList{}
	}

	Start(pairs)
	fmt.Println("main bids result --> ", ResultBids)

	fmt.Println("main asks result --> ", ResultAsks)

	// OrderbookResult := &OrderbookStruct{
	// 	Bids: ResultBids,
	// 	Asks: ResultAsks,
	// }

	// return OrderbookResult

}

func main() {
	test()
	// fmt.Println("Final ask result --> ", OrderbookResult.Asks)
	// fmt.Println("Final bids result --> ", OrderbookResult.Bids)
}
