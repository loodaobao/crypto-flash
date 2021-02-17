package main

import (
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

type ws struct {
	*websocket.Conn
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

func (c *ws) sendMessage() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	done := make(chan struct{})

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
			// TODO: Use `WriteJSON`
			// {"op": "ping"}. You will see an {"type": "pong"}
			err := c.WriteMessage(
				websocket.TextMessage,
				[]byte(`{"op": "subscribe", "channel": "orderbook", "market": "BTC-PERP"}`),
			)
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
	ws := &ws{c}
	ws.sendMessage()
}
