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
	"github.com/hashicorp/consul/api"
	"github.com/streadway/amqp"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	version = "1.0"
	app     = kingpin.New("sbus", "A command-line interface to sbus.")
	envName = app.Flag("env", "Environment: qa, stage, live").Default("local").String()

	register       = app.Command("register", "Register a new user.")
	consul         = register.Flag("save-consul", "Save user to consul?").Default("false").Bool()
	publicKeyPath  = register.Flag("public-key-path", "Where the public keys are on consul").Default("services/keys/public/").String()
	identitiesPath = register.Flag("identities-path", "Where the identities are on consul").Default("sbus/rbac/identities/").String()
	groups         = register.Flag("group", "Group for user.").Strings()
	registerName   = register.Arg("name", "Name of user.").Required().String()

	send        = app.Command("Send", "Send a message to the service bus.")
	isEvent     = send.Flag("event", "Is it event?").Default("false").Bool()
	routingKey  = send.Arg("routing-key", "Routing key").Required().String()
	requestBody = send.Arg("request-body", "Request JSON body").Required().String()
)

type ConsulPublicKey struct {
	PublicKey string `json:"publicKey"`
}

func main() {
	kingpin.Version(version)

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	// Register user
	case register.FullCommand():
		pubKey, privKey, _ := ed25519.GenerateKey(nil)

		hexEncodedPublicKEy := hex.EncodeToString(pubKey)

		userKey := "users/" + *registerName

		if *consul {
			ConfigureUserInConsul(hexEncodedPublicKEy, userKey)
		}

		println("add to env SBUS_USER=" + userKey)
		println("add to env SBUS_" + strings.ToUpper(*envName) + "_PRIVATE_KEY=" + hex.EncodeToString(privKey.Seed()))
		println("add to env SBUS_" + strings.ToUpper(*envName) + "_PUBLIC_KEY=" + hexEncodedPublicKEy)
		return

	// Post message
	case send.FullCommand():
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
			"correlation-id": RandString(32),
			"expired-at":     time.Now().Add(time.Minute * 5).UnixMilli(),
		}

		if privateKeyHex, ok := os.LookupEnv("SBUS_" + strings.ToUpper(*envName) + "_PRIVATE_KEY"); ok {
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
}

func ConfigureUserInConsul(hexEncodedPublicKEy string, userKey string) {
	if *envName == "local" {
		log.Printf("Consul cannot be configured for %s", *envName)
		return
	}

	consulUrl, ok := os.LookupEnv("SBUS_CONSUL_" + strings.ToUpper(*envName) + "_URL")
	if !ok {
		log.Panicf("Consul not configured for %s", *envName)
	}

	consulDatacenter, ok := os.LookupEnv("SBUS_CONSUL_" + strings.ToUpper(*envName) + "_DC")
	if !ok {
		log.Panicf("Consul datacenter not configured for %s", *envName)
	}

	config := api.DefaultConfig()
	config.Address = consulUrl
	config.Datacenter = consulDatacenter

	client, _ := api.NewClient(config)

	writeOptions := api.WriteOptions{}

	publicKey := ConsulPublicKey{PublicKey: hexEncodedPublicKEy}

	marshal, err := json.Marshal(publicKey)
	if err != nil {
		log.Panicf("Couldn't serialise publicKey to json: %s", err)
	}

	publicKeyKVPair := api.KVPair{
		Key:   *publicKeyPath + userKey,
		Value: marshal,
	}

	_, err = client.KV().Put(&publicKeyKVPair, &writeOptions)
	if err != nil {
		log.Panicf("Consul Public Key Put: %s", err)
	}

	marshal, err = json.Marshal(*groups)
	if err != nil {
		log.Panicf("Couldn't serialise groups to json: %s", err)
	}

	identityKeyPair := api.KVPair{
		Key:   *identitiesPath + userKey,
		Value: marshal,
	}

	_, err = client.KV().Put(&identityKeyPair, &writeOptions)
	if err != nil {
		log.Panicf("Consul Identity Key Put: %s", err)
	}
}

var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

func RandString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
