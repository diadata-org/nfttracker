package main

import (
	"context"
	"math/big"
	"strings"

	"github.com/diadata-org/nfttracker/pkg/helper/kafkaHelper"
	diatypes "github.com/diadata-org/nfttracker/pkg/types"

	"github.com/segmentio/kafka-go"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

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
      }
]

`

func main() {
	client, err := ethclient.Dial("wss://eth-mainnet.alchemyapi.io/v2/UpWALFqrTh5m8bojhDcgtBIif-Ug5UUE")
	if err != nil {
		log.Fatal(err)
	}

	w := kafkaHelper.NewWriter(kafkaHelper.TopicNFTMINT)
	start := int64(14569635)

	for {
		head, _ := client.HeaderByNumber(context.Background(), big.NewInt(start))
		processHead(head, client, w)
		start = start + 1
	}

	log.Infoln("listening to all NFT contract deployed events")
	subscribeBlock(client, w)

	// fmt.Println(checkType(common.HexToAddress("0xFAFf15C6cDAca61a4F87D329689293E07c98f578"), client))

}

func checkType(contractAddress common.Address, client *ethclient.Client) (string, bool) {
	parsedAbi, err := abi.JSON(strings.NewReader(abistring))
	if err != nil {
		log.Errorln("Errorparsing ABI: %v", err)
	}
	contract := bind.NewBoundContract(contractAddress, parsedAbi, client, client, client)

	if isERC1155(contract) {
		return ERC1155, true
	}

	if isERC721(contract) {
		return ERC721, true
	}
	return "", false

}

func isERC1155(contract *bind.BoundContract) (isNFT bool) {
	var out []interface{}
	contract.Call(&bind.CallOpts{}, &out, "supportsInterface", InterfaceIdErc1155)

	if len(out) >= 1 {
		isNFT = *abi.ConvertType(out[0], new(bool)).(*bool)
	}
	return

}

func isERC721(contract *bind.BoundContract) (isNFT bool) {
	var out []interface{}
	contract.Call(&bind.CallOpts{}, &out, "supportsInterface", InterfaceIdErc721)
	if len(out) >= 1 {
		isNFT = *abi.ConvertType(out[0], new(bool)).(*bool)
	}
	return

}

func processHead(header *types.Header, client *ethclient.Client, w *kafka.Writer) {
	log.Println(header.Hash().Hex()) // 0xbc10defa8dda384c96a17640d84de5578804945d347072e091b4e5f390ddea7f

	block, err := client.BlockByHash(context.Background(), header.Hash())
	if err != nil {
		log.Fatal(err)
	}

	// block, err = client.BlockByNumber(context.Background(), big.NewInt(14658107))

	// fmt.Println(block.Hash().Hex())                      // 0xbc10defa8dda384c96a17640d84de5578804945d347072e091b4e5f390ddea7f
	// fmt.Println("Block number", block.Number().Uint64()) // 3477413
	// fmt.Println(block.Time().Uint64())     // 1529525947
	// fmt.Println("Total Transactions in block", len(block.Transactions())) // 7

	txs := block.Transactions()

	for _, tx := range txs {

		if tx.To() == nil {
			txr, _ := client.TransactionReceipt(context.Background(), tx.Hash())
			nftType, isNFt := checkType(txr.ContractAddress, client)
			if isNFt {
				message := diatypes.NFTCreation{Address: txr.ContractAddress.String(), NFTType: nftType}
				log.Infoln("nft contract deployed", message)
				kafkaHelper.WriteMessage(w, message)

			} else {
				// fmt.Println("-----------------Contract but not NFT-------------------", txr.ContractAddress.String())
			}
		}
	}
}

func subscribeBlock(client *ethclient.Client, w *kafka.Writer) {
	headers := make(chan *types.Header)
	sub, err := client.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		log.Errorln(err)
	}

	for {
		select {
		case err := <-sub.Err():
			log.Errorln(err)
		case header := <-headers:
			processHead(header, client, w)

		}
	}

}
