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
	"github.com/diadata-org/nfttracker/pkg/helper/wshelper"

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

var nftDeploychannel *wshelper.WSChannel
var nftMintchannel *wshelper.WSChannel

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
				nftDeploychannel.AddConnection((c))
				msg := "subscibed to nftdeploy"
				c.WriteJSON(&WSResponse{Response: msg})

			}
		case "nftmint":
			{
				nftMintchannel.AddConnection((c))
				msg := "subscibed to nftmint"
				c.WriteJSON(&WSResponse{Response: msg})

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
	mintaddr = flag.String("mintaddr", "localhost:50052", "the address to connect to")
)
var nftdeployed chan string
var nftminted chan string

func main() {

	nftDeploychannel = wshelper.NewChannel()
	nftMintchannel = wshelper.NewChannel()

	nftdeployed = make(chan string)
	nftminted = make(chan string)

	flag.Parse()
	log.SetFlags(0)

	// send message to channels

	go func() {
		for {
			select {
			case msg := <-nftdeployed:
				nftDeploychannel.Send(&WSResponse{Response: msg})
				log.Println("----nft nftdeployed", msg)

			case msg := <-nftminted:
				log.Println("----nft minted", msg)
				nftMintchannel.Send(&WSResponse{Response: msg})
			}
		}
	}()

	//Listen to nftdeployed and nftmint events from restpected grpc streams

	nftdeployedconn, err := grpc.Dial(*grpcaddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer nftdeployedconn.Close()
	c := pb.NewEventCollectorClient(nftdeployedconn)
	// }
	in := emptypb.Empty{}

	nftDeployedstream, err := c.NFTCollection(context.Background(), &in)
	if err != nil {
		log.Fatalf("open stream error %v", err)
	}
	done := make(chan bool)
	go func() {
		log.Println("listening to nftcollection")
		for {
			resp, err := nftDeployedstream.Recv()
			if err == io.EOF {
				done <- true //means stream is finished
				log.Println("listening to nftcollection")
				return
			}
			if err != nil {
				log.Fatalf("cannot receive %v", err)
			}
			log.Printf("Resp sending to chan: %s", resp)

			nftminted <- resp.Address
			log.Printf("Resp sent to chan: %s", resp)
		}
	}()

	// listen to NFt mints

	nftmintedconn, err := grpc.Dial(*mintaddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer nftmintedconn.Close()
	nftmintev := pb.NewEventCollectorClient(nftmintedconn)
	// }
	in = emptypb.Empty{}

	nftmintstream, err := nftmintev.NFTTransfer(context.Background(), &in)
	if err != nil {
		log.Fatalf("open stream error %v", err)
	}

	log.Println("----")
	mintdone := make(chan bool)
	go func() {

		log.Println("listening to nftmintstream")
		for {

			resp, err := nftmintstream.Recv()
			if err == io.EOF {
				mintdone <- true //means stream is finished
				log.Println("listening to nftmintstream")

			}
			if err != nil {
				log.Fatalf("cannot receive %v", err)
			}
			log.Printf("Resp sending to chan: nftmintstream %s", resp)

			nftminted <- resp.Address
			log.Printf("Resp sent to chan: nftmintstream %s", resp)
		}
	}()

	http.HandleFunc("/nft", nft)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
