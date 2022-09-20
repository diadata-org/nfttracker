package probes

import (
	"errors"
	"net/http"

	"github.com/diadata-org/diadata/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type probe func() bool

var log = logrus.New()

var livenessProbe probe
var readinessProbe probe

func Start(liveness probe, readiness probe) {

	log.Infoln("Ready and Live probes loading")
	livenessProbe = liveness
	readinessProbe = readiness

	engine := gin.Default()
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())

	engine.GET("/ready", execReadiness)
	engine.GET("/live", execLiveness)
	// This environment variable is either set in docker-compose or empty
	go func() {
		err := engine.Run(utils.Getenv("LISTEN_PORT_PROBES", ":2345"))
		if err != nil {
			log.Error(err)
		}
	}()
	log.Infoln("Ready and Live probes starting")
}

func execReadiness(context *gin.Context) {
	executeProbe(context, readinessProbe)
}

func execLiveness(context *gin.Context) {
	executeProbe(context, livenessProbe)
}

func executeProbe(context *gin.Context, fn probe) bool {
	log.Infoln("probe has been started")
	if fn() {
		context.JSON(http.StatusOK, gin.H{"message": "success"})
		return true
	}
	SendError(context, http.StatusInternalServerError, errors.New(""))
	return false
}

func SendError(c *gin.Context, errorCode int, err error) {
	c.JSON(errorCode,
		&APIError{
			ErrorCode:    errorCode,
			ErrorMessage: err.Error(),
		})
}

type APIError struct {
	ErrorCode    int    `json:"errorcode"`
	ErrorMessage string `json:"errormessage"`
}
