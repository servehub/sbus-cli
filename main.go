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
	letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	version = "1.0"
	app     = kingpin.New("sbus", "A command-line interface to sbus.").Version(version)
	envName = app.Flag("env", "Environment: qa, stage, live").Default("local").String()

	register       = app.Command("register", "Register a new user.")
	registerName   = register.Arg("name", "Name of user.").Required().String()
	groups         = register.Flag("group", "Group for user.").Strings()
	consulUrl      = register.Flag("save-to-consul", "Save user to consul?").Default("").String()
	registerPKPath = register.Flag("public-key-path", "Where the public keys are on consul").Default("services/keys/public/").String()
	identitiesPath = register.Flag("identities-path", "Where the identities are on consul").Default("services/sbus/identities/").String()

	send              = app.Command("send", "Send a message to the service bus.").Default()
	routingKey        = send.Arg("routing-key", "Routing key").Required().String()
	requestBody       = send.Arg("request-body", "Request JSON body/JSON file").Required().String()
	readyBodyFromFile = send.Flag("file", "request-body parameter is a file").Default("false").Bool()
	noStatusCode      = send.Flag("no-status", "Exclude status code from response.").Default("false").Bool()
	isEvent           = send.Flag("event", "Is it event?").Default("false").Bool()
)

type ConsulPublicKey struct {
	PublicKey string `json:"publicKey"`
}

type Identity struct {
	Groups []string `json:"groups"`
}

func main() {
	kingpin.Version(version)

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	// Register user
	case register.FullCommand():
		registerUser()
		return

	// Post message
	case send.FullCommand():
		sendMessage()
		return
	}
}

func registerUser() {
	pubKey, privKey, _ := ed25519.GenerateKey(nil)

	hexEncodedPublicKey := hex.EncodeToString(pubKey)

	if *consulUrl != "" {
		configureUserInConsul(hexEncodedPublicKey, *registerName)
	}

	println("export SBUS_USER='" + *registerName + "'")
	println("export SBUS_" + strings.ToUpper(*envName) + "_PRIVATE_KEY=" + hex.EncodeToString(privKey.Seed()))
	println("export SBUS_" + strings.ToUpper(*envName) + "_PUBLIC_KEY=" + hexEncodedPublicKey)
}

func sendMessage() {
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

	now := time.Now()

	rand.Seed(now.UnixNano())

	var payload []byte

	if *readyBodyFromFile {
		data, err := os.ReadFile(*requestBody)
		if err != nil {
			log.Panicf("File cannot be read. file=%s, error=%s", *requestBody, err)
			os.Exit(1)
		}
		requestBodyStr := string(data)
		payload = []byte(`{"body":` + requestBodyStr + `}`)
	} else {
		payload = []byte(`{"body":` + *requestBody + `}`)
	}

	corrId := randString(32)

	headers := amqp.Table{
		"correlation-id": corrId,
		"expired-at":     now.Add(time.Minute * 5).UnixMilli(),
		"timestamp":      now.UnixMilli(),
	}

	if privateKeyHex, ok := os.LookupEnv("SBUS_" + strings.ToUpper(*envName) + "_PRIVATE_KEY"); ok {
		if privateKey, err := hex.DecodeString(privateKeyHex); err == nil {
			pvk := ed25519.NewKeyFromSeed(privateKey)
			routingKeyB := []byte(*routingKey)

			if user, ok := os.LookupEnv("SBUS_USER"); ok {
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

			if *noStatusCode {
				fmt.Printf("%s", jsonStr)
			} else {
				fmt.Printf("\n%s\n\n%s\n\n", response["status"], jsonStr)
			}

			d.Ack(false)

			if status, err2 := strconv.Atoi(fmt.Sprintf("%s", response["status"])); err2 != nil || status >= 400 {
				os.Exit(2)
			} else {
				os.Exit(0)
			}
		}
	}
}

func configureUserInConsul(hexEncodedPublicKey string, userKey string) {
	if *envName == "local" {
		log.Printf("Consul cannot be configured for %s", *envName)
		return
	}

	client := newConsulClient()

	writeOptions := api.WriteOptions{}

	publicKey := ConsulPublicKey{PublicKey: hexEncodedPublicKey}

	marshal, err := json.Marshal(publicKey)
	if err != nil {
		log.Panicf("Couldn't serialise publicKey to json: %s", err)
	}

	userPublicKeyKey := *registerPKPath + userKey

	publicKeyKVPair := api.KVPair{
		Key:   userPublicKeyKey,
		Value: marshal,
	}

	_, err = client.KV().Put(&publicKeyKVPair, &writeOptions)
	if err != nil {
		log.Panicf("Consul Public Key Put: %s", err)
	}

	identity := Identity{Groups: *groups}

	marshal, err = json.Marshal(identity)
	if err != nil {
		log.Panicf("Couldn't serialise identity to json: %s", err)
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

func newConsulClient() *api.Client {
	config := api.DefaultConfig()
	config.Address = *consulUrl

	client, _ := api.NewClient(config)
	return client
}

func randString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
