package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"time"

	db "github.com/diadata-org/nfttracker/pkg/db"
	pb "github.com/diadata-org/nfttracker/pkg/helper/events"
	"github.com/diadata-org/nfttracker/pkg/helper/kafkaHelper"
	diatypes "github.com/diadata-org/nfttracker/pkg/types"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

var (
	log                  = logrus.New()
	NFT_COLLECTION_TABLE = "nftcollection"
	port                 = ":50051"
)

type server struct {
	Messages []chan string
	pb.UnimplementedEventCollectorServer
}

func (s *server) NFTCollection(_ *emptypb.Empty, server pb.EventCollector_NFTCollectionServer) error {
	singlemessage := make(chan string)
	s.Messages = append(s.Messages, singlemessage)

	for {
		msg := <-singlemessage
		resp := pb.CollectionCreated{Address: msg}

		server.Send(&resp)
	}

	return nil
}

func main() {

	pgclient := db.PostgresDatabase()
	flag.Parse()

	//--------------------------no

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()

	deployednftserver := server{}

	pb.RegisterEventCollectorServer(s, &deployednftserver)
	go func() {
		log.Printf("server listening at %v", lis.Addr())
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}

		log.Printf("end listening at ")

	}()
	//--------------------------no

	query := `CREATE TABLE IF NOT EXISTS ` + NFT_COLLECTION_TABLE + ` (
		address text UNIQUE,
		type text,
		time timestamp
	 );`

	_, err = pgclient.Query(context.Background(), query)
	if err != nil {
		log.Errorln("Error on pg query", err)
	}

	kafkaReader := kafkaHelper.NewReaderNextMessage(kafkaHelper.TopicNFTMINT)
	defer func() {
		err := kafkaReader.Close()
		if err != nil {
			log.Errorln(err)
		}
	}()

	for {

		m, err := kafkaReader.ReadMessage(context.Background())
		if err != nil {
			log.Errorln("error on kafka reader", err.Error())
		} else {

			var nftcreated diatypes.NFTCreation

			json.Unmarshal(m.Value, &nftcreated)

			go func() {
				log.Infoln("nft deployed", nftcreated)
				if len(deployednftserver.Messages) > 0 {
					for _, channel := range deployednftserver.Messages {
						channel <- nftcreated.Address
					}
				}

			}()

			err = insertIntoNFTCollection(nftcreated, pgclient)
			if err != nil {
				log.Errorln("error on updating pg", err.Error())
			}
		}
	}

}

func insertIntoNFTCollection(nftcreated diatypes.NFTCreation, client *pgxpool.Pool) error {
	query := fmt.Sprintf("insert into %s (address,type,time) values ($1,$2,$3)", NFT_COLLECTION_TABLE)
	log.Infoln("query", query)
	_, err := client.Exec(context.Background(), query, nftcreated.Address, nftcreated.NFTType, time.Now())
	if err != nil {
		return err
	}
	return nil
}
