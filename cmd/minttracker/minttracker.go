package main

import (
	"context"
	"io"
	"net"
	"time"

	"sync"

	"github.com/diadata-org/nfttracker/pkg/db"
	pb "github.com/diadata-org/nfttracker/pkg/helper/events"
	"github.com/diadata-org/nfttracker/pkg/utils"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	diatypes "github.com/diadata-org/nfttracker/pkg/types"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/getsentry/sentry-go"
)

var (
	log = logrus.New()

	InterfaceIdErc165           = [4]byte{1, 255, 201, 167}  // 0x01ffc9a7
	InterfaceIdErc721           = [4]byte{128, 172, 88, 205} // 0x80ac58cd
	InterfaceIdErc721Metadata   = [4]byte{91, 94, 19, 159}   // 0x5b5e139f
	InterfaceIdErc721Enumerable = [4]byte{120, 14, 157, 99}  // 0x780e9d63
	InterfaceIdErc1155          = [4]byte{217, 182, 122, 38} // 0xd9b67a26

	ERC1155              = "ERC1155"
	ERC721               = "ERC721"
	LogAnySwapInbyte     = []byte("LogAnySwapIn(bytes32,address,address,uint,uint,uint)")
	logTransferSig       = []byte("Transfer(address,address,uint256)")
	logTransferSigHash   = crypto.Keccak256Hash(logTransferSig)
	NFT_COLLECTION_TABLE = "nftcollection"
	LogAnySwapInbyteHex  = crypto.Keccak256Hash(LogAnySwapInbyte)
	transferevent        chan pb.NFTTransaction
	mintgrpcPort         = ":50052"
	grpcaddr             = ""
)

type TransferTracker struct {
	WsClient     *ethclient.Client
	influxclient *db.DB
}

type server struct {
	pb.UnimplementedEventCollectorServer
}

func (s *server) NFTTransfer(_ *emptypb.Empty, server pb.EventCollector_NFTTransferServer) error {

	for {
		msg := <-transferevent
		server.Send(&msg)
	}

	return nil
}

func initsentry() {
	err := sentry.Init(sentry.ClientOptions{
		Dsn: "https://3983683f29344f40aef4e5125664dfb7@o1291064.ingest.sentry.io/6563600",
		// Set TracesSampleRate to 1.0 to capture 100%
		// of transactions for performance monitoring.
		// We recommend adjusting this value in production,
		TracesSampleRate: 1.0,
	})
	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}

	sentry.CaptureMessage("It works!")

}

func main() {
	initsentry()
	defer sentry.Flush(2 * time.Second)

	log.Infoln("starting Mint Tracker ...")
	grpcaddr = utils.Getenv("PERSISTOR_GRPC", "0.0.0.0:50051")
	ethws := utils.Getenv("ETH_URI_WS", "172.17.25.42:50051")

	transferevent = make(chan pb.NFTTransaction)

	lis, err := net.Listen("tcp", mintgrpcPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()

	pb.RegisterEventCollectorServer(s, &server{})
	go func() {
		log.Printf("server listening at %v", lis.Addr())
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	//------

	client, err := ethclient.Dial(ethws)
	if err != nil {
		log.Fatal("ethclient", err)
	}
	log.Println("connecting to influx")

	influxclient, err := db.NewDataStore()
	if err != nil {
		log.Fatal("influxclient", err)
	}

	log.Println("connected to influx")

	// head, _ := client.HeaderByNumber(context.Background(), big.NewInt(12543530))

	tt := &TransferTracker{WsClient: client, influxclient: influxclient}
	var wg sync.WaitGroup

	// wg.Add(1)

	go listenToDeployedNFT(&wg, tt)

	getNFTAddresstolisten(&wg, tt)

	wg.Wait()

}

func listenToDeployedNFT(wg *sync.WaitGroup, tt *TransferTracker) {

	log.Infoln("grpcaddr", grpcaddr)

	nftdeployedconn, err := grpc.Dial(grpcaddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Warnf("grpcaddr: %v %v", grpcaddr, err)
		return
	}
	defer nftdeployedconn.Close()
	c := pb.NewEventCollectorClient(nftdeployedconn)
	// }
	in := emptypb.Empty{}

	nftDeployedstream, err := c.NFTCollection(context.Background(), &in)
	if err != nil {
		log.Errorf("open stream error nftDeployedstream %v", err)
	}
	done := make(chan bool)

	log.Infoln("listening to nftdeployed")

	for {
		resp, err := nftDeployedstream.Recv()
		if err == io.EOF {
			done <- true //means stream is finished
			log.Infoln("ending  nftcollection")
		}

		if err != nil {
			log.Fatalf("cannot receive %v", err)
		} else {
			wg.Add(1)

			log.Infoln("Listening to newly deployed contract %s", resp.Address)

			go tt.subscribeNFT(resp.Address)

		}

	}

}

func getNFTAddresstolisten(wg *sync.WaitGroup, tt *TransferTracker) {
	pgclient := db.PostgresDatabase()
	//	query := `select address from ` + NFT_COLLECTION_TABLE + ` ORDER BY time DESC LIMIT 999 ;`

	query := `select address from ` + NFT_COLLECTION_TABLE + ` ORDER BY time DESC;`

	rows, err := pgclient.Query(context.Background(), query)
	if err != nil {
		log.Println("Error query", err)
	}

	defer rows.Close()

	for rows.Next() {
		var address string
		err := rows.Scan(&address)
		if err != nil {
			log.Errorln("error scanning row", err)
		}

		wg.Add(1)

		go tt.subscribeNFT(address)
	}

}

func (tt *TransferTracker) subscribeNFT(address string) {
	log.Infof("Subscribing to %s ", address)
	contractAddress := common.HexToAddress(address)
	query := ethereum.FilterQuery{
		// FromBlock: big.NewInt(14397771),
		// ToBlock:   big.NewInt(6383840),
		Addresses: []common.Address{
			contractAddress,
		},
	}

	events := make(chan types.Log)
	_, err := tt.WsClient.SubscribeFilterLogs(context.Background(), query, events)
	if err != nil {
		log.Fatal("error connecting to wsclient", err)
	}

	go func() {
		for {
			msg := <-events
			log.Infoln("recieved message")

			switch msg.Topics[0].Hex() {
			case logTransferSigHash.Hex():
				{
					mint := false
					log.Println("from tx", common.HexToAddress(msg.Topics[1].Hex()))
					log.Println("to tx", common.HexToAddress(msg.Topics[2].Hex()))
					log.Println("tokenid tx", msg.Topics[3].Hex())
					tx := pb.NFTTransaction{}
					tx.Address = msg.Address.Hex()
					tx.Txhash = msg.TxHash.Hex()

					if common.HexToAddress(msg.Topics[1].Hex()) == common.HexToAddress("0x0000000000000000000000000000000000000000") {
						mint = true
						transferevent <- tx
					}
					tt.influxclient.SaveNFTEvent(diatypes.NFTTransfer{Mint: mint, Address: msg.Address.Hex(), From: common.HexToAddress(msg.Topics[1].Hex()).Hex(), To: common.HexToAddress(msg.Topics[2].Hex()).Hex(), TransactionID: msg.TxHash.Hex()})
					tt.influxclient.Flush()
				}

			}

		}
	}()

}
