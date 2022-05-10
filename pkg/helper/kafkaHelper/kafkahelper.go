package kafkaHelper

import (
	"context"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
)

const (
	messageSizeMax = 1e8
)

type KafkaMessage interface {
	MarshalBinary() ([]byte, error)
}

type KafkaMessageWithAHash interface {
	Hash() string
}

const (
	TopicIndexBlock = 0

	TopicFiltersBlock = 1
	TopicTrades       = 2
	TopicTradesBlock  = 3

	TopicFiltersBlockHistorical = 4
	TopicTradesHistorical       = 5
	TopicTradesBlockHistorical  = 6

	TopicTradesEstimation = 7

	TopicFiltersBlockDone = 14

	retryDelay           = 2 * time.Second
	TopicOptionOrderBook = 13
	TopicNFTMINT         = 20
)

type Config struct {
	KafkaUrl []string
}

var KafkaConfig Config

func GetTopic(topic int) string {
	return getTopic(topic)
}

func getTopic(topic int) string {
	topicMap := map[int]string{
		1:  "filtersBlock",
		2:  "trades",
		3:  "tradesBlock",
		4:  "filtersBlockHistorical",
		5:  "tradesHistorical",
		6:  "tradesBlockHistorical",
		7:  "tradesEstimation",
		14: "filtersblockHistoricalDone",
		20: "TopicNFTMINT",
	}
	result, ok := topicMap[topic]
	if !ok {
		log.Error("getTopic cant find topic", topic)
	}
	return result
}

func init() {
	KafkaConfig.KafkaUrl = []string{os.Getenv("KAFKAURL")}
}

// WithRetryOnError
func ReadOffset(topic int) (offset int64, err error) {
	for _, ip := range KafkaConfig.KafkaUrl {
		var conn *kafka.Conn
		conn, err = kafka.DialLeader(context.Background(), "tcp", ip, getTopic(topic), 0)
		if err != nil {
			log.Errorln("ReadOffset conn error: <", err, "> ", ip)
		} else {
			offset, err = conn.ReadLastOffset()
			if err != nil {
				log.Errorln("ReadOffset ReadLastOffset error: <", err, "> ")
			} else {
				return
			}
			defer func() {
				cerr := conn.Close()
				if err == nil {
					err = cerr
				}
			}()
		}
	}
	return
}

func ReadOffsetWithRetryOnError(topic int) (offset int64) {
	// TO DO: check double infinite for loops.
	for {
		for {
			for _, ip := range KafkaConfig.KafkaUrl {
				conn, err := kafka.DialLeader(context.Background(), "tcp", ip, getTopic(topic), 0)
				if err != nil {
					log.Errorln("ReadOffsetWithRetryOnError conn error: <", err, "> ", ip, " topic:", topic)
					time.Sleep(retryDelay)
				} else {
					defer func() {
						err = conn.Close()
						if err != nil {
							log.Error(err)
						}
					}()

					offset, err = conn.ReadLastOffset()
					if err != nil {
						log.Errorln("ReadOffsetWithRetryOnError ReadLastOffset error: <", err, "> ", ip, " topic:", topic)
						time.Sleep(retryDelay)
					} else {
						return offset
					}
				}
			}
		}
	}
}

func NewWriter(topic int) *kafka.Writer {
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:  KafkaConfig.KafkaUrl,
		Topic:    getTopic(topic),
		Balancer: &kafka.LeastBytes{},
		Async:    true,
	})
}

func NewSyncWriter(topic int) *kafka.Writer {
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:    KafkaConfig.KafkaUrl,
		Topic:      getTopic(topic),
		Balancer:   &kafka.LeastBytes{},
		Async:      false,
		BatchBytes: 1e9, // 1GB
	})
}

func NewReader(topic int) *kafka.Reader {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   KafkaConfig.KafkaUrl,
		Topic:     getTopic(topic),
		Partition: 0,
		MinBytes:  0,
		MaxBytes:  10e6, // 10MB
	})
	return r
}

func WriteMessage(w *kafka.Writer, m KafkaMessage) error {
	key := []byte("helloKafka")
	value, err := m.MarshalBinary()
	if err == nil && value != nil {
		err = w.WriteMessages(context.Background(),
			kafka.Message{
				Key:   key,
				Value: value,
			},
		)
		if err != nil {
			log.Errorln("WriteMessage error:", err, "sizeMessage:", float64(len(value))/(1024.0*1024.0), "MB")
		}
	} else {
		log.Errorln("Skipping write of message ", err, m)
	}
	return err
}

func NewReaderXElementsBeforeLastMessage(topic int, x int64) *kafka.Reader {

	var offset int64
	o, err := ReadOffset(topic)

	if err == nil && o-x > 0 {
		offset = o - x
	} else {
		log.Warningf("err %v on readOffset on topic %v", err, topic)
	}

	log.Println("NewReaderXElementsBeforeLastMessage: setting offset ", offset, "/", o)

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   KafkaConfig.KafkaUrl,
		Topic:     getTopic(topic),
		Partition: 0,
		MinBytes:  0,
		MaxBytes:  10e6, // 10MB
	})
	err = r.SetOffset(offset)
	if err != nil {
		log.Error(err)
	}
	return r
}

func NewReaderNextMessage(topic int) *kafka.Reader {
	offset := ReadOffsetWithRetryOnError(topic)
	r := NewReader(topic)
	err := r.SetOffset(offset)
	if err != nil {
		log.Error(err)
	}
	log.Printf("Reading from offset %d/%d on topic %s", offset, offset, getTopic(topic))
	return r
}

func IsTopicEmpty(topic int) bool {
	log.Println("IsTopicEmpty: ", topic)
	offset := ReadOffsetWithRetryOnError(topic)
	offset--
	return offset < 0
}
