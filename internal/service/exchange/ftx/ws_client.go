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
	TICKER
	TRADES
	ORDERBOOK
	ORDERS
	FILLS
)

type request struct {
	Op      string `json:"op"`
	Channel string `json:"channel"`
	Market  string `json:"market"`
}

type Ticker struct {
	Bid     float64   `json:"bid"`
	Ask     float64   `json:"ask"`
	BidSize float64   `json:"bidSize"`
	AskSize float64   `json:"askSize"`
	Last    float64   `json:"last"`
	Time    time.Time `json:"time"`
}

type Trade struct {
	ID          int       `json:"id"`
	Liquidation bool      `json:"liquidation"`
	Price       float64   `json:"price"`
	Side        string    `json:"side"`
	Size        float64   `json:"size"`
	Time        time.Time `json:"time"`
}

type Order struct {
	ID            int       `json:"id"`
	ClientID      string    `json:"clientId"`
	Market        string    `json:"market"`
	Type          string    `json:"type"`
	Side          string    `json:"side"`
	Size          float64   `json:"size"`
	Price         float64   `json:"price"`
	ReduceOnly    bool      `json:"reduceOnly"`
	Ioc           bool      `json:"ioc"`
	PostOnly      bool      `json:"postOnly"`
	Status        string    `json:"status"`
	FilledSize    float64   `json:"filledSize"`
	RemainingSize float64   `json:"remainingSize"`
	AvgFillPrice  float64   `json:"avgFillPrice"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Response struct {
	Type   int
	Symbol string

	Ticker    Ticker
	Trades    Trade
	Orderbook Orderbook
	Orders    Order

	Results error
}

type RowList []Row
type Row struct {
	Price float64 `json:"price"`
	Size  float64 `json:"size"`
}
type Orderbook struct {
	Bids   [][]float64 `json:"bids"`
	Asks   [][]float64 `json:"asks"`
	Action string      `json:"action"`
	// Time     decimal.Decimal `json:"time"`
	// Checksum int             `json:"checksum"`
}

func subscribe(conn *websocket.Conn, channels, symbols []string) error {
	if symbols != nil {
		for i := range channels {
			for j := range symbols {
				if err := conn.WriteJSON(&request{
					Op:      "subscribe",
					Channel: channels[i],
					Market:  symbols[j],
				}); err != nil {
					return err
				}
			}
		}
	} else {
		for i := range channels {
			if err := conn.WriteJSON(&request{
				Op:      "subscribe",
				Channel: channels[i],
			}); err != nil {
				return err
			}
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

func Connect(ctx context.Context, ch chan Response, channels, symbols []string, l *log.Logger) error {
	if l == nil {
		l = log.New(os.Stdout, "ftx websocket", log.Llongfile)
	}

	u := url.URL{Scheme: wsScheme, Host: wsHost, Path: wsPath}
	log.Printf("connecting to %s", u.String())
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}

	if err := subscribe(conn, channels, symbols); err != nil {
		return err
	}

	// ping each 15sec for exchange
	go ping(conn)

	go func() {
		defer conn.Close()
		// defer unsubscribe(conn, channels, symbols)

	RESTART:
		for {
			var res Response
			_, msg, err := conn.ReadMessage()
			if err != nil {
				l.Printf("[ERROR]: msg error: %+v", err)
				res.Type = ERROR
				res.Results = fmt.Errorf("%v", err)
				ch <- res
				break RESTART
			}

			typeMsg, err := jsonparser.GetString(msg, "type")
			if typeMsg == "error" {
				l.Printf("[ERROR]: error: %+v", string(msg))
				res.Type = ERROR
				res.Results = fmt.Errorf("%v", string(msg))
				ch <- res
				break RESTART
			}

			channel, err := jsonparser.GetString(msg, "channel")
			if err != nil {
				l.Printf("[ERROR]: channel error: %+v", string(msg))
				res.Type = ERROR
				res.Results = fmt.Errorf("%v", string(msg))
				ch <- res
				break RESTART
			}

			market, err := jsonparser.GetString(msg, "market")
			if err != nil {
				l.Printf("[ERROR]: market err: %+v", string(msg))
				res.Type = ERROR
				res.Results = fmt.Errorf("%v", string(msg))
				ch <- res
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
					ch <- res
					break RESTART
				}
			}

			switch channel {
			case "ticker":
				res.Type = TICKER
				if err := json.Unmarshal(data, &res.Ticker); err != nil {
					l.Printf("[WARN]: cant unmarshal ticker %+v", err)
					continue
				}

			case "trades":
				res.Type = TRADES
				if err := json.Unmarshal(data, &res.Trades); err != nil {
					l.Printf("[WARN]: cant unmarshal trades %+v", err)
					continue
				}

			case "orderbook":
				res.Type = ORDERBOOK
				if err := json.Unmarshal(data, &res.Orderbook); err != nil {
					l.Printf("[WARN]: cant unmarshal orderbook %+v", err)
					continue
				}

			default:
				res.Type = UNDEFINED
				res.Results = fmt.Errorf("%v", string(msg))
			}

			ch <- res

		}
	}()

	return nil
}

func (e RowList) Len() int {
	return len(e)
}

func (e RowList) Less(i, j int) bool {
	return e[i].Price > e[j].Price
}

func (e RowList) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func SortAsks(original []Row, new []Row) *[]Row {
	original = append(original, new...)
	sort.Sort(RowList(original))

	var result []Row
	result = append(result, original[0])
	for i := 1; i < len(original); i++ {
		if result[len(result)-1].Price == original[i].Price && result[len(result)-1].Size < original[i].Size {
			result[len(result)-1].Size = original[i].Size
		} else {
			result = append(result, original[i])
		}
	}

	if len(result) > 50 {
		original = result[:50]
	}

	return &result
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	channels := []string{"orderbook"}
	pairs := []string{"BTC-PERP", "ETH-PERP"}
	ch := make(chan Response)
	go Connect(ctx, ch, channels, pairs, nil)

	type orderbookResult struct {
		Symbol string
		Result []Row
	}

	for {
		select {
		case v := <-ch:
			switch v.Type {
			case TICKER:
				fmt.Printf("%s	%+v\n", v.Symbol, v.Ticker)

			case ORDERBOOK:
				fmt.Printf("%s	%+v\n", v.Symbol, v.Orderbook)
				fmt.Printf("Bids: %+v\n", v.Orderbook.Bids)
				fmt.Printf("Asks: %+v\n", v.Orderbook.Asks)

				// tt := Row{}
				// orderbookResult[v.Symbol] = append(orderbookResult[v.Symbol], v.Orderbook.Bids)

			case UNDEFINED:
				fmt.Printf("%s	%s\n", v.Symbol, v.Results.Error())
			}
		}
	}
}
