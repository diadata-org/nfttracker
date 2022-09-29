package main

import (
	"time"

	"github.com/diadata-org/nfttracker/pkg/utils"
	"github.com/diadata-org/nfttracker/pkg/utils/probes"
	"github.com/gin-gonic/gin"
)

func startProbe() {

	engine := gin.Default()
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())

	log.Infoln("starting probes")

	probes.Start(live, ready)

	go func() {
		err := engine.Run(utils.Getenv("LISTEN_PORT", ":8080"))
		if err != nil {
			log.Error("error running probe server", err)
		}
	}()

	go func() {
		for {

			lastminute := <-lastminttxupdatechan
			lastminttx = lastminute
		}

	}()

}

func ready() bool {
	return StartupDone
}

func live() bool {
	if !StartupDone {
		return false
	}
	// restart if nbo mint tx in 4 hour
	return time.Since(lastminttx).Hours() < 4
}
