const WebSocketClient = require('websocket').client;
 
var client = new WebSocketClient();
 
keepAlive = async(connection) => {
    connection.send(JSON.stringify({"Channel":"ping"}));
    setTimeout(()=>keepAlive(connection), 5000); //ping 5 seconds
}
 
client.on('connectFailed', function(error) {
    console.log('Connect Error: ' + error.toString());
});
 
client.on('connect', function(connection) {
    connection.send(JSON.stringify({"Channel": "nftmint"}));
    connection.send(JSON.stringify({"Channel": "nftdeploy"}));
    connection.send(JSON.stringify({"Channel": "nfttransfer"}));


    
    connection.on('error', function(error) {
        console.log("Connection error: " + error.toString());
    });
    
    connection.on('close', function() {
        console.log("connection closed");
    });
    
    connection.on('message', function(message) {
        console.log(message);
    });
    keepAlive(connection);
});
 
client.connect('wss://api.diadata.org/ws/nft'); //connect to diadata ws api

// client.connect('ws://127.0.0.1:8080/ws/nft'); //connect to diadata ws api