// Ref: https://github.com/go-numb/go-ftx/blob/3144aa68f5/realtime/websocket.go

package exchange

import (
	"context"
	util "crypto-flash/internal/service/util"
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

var (
	OrderbookRes map[string]*util.Orderbook = make(map[string]*util.Orderbook)
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

type AsksRow []util.Row

type BidsRow []util.Row

type Orderbook struct {
	Bids   [][]float64 `json:"bids"`
	Asks   [][]float64 `json:"asks"`
	Action string      `json:"action"`
}

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

func unsubscribe(conn *websocket.Conn, channel string, symbols []string) error {
	if symbols != nil {
		for j := range symbols {
			if err := conn.WriteJSON(&request{
				Op:      "unsubscribe",
				Channel: channel,
				Market:  symbols[j],
			}); err != nil {
				return err
			}
		}
	} else {
		if err := conn.WriteJSON(&request{
			Op:      "unsubscribe",
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
		defer unsubscribe(conn, channel, symbols)

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

				// Sort of Bids
				OrderbookRes[res.Symbol].Bids = *sortOrderbooks(
					OrderbookRes[res.Symbol].Bids,
					res.Orderbook.Bids,
					"bids",
				)

				// Sort of Asks
				OrderbookRes[res.Symbol].Asks = *sortOrderbooks(
					OrderbookRes[res.Symbol].Asks,
					res.Orderbook.Asks,
					"asks",
				)

			default:
				res.Type = UNDEFINED
				res.Results = fmt.Errorf("%v", string(msg))
			}
		}
	}()

	return nil
}

func (e BidsRow) Len() int {
	return len(e)
}

func (e AsksRow) Len() int {
	return len(e)
}

func (e BidsRow) Less(i, j int) bool {
	return e[i].Price > e[j].Price
}

func (e AsksRow) Less(i, j int) bool {
	return e[i].Price < e[j].Price
}

func (e BidsRow) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func (e AsksRow) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func sortOrderbooks(original []util.Row, new [][]float64, orderbookType string) *[]util.Row {
	var convertNewObj []util.Row
	for _, elem := range new {
		// Filter size = 0
		if elem[1] > 0 {
			orderbookRow := util.Row{elem[0], elem[1]}
			convertNewObj = append(convertNewObj, orderbookRow)
		}
	}

	original = append(original, convertNewObj...)
	if orderbookType == "bids" {
		sort.Sort(BidsRow(original))
	} else if orderbookType == "asks" {
		sort.Sort(AsksRow(original))
	}

	var result []util.Row
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

func GetOrderbookRes() map[string]*util.Orderbook {
	return OrderbookRes
}

func SubscribeOrderbook(pairs []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// initial pairs
	for _, val := range pairs {
		OrderbookRes[val] = &util.Orderbook{
			[]util.Row{},
			[]util.Row{},
		}
	}

	channel := "orderbook"
	ch := make(chan Response)
	go Connect(ctx, ch, channel, pairs, nil)

	return nil
}
