package transports

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/TylerBrock/colorjson"
	"github.com/copperexchange/sbus-cli/internal/config"
	"github.com/copperexchange/sbus-cli/internal/utils"
	"github.com/hashicorp/go-uuid"
	kafka "github.com/segmentio/kafka-go"
	"io"
	"log"
	"math/rand"
	"strconv"
	"time"
)

func SendKafkaMessage(requestBody *string, routingKey *string, isEvent *bool, envName *string, envData *config.AppConfig) bool {
	kafkaUrl, ok := envData.GetValue(envName, config.EnvKafkaUrl)
	if !ok {
		kafkaUrl = "localhost"
	}

	w := &kafka.Writer{
		Addr:                   kafka.TCP(kafkaUrl),
		AllowAutoTopicCreation: true,
	}

	now := time.Now()

	rand.Seed(now.UnixNano())

	uuid, err := uuid.GenerateUUID()
	if err != nil {
		log.Panicf("Producer: %s", err)
		return false
	}

	client := &kafka.Client{
		Addr:    kafka.TCP(kafkaUrl),
		Timeout: 10 * time.Second,
	}

	responseTopic := "cli." + uuid

	log.Printf("Response Topic %s", responseTopic)

	if !*isEvent {
		_, err := client.CreateTopics(context.Background(), &kafka.CreateTopicsRequest{
			Addr: kafka.TCP(kafkaUrl),
			Topics: []kafka.TopicConfig{
				{Topic: responseTopic, NumPartitions: 1, ReplicationFactor: 1},
			},
		})

		if err != nil {
			log.Panicf("Admin: %s", err)
			return false
		}

		defer client.DeleteTopics(context.Background(), &kafka.DeleteTopicsRequest{
			Addr:   kafka.TCP(kafkaUrl),
			Topics: []string{responseTopic},
		})
	}

	payload := []byte(`{"body":` + *requestBody + `}`)

	corrId := utils.RandString(32)

	headers := []kafka.Header{
		{Key: "routing-key", Value: []byte(*routingKey)},
		{Key: "correlation-id", Value: []byte(corrId)},
		{Key: "expired-at", Value: []byte(strconv.FormatInt(now.Add(time.Minute*5).UnixMilli(), 10))},
		{Key: "timestamp", Value: []byte(strconv.FormatInt(now.UnixMilli(), 10))},
	}

	if !*isEvent {
		headers = append(headers, kafka.Header{Key: "reply-to", Value: []byte(responseTopic)})
	}

	if privateKeyHex, ok := envData.GetValue(envName, config.EnvSbusPrivateKey); ok {
		if privateKey, err := hex.DecodeString(privateKeyHex); err == nil {
			pvk := ed25519.NewKeyFromSeed(privateKey)
			routingKeyB := []byte(*routingKey)

			if user, ok := envData.GetValue(envName, config.EnvSbusUser); ok {
				headers = append(headers, kafka.Header{Key: "origin", Value: []byte(user)})

				cmdSigB := ed25519.Sign(pvk, append(append(payload, routingKeyB...), []byte(user)...))
				cmdSigS := base64.URLEncoding.EncodeToString(cmdSigB)

				headers = append(headers, kafka.Header{Key: "signature", Value: []byte(cmdSigS)})
			}
		}
	}

	// Produce messages to topic (asynchronously)
	err = w.WriteMessages(context.Background(), kafka.Message{
		Topic:   *routingKey,
		Key:     []byte(utils.RandString(32)),
		Value:   payload,
		Headers: headers,
	})
	if err != nil {
		log.Panicf("Writer: %s", err)
		return false
	}

	w.Close()

	if !*isEvent {
		channel := make(chan kafka.Message)
		error := make(chan bool)
		go func() {
			r := kafka.NewReader(kafka.ReaderConfig{
				Brokers: []string{kafkaUrl},
				Topic:   responseTopic,
			})

			if err != nil {
				log.Panicf("Consumer: %s", err)
				error <- true
				return
			}

			run := true
			for run {
				msg, err := r.ReadMessage(context.Background())
				if err != nil {
					if err != io.EOF {
						log.Panicf("Consumer error: %s", err)
					}
					error <- true
					run = false
				} else {
					channel <- msg
					run = false
				}
			}

			r.Close()
		}()
		select {
		case payload := <-channel:
			var response map[string]interface{}
			json.Unmarshal(payload.Value, &response)

			f := colorjson.NewFormatter()
			f.Indent = 2

			jsonStr, err := f.Marshal(response["body"])
			if err != nil {
				log.Panicf("Error parse response: %s", err)
				return false
			}

			fmt.Printf("\n%s\n\n%s\n\n", response["status"], jsonStr)

			if status, err2 := strconv.Atoi(fmt.Sprintf("%s", response["status"])); err2 != nil || status >= 400 {
				return false
			} else {
				return true
			}
		case _ = <-error:
			return false
		case <-time.After(60 * time.Second):
			log.Panicf("No response in 60 seconds")
			return false

		}
	}

	return true
}
