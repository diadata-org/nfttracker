package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"

	"github.com/diadata-org/nfttracker/pkg/db"
	"github.com/diadata-org/nfttracker/pkg/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/diadata-org/nfttracker/pkg/helper/events"
	"github.com/diadata-org/nfttracker/pkg/helper/wshelper"
	"github.com/diadata-org/nfttracker/pkg/utils"
	"github.com/ethereum/go-ethereum/common"

	"github.com/diadata-org/nfttracker/pkg/helper/kafkaHelper"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type WSRequest struct {
	Channel  string //nftdeploy,nftmint, nftdetail
	Address  []string
	Duration string
}

type WSResponse struct {
	Error    string `json:",omitempty"`
	Response interface{}
	Channel  string
}

type WSMintStatsResponse struct {
	Address   string
	Mint      int64
	TotalMint int64

	Duration string
}

const (
	NFT_MINT_CHANNEL     = "nftmint"
	NFT_DEPLOY_CHANNEL   = "nftdeploy"
	NFT_TRANSFER_CHANNEL = "nfttransfer"
	NFT_SALES_CHANNEL    = "nftsales"

	NFT_MINT_STATS_CHANNEL = "nftstats"

	PING = "ping"
)

var (
	log                = logrus.New()
	nftDeploychannel   *wshelper.WSChannel
	nftMintchannel     *wshelper.WSChannel
	nftTransferchannel *wshelper.WSChannel
	nftTradechannel    *wshelper.WSChannel

	addr        = flag.String("addr", ":8080", "http service address")
	upgrader    = websocket.Upgrader{} // use default options
	nftdeployed chan string
	grpcaddr    string
	mintaddr    string
	nftminted   chan pb.NFTTransaction
	nfttransfer chan pb.NFTTransaction
	nfttrades   chan types.NFTTrade

	influxclient *db.DB
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
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Errorln("IsUnexpectedCloseError error: %v", err)

			}
			nftDeploychannel.RemoveConnection(c)
			nftMintchannel.RemoveConnection(c)

			break
		}
		switch message.Channel {

		case NFT_DEPLOY_CHANNEL:
			{
				nftDeploychannel.AddConnection((c))
				msg := "subscribed to " + message.Channel
				c.WriteJSON(&WSResponse{Response: msg, Channel: message.Channel})

			}
		case NFT_TRANSFER_CHANNEL:
			{
				nftTransferchannel.AddConnection((c))
				msg := "subscribed to " + message.Channel
				c.WriteJSON(&WSResponse{Response: msg})

			}
		case NFT_MINT_CHANNEL:
			{
				nftMintchannel.AddConnection((c))
				msg := "subscribed to " + message.Channel
				c.WriteJSON(&WSResponse{Response: msg, Channel: message.Channel})
			}
		case NFT_SALES_CHANNEL:
			{
				nftTradechannel.AddConnection((c))
				msg := "subscibed to " + message.Channel
				c.WriteJSON(&WSResponse{Response: msg})
			}
		case NFT_MINT_STATS_CHANNEL:
			{
				if message.Address != nil && len(message.Address) > 0 || message.Duration != "" {
					for _, address := range message.Address {
						count, err := influxclient.GetMintStats(message.Duration, address)
						if err != nil {
							log.Errorln("error getting minstats from influx", err)
						}

						totalMint, err := influxclient.GetMintStats("0", address)
						if err != nil {
							log.Errorln("error getting minstats from influx", err)
						}
						c.WriteJSON(&WSResponse{Response: WSMintStatsResponse{Address: address, Duration: message.Duration, Mint: count, TotalMint: totalMint}, Channel: message.Channel})
					}
				} else {
					c.WriteJSON(&WSResponse{Error: "Missing addresses or duration", Channel: message.Channel})

				}

			}
		case PING:
			{
				msg := "alive"
				c.WriteJSON(&WSResponse{Response: msg})
			}
		default:
			{
				c.WriteJSON(&WSResponse{Error: "Invalid Command", Channel: message.Channel})

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

	grpcaddr = utils.Getenv("PERSISTOR_GRPC", "127.0.0.1:50051")
	mintaddr = utils.Getenv("MINTTRACKER_GRPC", "127.0.0.1:50052")

	log.Infoln("PERSISTOR_GRPC", grpcaddr)
	log.Infoln("MINTTRACKER_GRPC", mintaddr)
	var err error
	influxclient, err = db.NewDataStore()
	if err != nil {
		log.Fatal("influxclient", err)
	}

	kafkaReader := kafkaHelper.NewReaderNextMessage(kafkaHelper.TopicNFTTrades)
	defer func() {
		err := kafkaReader.Close()
		if err != nil {
			log.Errorln(err)
		}
	}()

	nftDeploychannel = wshelper.NewChannel()
	nftMintchannel = wshelper.NewChannel()
	nftTransferchannel = wshelper.NewChannel()
	nftTradechannel = wshelper.NewChannel()

	nftdeployed = make(chan string)
	nftminted = make(chan pb.NFTTransaction)
	nfttransfer = make(chan pb.NFTTransaction)
	nfttrades = make(chan types.NFTTrade)

	flag.Parse()

	// send message to channels

	go func() {
		for {
			select {
			case msg := <-nftdeployed:
				log.Infoln("nft deployed", msg)
				nftDeploychannel.Send(&WSResponse{Response: msg})

			case msg := <-nftminted:
				log.Infoln("nft minted", msg)
				nftMintchannel.Send(&WSResponse{Response: msg})

			case msg := <-nfttransfer:
				log.Infoln("nft transfer", msg)
				nftTransferchannel.Send(&WSResponse{Response: msg})

			case msg := <-nfttrades:
				log.Infoln("nft trade", msg)
				nftTradechannel.Send(&WSResponse{Response: msg})

			}
		}

	}()

	//Listen to nftdeployed and nftmint events from restpected grpc streams

	nftdeployedconn, err := grpc.Dial(grpcaddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("error connecting to persistor grpc: %v", err)
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

			}
			if err != nil {
				log.Fatalf("cannot receive %v", err)
			}
			log.Infoln("Resp sending nftdeployedto chan: %s", resp)
			nftdeployed <- resp.Address
			log.Infoln("Resp sent to chan: %s nftdeployed", resp)
		}
	}()

	// listen to NFt mints

	nftmintedconn, err := grpc.Dial(mintaddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("error connecting to minttracker grpc: %v", err)
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

			if resp.GetAddress() == common.HexToAddress("0x0000000000000000000000000000000000000000").Hex() {
				nftminted <- *resp
			} else {
				nfttransfer <- *resp
			}

			log.Infoln("Resp sent to chan: nftmintstream %s", resp)
		}
	}()

	go func() {
		for {

			m, err := kafkaReader.ReadMessage(context.Background())
			if err != nil {
				log.Errorln("error on kafka reader", err.Error())
			} else {

				var nfttrade types.NFTTrade

				json.Unmarshal(m.Value, &nfttrade)

				nfttrades <- nfttrade

				if err != nil {
					log.Errorln("error on updating pg", err.Error())
				}
			}
		}
	}()

	http.HandleFunc("/ws/nft", nft)

	log.Infoln("Server running at", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
