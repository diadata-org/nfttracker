package wshelper

import (
	"log"

	"github.com/gorilla/websocket"
)

type WSChannel struct {
	clients map[*websocket.Conn]bool
}

func NewChannel() *WSChannel {
	client := make(map[*websocket.Conn]bool)
	return &WSChannel{clients: client}
}

func (wc *WSChannel) AddConnection(conn *websocket.Conn) {
	wc.clients[conn] = true
}

func (wc *WSChannel) RemoveConnection(conn *websocket.Conn) {
	wc.clients[conn] = false
	conn.Close()

}

func (wc *WSChannel) Ping(conn *websocket.Conn) {
	// wc.clients = append(wc.connections, ConnectedClient{client: conn, lastPing: time.Now()})
}

func (wc *WSChannel) Send(m interface{}) {

	for client, isConnected := range wc.clients {
		if isConnected {
			//TODO send only if last ping is in 5 minutes
			err := client.WriteJSON(m)
			if err != nil {
				log.Println(err)
			}

		}

	}
}
