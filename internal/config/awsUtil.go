package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

var (
	awsDefaultRegion = "AWS_DEFAULT_REGION"
)

const (
	AmqpKeySuffix  string = "amq_url"
	KafkaKeySuffix string = "kafka_url"
)

/*
Connect to AWS Secret Manager and return secret value. If --sm-key is provide it will return exact secret value, if not it will return secret with name contains env and ends with "amq_url"
*/
func FetchDataFromAWSSM(smKey *string, env *string, suffix string) (*string, error) {
	region, ok := os.LookupEnv(awsDefaultRegion)
	if ok == false {
		return nil, fmt.Errorf("AWS_DEFAULT_REGION env variable must be provided")
	}
	if len(*env) == 0 || *env == "local" {
		return nil, fmt.Errorf("--env flag must be provided and != from local")
	}

	sess := session.Must(session.NewSession())

	svc := secretsmanager.New(sess, aws.NewConfig().WithRegion(region))

	if *smKey != "" {
		//we know exact key
		return getSecretValue(svc, smKey)
	} else {
		inputFilter := &secretsmanager.ListSecretsInput{}
		count := 0
		key := ""

		for {
			resultList, err := svc.ListSecrets(inputFilter)
			if err != nil {
				return nil, fmt.Errorf("Problems during fetching secrets from AWS Secret Manager")
			}

			for _, secret := range resultList.SecretList {
				if strings.Contains(*secret.Name, *env) && strings.HasSuffix(*secret.Name, suffix) {
					key = *secret.Name
					count++
				}
			}

			if resultList.NextToken == nil {
				break
			} else {
				inputFilter.NextToken = resultList.NextToken
			}
		}
		if count > 1 {
			return nil, fmt.Errorf("to many keys, can't load configuration from AWS Secret Manager")
		}
		if count < 1 {
			return nil, fmt.Errorf("key not found on AWS Secret Manager")
		}
		return getSecretValue(svc, &key)
	}
}

func getSecretValue(svc *secretsmanager.SecretsManager, key *string) (*string, error) {
	keyResult, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{SecretId: key})
	if err != nil {
		return nil, err
	}
	return keyResult.SecretString, nil
}
