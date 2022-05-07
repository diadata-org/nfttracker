package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"

	"github.com/jackc/pgx/v4/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/diadata-org/nfttracker/pkg/helper/events"
	"github.com/gorilla/websocket"
)

type WSRequest struct {
	Channel string //nftdeploy,nftmint, nftdetail
	Address string
}

type WSResponse struct {
	Error    string
	Response interface{}
}

var nftSubscribed []*websocket.Conn

var addr = flag.String("addr", "localhost:8080", "http service address")
var pgclient *pgxpool.Pool
var upgrader = websocket.Upgrader{} // use default options

func nft(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		var message WSRequest
		log.Printf("recv: %s", message)

		err := c.ReadJSON(&message)
		if err != nil {
			log.Println("err:", err)
		}
		switch message.Channel {

		case "nftdeploy":
			{
				nftSubscribed = append(nftSubscribed, c)
				msg := "subscibed to nftdeploy"
				c.WriteJSON(&WSResponse{Response: msg})

			}
		case "nftmint":
			{
				c.WriteJSON(&WSResponse{})

			}
		case "nftdetail":
			{

			}
		default:
			{
				c.WriteJSON(&WSResponse{Error: "Invalid command"})

			}

		}
		// err = c.WriteJSON(message)
		// if err != nil {
		// 	log.Println("write:", err)
		// 	break
		// }
	}
}

var (
	grpcaddr = flag.String("grpcaddr", "localhost:50051", "the address to connect to")
)
var messages chan string

func main() {

	messages = make(chan string)

	// send message to channels

	go func() {
		for {
			select {
			case msg := <-messages:
				for _, conn := range nftSubscribed {
					err := conn.WriteJSON(&WSResponse{Response: msg})
					if err != nil {
						log.Println(err)

					}
				}
			}
		}
	}()

	flag.Parse()
	log.SetFlags(0)
	conn, err := grpc.Dial(*grpcaddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewEventCollectorClient(conn)
	// }
	in := emptypb.Empty{}

	stream, err := c.NFTCollection(context.Background(), &in)
	if err != nil {
		log.Fatalf("open stream error %v", err)
	}
	done := make(chan bool)
	go func() {
		log.Println("listening to nftcollection")
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				done <- true //means stream is finished
				log.Println("listening to nftcollection")

				return
			}
			if err != nil {
				log.Fatalf("cannot receive %v", err)
			}
			log.Printf("Resp sending to chan: %s", resp)

			messages <- resp.Address
			log.Printf("Resp sent to chan: %s", resp)
		}
	}()

	http.HandleFunc("/nft", nft)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
