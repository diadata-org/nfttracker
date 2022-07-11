package wshelper

import (
	"log"

	"github.com/gorilla/websocket"
)

type Client struct {
	IsConnected       bool
	SubscribedAddress []string
}

type WSChannel struct {
	clients map[*websocket.Conn]Client
}

func NewChannel() *WSChannel {
	client := make(map[*websocket.Conn]Client)
	return &WSChannel{clients: client}
}

func (wc *WSChannel) AddConnection(conn *websocket.Conn) {
	wc.clients[conn] = Client{IsConnected: true}
}

func (wc *WSChannel) RemoveConnection(conn *websocket.Conn) {
	wc.clients[conn] = Client{IsConnected: false}
	conn.Close()

}

func (wc *WSChannel) Ping(conn *websocket.Conn) {
	// wc.clients = append(wc.connections, ConnectedClient{client: conn, lastPing: time.Now()})
}

func (wc *WSChannel) Send(m interface{}) {

	for conn, client := range wc.clients {
		if client.IsConnected {
			//TODO send only if last ping is in 5 minutes
			err := conn.WriteJSON(m)
			if err != nil {
				log.Println(err)
			}

		}

	}
}
