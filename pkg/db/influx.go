package db

import (
	"fmt"
	diatypes "github.com/diadata-org/nfttracker/pkg/types"
	"github.com/diadata-org/nfttracker/pkg/utils"
	clientInfluxdb "github.com/influxdata/influxdb1-client/v2"
	"time"
)

func GetInfluxClient(url string) clientInfluxdb.Client {
	var influxClient clientInfluxdb.Client
	var err error

	address := utils.Getenv("INFLUXURL", url)
	log.Info("INFLUXURL: ", address)
	username := utils.Getenv("INFLUXUSER", "")
	password := utils.Getenv("INFLUXPASSWORD", "")
	influxClient, err = clientInfluxdb.NewHTTPClient(clientInfluxdb.HTTPConfig{
		Addr:     address,
		Username: username,
		Password: password,
	})

	if err != nil {
		log.Error("NewDataStore influxdb", err)
	}

	return influxClient
}

type Datastore interface {
	SetInfluxClient(url string)
}

const (
	influxMaxPointsInBatch = 5000
	// timeOutRedisOneBlock   = 60 * 3 * time.Second
)

type DB struct {
	influxClient        clientInfluxdb.Client
	influxBatchPoints   clientInfluxdb.BatchPoints
	influxPointsInBatch int
}

const (
	influxDbName = "dia"

	influxDBDefaultURL = "http://influxdb:8086"
)

// queryInfluxDB convenience function to query the database.
func queryInfluxDB(clnt clientInfluxdb.Client, cmd string) (res []clientInfluxdb.Result, err error) {
	res, err = queryInfluxDBName(clnt, influxDbName, cmd)
	return
}

// queryInfluxDBName is a wrapper for queryInfluxDB that allows for queries on the database with name @dbName.
func queryInfluxDBName(clnt clientInfluxdb.Client, dbName string, cmd string) (res []clientInfluxdb.Result, err error) {
	q := clientInfluxdb.Query{
		Command:  cmd,
		Database: dbName,
	}
	if response, err := clnt.Query(q); err == nil {
		if response.Error() != nil {
			return res, response.Error()
		}
		res = response.Results
	} else {
		return res, err
	}
	return res, nil
}

func NewDataStore() (*DB, error) {
	var influxClient clientInfluxdb.Client
	var influxBatchPoints clientInfluxdb.BatchPoints

	var err error
	influxClient = GetInfluxClient(influxDBDefaultURL)
	influxBatchPoints = createBatchInflux()
	_, err = queryInfluxDB(influxClient, fmt.Sprintf("CREATE DATABASE %s", influxDbName))
	if err != nil {
		log.Errorln("queryInfluxDB CREATE DATABASE", err)
	}

	return &DB{influxClient, influxBatchPoints, 0}, nil
}

// SetInfluxClient resets influx's client url to @url.
func (datastore *DB) SetInfluxClient(url string) {
	datastore.influxClient = GetInfluxClient(url)
}

func createBatchInflux() clientInfluxdb.BatchPoints {
	bp, err := clientInfluxdb.NewBatchPoints(clientInfluxdb.BatchPointsConfig{
		Database:  influxDbName,
		Precision: "ns",
	})
	if err != nil {
		log.Errorln("NewBatchPoints", err)
	}
	return bp
}

func (datastore *DB) Flush() error {
	var err error
	if datastore.influxBatchPoints != nil {
		err = datastore.WriteBatchInflux()
	}
	return err
}

func (datastore *DB) WriteBatchInflux() (err error) {
	err = datastore.influxClient.Write(datastore.influxBatchPoints)
	if err != nil {
		log.Errorln("WriteBatchInflux", err)
		return
	}
	datastore.influxPointsInBatch = 0
	datastore.influxBatchPoints = createBatchInflux()
	return
}

func (datastore *DB) addPoint(pt *clientInfluxdb.Point) {
	datastore.influxBatchPoints.AddPoint(pt)
	datastore.influxPointsInBatch++
	if datastore.influxPointsInBatch >= influxMaxPointsInBatch {
		err := datastore.WriteBatchInflux()
		if err != nil {
			log.Error("add point to influx batch: ", err)
		}
	}
}

// SaveNFTEvent

func (datastore *DB) SaveNFTEvent(transfer diatypes.NFTTransfer) error {

	tags := map[string]string{
		"address": transfer.Address,
		// "pair":                 t.Pair,
		// "exchange":             t.Source,
		// "verified":             strconv.FormatBool(t.VerifiedPair),
		// "quotetokenaddress":    t.QuoteToken.Address,
		// "basetokenaddress":     t.BaseToken.Address,
		// "quotetokenblockchain": t.QuoteToken.Blockchain,
		// "basetokenblockchain":  t.BaseToken.Blockchain,
	}
	fields := map[string]interface{}{
		"from":          transfer.From,
		"to":            transfer.To,
		"mint":          transfer.Mint,
		"transactionid": transfer.TransactionID,
	}
	pt, err := clientInfluxdb.NewPoint("nfttransfer", tags, fields, time.Now())
	if err != nil {
		log.Errorln("nfttransfer:", err)
	} else {
		datastore.addPoint(pt)
	}
	return err

}

// SaveTradeInflux stores a trade in influx. Flushed when more than maxPoints in batch.
// // Wrapper around SaveTradeInfluxToTable.
// func (datastore *DB) SaveTradeInflux(t interface{}) error {
// 	return datastore.SaveTradeInfluxToTable(t, influxDbTradesTable)
// }

// // SaveTradeInfluxToTable stores a trade in influx into @table.
// // Flushed when more than maxPoints in batch.
// func (datastore *DB) SaveTradeInfluxToTable(t interface{}, table string) error {

// 	// Create a point and add to batch
// 	tags := map[string]string{
// 		"symbol":               t.Symbol,
// 		"pair":                 t.Pair,
// 		"exchange":             t.Source,
// 		"verified":             strconv.FormatBool(t.VerifiedPair),
// 		"quotetokenaddress":    t.QuoteToken.Address,
// 		"basetokenaddress":     t.BaseToken.Address,
// 		"quotetokenblockchain": t.QuoteToken.Blockchain,
// 		"basetokenblockchain":  t.BaseToken.Blockchain,
// 	}
// 	fields := map[string]interface{}{
// 		"price":             t.Price,
// 		"volume":            t.Volume,
// 		"estimatedUSDPrice": t.EstimatedUSDPrice,
// 		"foreignTradeID":    t.ForeignTradeID,
// 	}

// 	pt, err := clientInfluxdb.NewPoint(table, tags, fields, t.Time)
// 	if err != nil {
// 		log.Errorln("NewTradeInflux:", err)
// 	} else {
// 		datastore.addPoint(pt)
// 	}

// 	return err
// }
