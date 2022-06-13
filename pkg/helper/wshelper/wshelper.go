package wshelper

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

type ConnectedClient struct {
	client   *websocket.Conn
	lastPing time.Time
}

type WSChannel struct {
	connections []ConnectedClient
}

func NewChannel() *WSChannel {
	connections := []ConnectedClient{}
	return &WSChannel{connections: connections}
}

func (wc *WSChannel) AddConnection(conn *websocket.Conn) {
	wc.connections = append(wc.connections, ConnectedClient{client: conn, lastPing: time.Now()})
}

func (wc *WSChannel) Ping(conn *websocket.Conn) {
	wc.connections = append(wc.connections, ConnectedClient{client: conn, lastPing: time.Now()})
}

func (wc *WSChannel) Send(m interface{}) {
	if len(wc.connections) > 0 {
		for _, conn := range wc.connections {
			//TODO send only if last ping is in 5 minutes
			err := conn.client.WriteJSON(m)
			if err != nil {
				log.Println(err)
			}
		}
	}
}
