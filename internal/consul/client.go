package consul

import (
	"encoding/json"
	"github.com/hashicorp/consul/api"
	"log"
	"time"
)

type ConsulPublicKey struct {
	PublicKey string `json:"publicKey"`
	CreatedAt int64  `json:"createdAt"`
}

type Identity struct {
	Groups    []string `json:"groups"`
	CreatedAt int64    `json:"createdAt"`
}

func ConfigureUserInConsul(hexEncodedPublicKey string, userKey *string, envName *string, registerPKPath *string, groups *[]string, identitiesPath *string, consulUrl *string) {
	if *envName == "local" {
		log.Printf("Consul cannot be configured for %s", *envName)
		return
	}

	client := newConsulClient(consulUrl)

	writeOptions := api.WriteOptions{}

	publicKey := ConsulPublicKey{
		PublicKey: hexEncodedPublicKey,
		CreatedAt: time.Now().UnixMilli(),
	}

	marshal, err := json.Marshal(publicKey)
	if err != nil {
		log.Panicf("Couldn't serialise publicKey to json: %s", err)
	}

	userPublicKeyKey := *registerPKPath + *userKey

	publicKeyKVPair := api.KVPair{
		Key:   userPublicKeyKey,
		Value: marshal,
	}

	_, err = client.KV().Put(&publicKeyKVPair, &writeOptions)
	if err != nil {
		log.Panicf("Consul Public Key Put: %s", err)
	}

	identity := Identity{
		Groups:    *groups,
		CreatedAt: time.Now().UnixMilli(),
	}

	marshal, err = json.Marshal(identity)
	if err != nil {
		log.Panicf("Couldn't serialise identity to json: %s", err)
	}

	identityKeyPair := api.KVPair{
		Key:   *identitiesPath + *userKey,
		Value: marshal,
	}

	_, err = client.KV().Put(&identityKeyPair, &writeOptions)
	if err != nil {
		log.Panicf("Consul Identity Key Put: %s", err)
	}
}

func newConsulClient(consulUrl *string) *api.Client {
	config := api.DefaultConfig()
	config.Address = *consulUrl

	client, _ := api.NewClient(config)

	return client
}
