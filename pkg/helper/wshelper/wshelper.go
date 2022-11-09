package wshelper

import (
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

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
	log.Println("AddConnection: Total connection", len(wc.clients))
	wc.clients[conn] = Client{IsConnected: true}
}

func (wc *WSChannel) RemoveConnection(conn *websocket.Conn) {
	log.Println("RemoveConnection: exsiting status", wc.clients[conn].IsConnected)
	wc.clients[conn] = Client{IsConnected: false}
	log.Println("RemoveConnection: new status", wc.clients[conn].IsConnected)
	conn.Close()
	delete(wc.clients, conn)

}

func (wc *WSChannel) Ping(conn *websocket.Conn) {
	// wc.clients = append(wc.connections, ConnectedClient{client: conn, lastPing: time.Now()})
}

func (wc *WSChannel) Send(m interface{}) {
	log.Infoln("sending message to clients ", m)

	for conn, metadata := range wc.clients {

		if metadata.IsConnected {
			err := conn.WriteJSON(m)
			if err != nil {
				log.Errorln("error sending json to client", err)
			}

		}

	}
}
