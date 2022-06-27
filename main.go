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
	consul         = register.Flag("save-consul", "Save user to consul?").Default("false").Bool()
	registerPKPath = register.Flag("public-key-path", "Where the public keys are on consul").Default("services/keys/public/").String()
	identitiesPath = register.Flag("identities-path", "Where the identities are on consul").Default("sbus/rbac/identities/").String()
	groups         = register.Flag("group", "Group for user.").Strings()
	registerName   = register.Arg("name", "Name of user.").Required().String()

	send        = app.Command("send", "Send a message to the service bus.")
	isEvent     = send.Flag("event", "Is it event?").Default("false").Bool()
	routingKey  = send.Arg("routing-key", "Routing key").Required().String()
	requestBody = send.Arg("request-body", "Request JSON body").Required().String()

	verify       = app.Command("verify", "Verify a message signature.")
	verifyPKPath = verify.Flag("public-key-path", "Where the public keys are on consul").Default("services/keys/public/").String()
	signature    = verify.Arg("signature", "Signature").Required().String()
	caller       = verify.Arg("caller", "Who sent the message").Required().String()
	body         = verify.Arg("body", "Message body").Required().String()
	timestamp    = verify.Arg("timestamp", "Timestamp").String()
)

type ConsulPublicKey struct {
	PublicKey string `json:"publicKey"`
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

	case verify.FullCommand():
		verifySignature()
		return
	}
}

func verifySignature() {
	client := newConsulClient()

	callerPublicKeyKey := *verifyPKPath + *caller

	queryOptions := api.QueryOptions{}

	get, _, err := client.KV().Get(callerPublicKeyKey, &queryOptions)
	if err != nil {
		log.Panicf("Public Key: %s", err)
	}

	var pkc ConsulPublicKey
	err = json.Unmarshal(get.Value, &pkc)
	if err != nil {
		log.Panicf("Public Key: %s", err)
	}

	pk, err := hex.DecodeString(pkc.PublicKey)
	if err != nil {
		log.Panicf("Public Key: %s", err)
	}

	sigb, err := base64.URLEncoding.DecodeString(*signature)
	if err != nil {
		log.Panicf("Signature Key: %s", err)
	}

	payload := append([]byte(*body), []byte(*timestamp)...)

	verified := ed25519.Verify(pk, payload, sigb)

	println("Verified?: ", verified)
}

func registerUser() {
	pubKey, privKey, _ := ed25519.GenerateKey(nil)

	hexEncodedPublicKey := hex.EncodeToString(pubKey)

	if *consul {
		configureUserInConsul(hexEncodedPublicKey, *registerName)
	}

	println("add to env SBUS_USER=" + *registerName)
	println("add to env SBUS_" + strings.ToUpper(*envName) + "_PRIVATE_KEY=" + hex.EncodeToString(privKey.Seed()))
	println("add to env SBUS_" + strings.ToUpper(*envName) + "_PUBLIC_KEY=" + hexEncodedPublicKey)
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

	payload := []byte(`{"body":` + *requestBody + `}`)

	headers := amqp.Table{
		"correlation-id": randString(32),
		"expired-at":     now.Add(time.Minute * 5).UnixMilli(),
		"timestamp":      now.UnixMilli(),
	}

	if privateKeyHex, ok := os.LookupEnv("SBUS_" + strings.ToUpper(*envName) + "_PRIVATE_KEY"); ok {
		if privateKey, err := hex.DecodeString(privateKeyHex); err == nil {
			pvk := ed25519.NewKeyFromSeed(privateKey)
			timestampS := strconv.FormatInt(now.UnixMilli(), 10)
			timestampB := []byte(timestampS)
			sigb := ed25519.Sign(pvk, append(payload, timestampB...))
			sigs := base64.URLEncoding.EncodeToString(sigb)

			if user, ok := os.LookupEnv("SBUS_USER"); ok {
				headers["origin"] = user
			}

			headers["signature"] = sigs
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

func newConsulClient() *api.Client {
	consulUrl, ok := os.LookupEnv("SBUS_CONSUL_" + strings.ToUpper(*envName) + "_URL")
	if !ok {
		log.Panicf("Consul not configured for %s", *envName)
	}

	consulDatacenter, ok := os.LookupEnv("SBUS_CONSUL_" + strings.ToUpper(*envName) + "_DC")
	if !ok {
		consulDatacenter = "dc1"
	}

	config := api.DefaultConfig()
	config.Address = consulUrl
	config.Datacenter = consulDatacenter

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
