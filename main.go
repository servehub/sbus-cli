package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"github.com/copperexchange/sbus-cli/internal/consul"
	"os"
	"strings"

	"github.com/copperexchange/sbus-cli/internal/config"
	"github.com/copperexchange/sbus-cli/internal/transports"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	version = "1.0"
	app     = kingpin.New("sbus", "A command-line interface to sbus.").Version(version)
	envName = app.Flag("env", "Environment: qa, stage, live").Default("local").String()

	register       = app.Command("register", "Register a new user.")
	registerName   = register.Arg("name", "Name of user.").Required().String()
	groups         = register.Flag("group", "Group for user.").Strings()
	consulUrl      = register.Flag("save-to-consul", "Save user to consul?").Default("").String()
	registerPKPath = register.Flag("public-key-path", "Where the public keys are on consul").Default("services/keys/public/").String()
	identitiesPath = register.Flag("identities-path", "Where the identities are on consul").Default("services/sbus/identities/").String()

	send        = app.Command("send", "Send a message to the service bus.").Default()
	routingKey  = send.Arg("routing-key", "Routing key").Required().String()
	requestBody = send.Arg("request-body", "Request JSON body").Required().String()
	isEvent     = send.Flag("event", "Is it event?").Default("false").Bool()
	transport   = send.Flag("transport", "Transport: rabbitmq, kafka, all").Default("rabbitmq").String()

	fetchConfig = app.Command("config", "Read amqp url key from AWS Secret Manager and save to config file.")
	smAmqpKey   = fetchConfig.Flag("sm-amqp-key", "Exact Amqp AWS Secret Manager key name.").Default("").String()
	smKafkaKey  = fetchConfig.Flag("sm-kafka-key", "Exact Kafka AWS Secret Manager key name.").Default("").String()
)

func main() {
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	// Register user
	case register.FullCommand():
		registerUser()
		return

	// Post message
	case send.FullCommand():
		envData, _ := config.LoadConfigWithOverrides(envName)
		sendMessage(envData)
		return

	case fetchConfig.FullCommand():
		envData, _ := config.LoadConfigNoOverrides(envName)
		amqpUrl, kafkaUrl, err := readConfigFromAWS()
		if err != nil {
			fmt.Println("Error:", err.Error()) //TODO
		} else {
			envData.SetValueAmpqUrl(envName, amqpUrl)
			envData.SetValueKafkaUrl(envName, kafkaUrl)
			envData.SaveConfiguration()
		}
		return
	}
}

func readConfigFromAWS() (*string, *string, error) {
	if len(*envName) == 0 {
		return nil, nil, fmt.Errorf("--env flag must be provided")
	}

	ampqUrl, err := config.FetchDataFromAWSSM(smAmqpKey, envName, config.AmqpKeySuffix)
	if err != nil {
		return nil, nil, err
	}
	kafkaUrl, err := config.FetchDataFromAWSSM(smKafkaKey, envName, config.KafkaKeySuffix)
	if err != nil {
		return nil, nil, err
	}
	return ampqUrl, kafkaUrl, nil
}

func registerUser() {
	pubKey, privKey, _ := ed25519.GenerateKey(nil)

	hexEncodedPublicKey := hex.EncodeToString(pubKey)

	if *consulUrl != "" {
		consul.ConfigureUserInConsul(hexEncodedPublicKey, registerName, envName, registerPKPath, groups, identitiesPath, consulUrl)
	}

	println("export SBUS_USER='" + *registerName + "'")
	println("export SBUS_" + strings.ToUpper(*envName) + "_PRIVATE_KEY=" + hex.EncodeToString(privKey.Seed()))
	println("export SBUS_" + strings.ToUpper(*envName) + "_PUBLIC_KEY=" + hexEncodedPublicKey)
}

func sendMessage(envData *config.AppConfig) {
	switch *transport {
	case "rabbitmq":
		sentToRabbit := transports.SendRabbitMqMessage(requestBody, routingKey, isEvent, envName, envData)
		if !sentToRabbit {
			os.Exit(2)
		}
		return
	case "kafka":
		sentToKafka := transports.SendKafkaMessage(requestBody, routingKey, isEvent, envName, envData)
		if !sentToKafka {
			os.Exit(2)
		}
		return

	case "all":
		if ok := transports.SendRabbitMqMessage(requestBody, routingKey, isEvent, envName, envData); !ok {
			if ok = transports.SendKafkaMessage(requestBody, routingKey, isEvent, envName, envData); !ok {
				os.Exit(2)
			}
		}

		return
	}
}
