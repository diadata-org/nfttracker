package wshelper

import (
	"log"

	"github.com/gorilla/websocket"
)

type WSChannel struct {
	connections []*websocket.Conn
}

func NewChannel() *WSChannel {
	connections := []*websocket.Conn{}
	return &WSChannel{connections: connections}
}

func (wc *WSChannel) AddConnection(conn *websocket.Conn) {
	wc.connections = append(wc.connections, conn)
}

func (wc *WSChannel) Send(m interface{}) {
	if len(wc.connections) > 0 {

		for _, conn := range wc.connections {
			err := conn.WriteJSON(m)
			if err != nil {
				log.Println(err)

			}
		}
	}
}
