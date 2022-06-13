# nfttracker

will be tracking latest NFTs being deployed in the chain


## Features of WoodPecker

   - Listen to new NFT contract deployed
   - Listen to New Mint happening

## API Usage

woodpecker provides websocket API to listen to certain events

Available channels
   - nftmint : Listen to all Mint events
   - nftdeploy : Listen to all new NFT deployed contract events


## Example 
   

## Listen to channel

````

const url = "ws://api.diadata.org/ws/nft";

const connection = new WebSocket(url);
connection.onopen = () => {
  console.log("Subscribe to nft mint")
 // subscribe
  connection.send("{"Channel": "nftmint"}");
};



````

to make your connection alive websocket clients need to send ping message every 5 seconds


````
  connection.send("{"Channel": "ping"}");

````


## Upcoming feature
   - Listen to Transfer events by NFT address
   - 

