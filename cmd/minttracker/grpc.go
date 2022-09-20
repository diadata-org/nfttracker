package main

import (
	pb "github.com/diadata-org/nfttracker/pkg/helper/events"
	"google.golang.org/protobuf/types/known/emptypb"
)

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
