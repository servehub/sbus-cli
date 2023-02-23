package transports

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/TylerBrock/colorjson"
	"github.com/copperexchange/sbus-cli/internal/config"
	"github.com/copperexchange/sbus-cli/internal/utils"
	"github.com/streadway/amqp"
	"log"
	"math/rand"
	"strconv"
	"time"
)

func SendRabbitMqMessage(requestBody *string, routingKey *string, isEvent *bool, envName *string, envData *config.AppConfig) bool {
	amqpUrl, ok := envData.GetValue(envName, config.EnvSbusAmqpUrl)
	if !ok {
		amqpUrl = "amqp://guest:guest@localhost:5672/"
	}

	connection, err := amqp.Dial(amqpUrl)
	if err != nil {
		log.Panicf("Dial: %s", err)
		return false
	}
	defer connection.Close()

	channel, err := connection.Channel()
	if err != nil {
		log.Panicf("Channel: %s", err)
		return false
	}

	replyQueue, err := channel.QueueDeclare(
		"",    // name of the queue
		false, // durable
		true,  // delete when unused
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		log.Panicf("Queue Declare: %s", err)
		return false
	}

	deliveries, err := channel.Consume(
		replyQueue.Name, // name
		"",              // consumerTag,
		false,           // noAck
		false,           // exclusive
		false,           // noLocal
		false,           // noWait
		nil,             // arguments
	)
	if err != nil {
		log.Panicf("Queue Consume: %s", err)
		return false
	}

	exchange := "sbus.common"
	replyTo := replyQueue.Name

	if *isEvent {
		exchange = "sbus.events"
		replyTo = ""
	}

	now := time.Now()

	rand.Seed(now.UnixNano())

	payload := []byte(`{"body":` + *requestBody + `}`)

	corrId := utils.RandString(32)

	headers := amqp.Table{
		"correlation-id": corrId,
		"expired-at":     now.Add(time.Minute * 5).UnixMilli(),
		"timestamp":      now.UnixMilli(),
	}

	if privateKeyHex, ok := envData.GetValue(envName, config.EnvSbusPrivateKey); ok {
		if privateKey, err := hex.DecodeString(privateKeyHex); err == nil {
			pvk := ed25519.NewKeyFromSeed(privateKey)
			routingKeyB := []byte(*routingKey)

			if user, ok := envData.GetValue(envName, config.EnvSbusUser); ok {
				headers["origin"] = user

				cmdSigB := ed25519.Sign(pvk, append(append(payload, routingKeyB...), []byte(user)...))
				cmdSigS := base64.URLEncoding.EncodeToString(cmdSigB)

				headers["signature"] = cmdSigS
			}
		}
	}

	if err = channel.Publish(
		exchange,    // publish to an exchange
		*routingKey, // routing to 0 or more queues
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			Headers:      headers,
			Body:         payload,
			DeliveryMode: amqp.Transient, // 1=non-persistent, 2=persistent
			Priority:     0,              // 0-9
			ReplyTo:      replyTo,
		},
	); err != nil {
		log.Panicf("Exchange Publish: %s", err)
		return false
	}

	if !*isEvent {
		select {
		case payload := <-deliveries:
			var response map[string]interface{}
			json.Unmarshal(payload.Body, &response)

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
		case <-time.After(60 * time.Second):
			log.Panicf("No response in 60 seconds")
			return false

		}
	}

	return true
}
