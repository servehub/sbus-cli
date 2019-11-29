package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/TylerBrock/colorjson"
	"github.com/streadway/amqp"
	"gopkg.in/alecthomas/kingpin.v2"
)

var version = "1.0"

func main() {
	routingKey := kingpin.Arg("routing-key", "Routing key").Required().String()
	requestBody := kingpin.Arg("request-body", "Request JSON body").Required().String()
	envName := kingpin.Flag("env", "Environment: qa, stage, live").Default("qa").String()

	kingpin.Version(version)
	kingpin.Parse()

	amqpUrl, ok := os.LookupEnv("SBUS_AMQP_"+ strings.ToUpper(*envName) +"_URL")
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

	queue, err := channel.QueueDeclare(
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
		queue.Name, // name
		"sbus-cli", // consumerTag,
		false,      // noAck
		false,      // exclusive
		false,      // noLocal
		false,      // noWait
		nil,        // arguments
	)
	if err != nil {
		log.Panicf("Queue Consume: %s", err)
	}

	if err = channel.Publish(
		"sbus.common", // publish to an exchange
		*routingKey,   // routing to 0 or more queues
		false,         // mandatory
		false,         // immediate
		amqp.Publishing{
			Headers:      amqp.Table{},
			Body:         []byte(`{"body":` + *requestBody + `}`),
			DeliveryMode: amqp.Transient, // 1=non-persistent, 2=persistent
			Priority:     0,              // 0-9
			ReplyTo:      queue.Name,
		},
	); err != nil {
		log.Panicf("Exchange Publish: %s", err)
	}

	for d := range deliveries {
		var response map[string]interface{}
		json.Unmarshal(d.Body, &response)

		f := colorjson.NewFormatter()
		f.Indent = 2

		jsonStr, err := f.Marshal(response["body"])
		if err != nil {
			log.Panicf("Error parse response: %s", err)
		}

		fmt.Printf("\n %s \n\n %s \n\n", response["status"], jsonStr)

		d.Ack(false)
		os.Exit(0)
	}
}
