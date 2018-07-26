package config

import (
	"encoding/base64"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/greg-szabo/f11/defaults"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
)

type Config struct {
	TestnetName     string   `json:"testnet-name"`
	PrivateKey      string   `json:"private-key"`
	PublicKey       string   `json:"public-key"`
	AccountAddress  string   `json:"account-address"`
	Sequence        int64    `json:"sequence"`
	AccountNumber   int64    `json:"account-number"`
	Node            string   `json:"node"`
	Amount          string   `json:"amount"`
	Origins         []string `json:"origins"`
	RedisEndpoint   string   `json:"redis-endpoint"`
	RedisPassword   string   `json:"redis-password"`
	RecaptchaSecret string   `json:"recaptcha-secret"`
}

func GetPrivkeyBytesFromString(privkeystring string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(privkeystring)
}

func GetConfigFromFile(configFile string) (*Config, error) {
	jsonFile, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	cfg := Config{}
	err = json.Unmarshal(byteValue, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func GetConfigFromDB(dbSession *dynamodb.DynamoDB) (*Config, error) {
	dbOutput, err := dbSession.Query(&dynamodb.QueryInput{
		TableName:              aws.String(defaults.DynamoDBTable),
		KeyConditionExpression: aws.String("#testnetname=:testnetname AND #testnetinstance=:testnetinstance"),
		ExpressionAttributeNames: map[string]*string{
			"#testnetname":     aws.String(defaults.DynamoDBTestnetNameColumn),
			"#testnetinstance": aws.String(defaults.DynamoDBTestnetInstanceColumn),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":testnetname": {
				S: aws.String(defaults.TestnetName),
			},
			":testnetinstance": {
				S: aws.String(defaults.TestnetInstance),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if *dbOutput.Count == 0 {
		return nil, errors.New("no configuration found")
	}
	if *dbOutput.Count > 1 {
		return nil, errors.New("ambivalent configuration found")
	}
	cfg := Config{}
	err = dynamodbattribute.UnmarshalMap(dbOutput.Items[0], &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
