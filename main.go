package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"

	"github.com/TylerBrock/colorjson"
	"github.com/streadway/amqp"
	"gopkg.in/alecthomas/kingpin.v2"
)

var version = "1.0"

func main() {
	routingKey := kingpin.Arg("routing-key", "Routing key").Required().String()
	requestBody := kingpin.Arg("request-body", "Request JSON body").Required().String()
	envName := kingpin.Flag("env", "Environment: qa, stage, live").Default("local").String()
	isEvent := kingpin.Flag("event", "Is it event?").Default("false").Bool()

	kingpin.Version(version)
	kingpin.Parse()

	if *routingKey == "new-user" {
		pubKey, privKey, _ := ed25519.GenerateKey(nil)

		println(*requestBody)
		println("pub:", hex.EncodeToString(pubKey))
		println("priv:", hex.EncodeToString(privKey.Seed()))
		return
	}

	amqpUrl, ok := os.LookupEnv("SBUS_AMQP_" + strings.ToUpper(*envName) + "_URL")
	if !ok {
		amqpUrl = "amqp://guest:guest@localhost:5672/"
	}

	connection, err := amqp.Dial(amqpUrl)
	if err != nil {
		log.Panicf("Dial: %s", err)
	}
	defer connection.Close()

	channel, err := connection.Channel()
	if err != nil {
		log.Panicf("Channel: %s", err)
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
	}

	exchange := "sbus.common"
	replyTo := replyQueue.Name

	if *isEvent {
		exchange = "sbus.events"
		replyTo = ""
	}

	rand.Seed(time.Now().UnixNano())

	payload := []byte(`{"body":` + *requestBody + `}`)

	headers := amqp.Table{
		"correlation-id": randString(32),
	}

	if privateKeyHex, ok := os.LookupEnv("SBUS_PRIVATE_KEY"); ok {
		if privateKey, err := hex.DecodeString(privateKeyHex); err == nil {
			pvk := ed25519.NewKeyFromSeed(privateKey)
			sigb := ed25519.Sign(pvk, payload)

			if user, ok := os.LookupEnv("SBUS_USER"); ok {
				headers["origin"] = user
			}

			headers["signature"] = base64.URLEncoding.EncodeToString(sigb)
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
	}

	if !*isEvent {
		for d := range deliveries {
			var response map[string]interface{}
			json.Unmarshal(d.Body, &response)

			f := colorjson.NewFormatter()
			f.Indent = 2

			jsonStr, err := f.Marshal(response["body"])
			if err != nil {
				log.Panicf("Error parse response: %s", err)
			}

			fmt.Printf("\n%s\n\n%s\n\n", response["status"], jsonStr)

			d.Ack(false)

			if status, err2 := strconv.Atoi(fmt.Sprintf("%s", response["status"])); err2 != nil || status >= 400 {
				os.Exit(2)
			} else {
				os.Exit(0)
			}
		}
	}
}

var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

func randString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
