package main

import (
	"context"
	"net"

	"math/big"

	"sync"

	"github.com/diadata-org/nfttracker/pkg/db"
	pb "github.com/diadata-org/nfttracker/pkg/helper/events"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	diatypes "github.com/diadata-org/nfttracker/pkg/types"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
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

func main() {
	log.Println("minttracker")

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

	client, err := ethclient.Dial("wss://eth-mainnet.alchemyapi.io/v2/UpWALFqrTh5m8bojhDcgtBIif-Ug5UUE")
	if err != nil {
		log.Fatal("ethclient", err)
	}
	log.Println("conneting to influx")

	influxclient, err := db.NewDataStore()
	if err != nil {
		log.Fatal("influxclient", err)
	}

	log.Println("conneted to influx")

	// head, _ := client.HeaderByNumber(context.Background(), big.NewInt(12543530))

	tt := &TransferTracker{WsClient: client, influxclient: influxclient}
	var wg sync.WaitGroup

	addresses := getNFTAddresstolisten()
	for _, address := range addresses {
		log.Println(address)
		wg.Add(1)

		go tt.subscribeNFT(address)

	}
	wg.Wait()

}

func getNFTAddresstolisten() (addresses []string) {
	pgclient := db.PostgresDatabase()

	query := `select address from ` + NFT_COLLECTION_TABLE + ` ORDER BY time DESC LIMIT 999 ;`

	rows, err := pgclient.Query(context.Background(), query)
	if err != nil {
		log.Println("Error query", err)
	}

	log.Println("rows", rows)

	defer rows.Close()

	for rows.Next() {
		var address string
		err := rows.Scan(&address)
		if err != nil {
			log.Println("err", err)
		}
		addresses = append(addresses, address)
	}

	return

}

func (tt *TransferTracker) subscribeNFT(address string) {
	log.Println("Subscribing to ", address)
	contractAddress := common.HexToAddress(address)
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(14397771),
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
			log.Infoln("recieved message", msg)

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
					}
					transferevent <- tx
					tt.influxclient.SaveNFTEvent(diatypes.NFTTransfer{Mint: mint, Address: msg.Address.Hex(), From: common.HexToAddress(msg.Topics[1].Hex()).Hex(), To: common.HexToAddress(msg.Topics[2].Hex()).Hex(), TransactionID: msg.TxHash.Hex()})
					tt.influxclient.Flush()
				}

			}

		}
	}()

}
