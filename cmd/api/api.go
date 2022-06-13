package main

import (
	"context"
	"flag"
	"io"
	"net/http"

	"github.com/jackc/pgx/v4/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/diadata-org/nfttracker/pkg/helper/events"
	"github.com/diadata-org/nfttracker/pkg/helper/wshelper"
	"github.com/diadata-org/nfttracker/pkg/utils"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type WSRequest struct {
	Channel string //nftdeploy,nftmint, nftdetail
	Address string
}

type WSResponse struct {
	Error    string
	Response interface{}
}

const (
	NFT_MINT_CHANNEL   = "nftmint"
	NFT_DEPLOY_CHANNEL = "nftdeploy"
	PING               = "ping"
)

var (
	log              = logrus.New()
	nftDeploychannel *wshelper.WSChannel
	nftMintchannel   *wshelper.WSChannel
	addr             = flag.String("addr", ":8080", "http service address")
	pgclient         *pgxpool.Pool
	upgrader         = websocket.Upgrader{} // use default options
	nftdeployed      chan string
	grpcaddr         string
	mintaddr         string
	nftminted        chan string
)

func nft(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorln("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		var message WSRequest

		err := c.ReadJSON(&message)
		if err != nil {
			log.Errorln("err:", err)
		}
		switch message.Channel {

		case NFT_DEPLOY_CHANNEL:
			{
				nftDeploychannel.AddConnection((c))
				msg := "subscribed to " + message.Channel
				c.WriteJSON(&WSResponse{Response: msg})

			}
		case NFT_MINT_CHANNEL:
			{
				nftMintchannel.AddConnection((c))
				msg := "subscibed to " + message.Channel
				c.WriteJSON(&WSResponse{Response: msg})

			}
		case PING:
			{
				msg := "alive"
				c.WriteJSON(&WSResponse{Response: msg})
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

func main() {

	grpcaddr = utils.Getenv("PERSISTOR_GRPC", "172.17.25.42:50051")
	mintaddr = utils.Getenv("MINTTRACKER_GRPC", "172.17.25.42:50052")

	nftDeploychannel = wshelper.NewChannel()
	nftMintchannel = wshelper.NewChannel()

	nftdeployed = make(chan string)
	nftminted = make(chan string)

	flag.Parse()

	// send message to channels

	go func() {
		for {
			select {
			case msg := <-nftdeployed:
				nftDeploychannel.Send(&WSResponse{Response: msg})

			case msg := <-nftminted:
				nftMintchannel.Send(&WSResponse{Response: msg})
			}
		}
	}()

	//Listen to nftdeployed and nftmint events from restpected grpc streams

	nftdeployedconn, err := grpc.Dial(grpcaddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("grpcaddr: %v", grpcaddr)

		log.Fatalf("did not connect: %v", err)
	}
	defer nftdeployedconn.Close()
	c := pb.NewEventCollectorClient(nftdeployedconn)
	// }
	in := emptypb.Empty{}

	nftDeployedstream, err := c.NFTCollection(context.Background(), &in)
	if err != nil {
		log.Fatalf("open stream error nftDeployedstream %v", err)
	}
	done := make(chan bool)
	go func() {
		log.Println("listening to nftcollection")
		for {
			resp, err := nftDeployedstream.Recv()
			if err == io.EOF {
				done <- true //means stream is finished
				log.Infoln("listening to nftcollection")
				return
			}
			if err != nil {
				log.Fatalf("cannot receive %v", err)
			}
			log.Infoln("Resp sending to chan: %s", resp)

			nftminted <- resp.Address
			log.Infoln("Resp sent to chan: %s", resp)
		}
	}()

	// listen to NFt mints

	nftmintedconn, err := grpc.Dial(mintaddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer nftmintedconn.Close()
	nftmintev := pb.NewEventCollectorClient(nftmintedconn)
	// }
	in = emptypb.Empty{}

	nftmintstream, err := nftmintev.NFTTransfer(context.Background(), &in)
	if err != nil {
		log.Fatalf("open stream error nftmintstream %v", err)
	}

	mintdone := make(chan bool)
	go func() {

		log.Infoln("listening to nftmintstream")
		for {

			resp, err := nftmintstream.Recv()
			if err == io.EOF {
				mintdone <- true //means stream is finished
				log.Infoln("listening to nftmintstream")

			}
			if err != nil {
				log.Fatalf("cannot receive %v", err)
			}
			nftminted <- resp.Address
			log.Infoln("Resp sent to chan: nftmintstream %s", resp)
		}
	}()

	http.HandleFunc("/ws/nft", nft)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
