package main

import (
	"context"
	// "fmt"
	"log"
	"math/big"
	// "strings"
	"sync"

	"github.com/diadata-org/nfttracker/pkg/db"
	"github.com/diadata-org/nfttracker/pkg/helper/kafkaHelper"

	diatypes "github.com/diadata-org/nfttracker/pkg/types"

	"github.com/segmentio/kafka-go"

	// "github.com/ethereum/go-ethereum/accounts/abi"
	// "github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	InterfaceIdErc165           = [4]byte{1, 255, 201, 167}  // 0x01ffc9a7
	InterfaceIdErc721           = [4]byte{128, 172, 88, 205} // 0x80ac58cd
	InterfaceIdErc721Metadata   = [4]byte{91, 94, 19, 159}   // 0x5b5e139f
	InterfaceIdErc721Enumerable = [4]byte{120, 14, 157, 99}  // 0x780e9d63
	InterfaceIdErc1155          = [4]byte{217, 182, 122, 38} // 0xd9b67a26

	ERC1155 = "ERC1155"
	ERC721  = "ERC721"
)

const abistring = `[
    {
        "inputs": [
          { "internalType": "bytes4", "name": "interfaceId", "type": "bytes4" }
        ],
        "name": "supportsInterface",
        "outputs": [{ "internalType": "bool", "name": "", "type": "bool" }],
        "stateMutability": "view",
        "type": "function"
      },
	  {
		"anonymous": false,
		"inputs": [
		  {
			"indexed": true,
			"internalType": "address",
			"name": "from",
			"type": "address"
		  },
		  {
			"indexed": true,
			"internalType": "address",
			"name": "to",
			"type": "address"
		  },
		  {
			"indexed": true,
			"internalType": "uint256",
			"name": "tokenId",
			"type": "uint256"
		  }
		],
		"name": "Transfer",
		"type": "event"
	  }
]


`

var logTransferSig = []byte("Transfer(address,address,uint256)")
var logTransferSigHash = crypto.Keccak256Hash(logTransferSig)

type TransferTracker struct {
	WsClient     *ethclient.Client
	w            *kafka.Writer
	influxclient *db.DB
}

const NFT_COLLECTION_TABLE = "nftcollection"

func main() {
	client, err := ethclient.Dial("wss://eth-mainnet.alchemyapi.io/v2/UpWALFqrTh5m8bojhDcgtBIif-Ug5UUE")
	if err != nil {
		log.Fatal(err)
	}

	influxclient, _ := db.NewDataStore()
	w := kafkaHelper.NewWriter(kafkaHelper.TopicNFTMINT)

	pgclient := db.PostgresDatabase()

	query := `select address from  ` + NFT_COLLECTION_TABLE + `;`

	rows, err := pgclient.Query(context.Background(), query)
	if err != nil {
		log.Println("Error query", err)
	}

	log.Println(rows)

	defer rows.Close()

	var addresses []string
	for rows.Next() {
		var address string
		err := rows.Scan(&address)
		if err != nil {
			log.Println(err)
		}
		addresses = append(addresses, address)
	}

	// head, _ := client.HeaderByNumber(context.Background(), big.NewInt(12543530))

	tt := &TransferTracker{WsClient: client, w: w, influxclient: influxclient}
	var wg sync.WaitGroup

	for _, address := range addresses {
		log.Println(address)
		wg.Add(1)

		go tt.subscribeNFT(address)

	}
	wg.Wait()

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
		log.Fatal(err)
	}

	go func() {
		for {
			msg := <-events
			log.Println("recieved message", msg)
			log.Println("topics", msg.Topics)
			log.Println("Data", msg.Data)
			log.Println("TxHash", msg.TxHash)

			switch msg.Topics[0].Hex() {
			case logTransferSigHash.Hex():
				{
					mint := false
					log.Println("from tx", common.HexToAddress(msg.Topics[1].Hex()))
					log.Println("to tx", common.HexToAddress(msg.Topics[2].Hex()))
					log.Println("tokenid tx", msg.Topics[3].Hex())
					if common.HexToAddress(msg.Topics[1].Hex()) == common.HexToAddress("0x0000000000000000000000000000000000000000") {
						mint = true
					}
					tt.influxclient.SaveNFTEvent(diatypes.NFTTransfer{Mint: mint, Address: msg.Address.Hex(), From: common.HexToAddress(msg.Topics[1].Hex()).Hex(), To: common.HexToAddress(msg.Topics[2].Hex()).Hex(), TransactionID: msg.TxHash.Hex()})
					tt.influxclient.Flush()
				}

			}

		}
	}()

	// contractAbi, err := abi.JSON(strings.NewReader(abistring))
	// if err != nil {
	// 	log.Fatal("contractAbi", err)
	// }

	// log.Println(contractAbi)

	// logTransferSig := []byte("Transfer(address,address,uint256)")
	// logTransferSigHash := crypto.Keccak256Hash(logTransferSig)
	// for _, vLog := range logs {
	// 	fmt.Printf("Log Block Number: %d\n", vLog.BlockNumber)
	// 	fmt.Printf("Log Index: %d\n", vLog.Index)

	// 	switch vLog.Topics[0].Hex() {
	// 	case logTransferSigHash.Hex():
	// 		log.Println("transfer")
	// 		//
	// 		//
	// 	}
	// }
}
