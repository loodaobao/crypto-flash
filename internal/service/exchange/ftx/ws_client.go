package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsScheme = "wss"
	wsHost   = "ftx.com"
	wsPath   = "/ws"
)

type WS struct {
	*websocket.Conn
}

type PayloadStruct struct {
	Op      string `json:"op"`
	Channel string `json:"channel"`
	Market  string `json:"market"`
}

func Connect() *websocket.Conn {
	u := url.URL{Scheme: wsScheme, Host: wsHost, Path: wsPath}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	return c
}

func (c *WS) SubscribePairs(channel string, pairs []string) {

	for _, pair := range pairs {
		fmt.Println("pair: ", pair)
		payload := &PayloadStruct{
			Op:      "subscribe",
			Channel: channel,
			Market:  pair,
		}

		c.sendMessage(payload)
	}

}

func (c *WS) sendMessage(payload *PayloadStruct) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	done := make(chan struct{})

	fmt.Println("payload: ", payload)

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
		}
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			// {"op": "ping"}. You will see an {"type": "pong"}
			err := c.WriteJSON(payload)
			if err != nil {
				log.Println("write:", err)
				return
			}
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}

}

func main() {
	c := Connect()
	defer c.Close()

	ws := &WS{c}
	pairs := []string{"BTC-PERP", "ETH-PERP"}
	ws.SubscribePairs("orderbook", pairs)
}
