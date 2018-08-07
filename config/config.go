package config

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	TestnetName     string   `json:"TESTNETNAME"`
	PrivateKey      string   `json:"PRIVATEKEY"`
	PublicKey       string   `json:"PUBLICKEY"`
	AccountAddress  string   `json:"ACCOUNTADDRESS"`
	AccountNumber   int64    `json:"ACCOUNTNUMBER"`
	Node            string   `json:"NODE"`
	Amount          string   `json:"AMOUNT"`
	Origins         []string `json:"ORIGINS"`
	RedisEndpoint   string   `json:"REDISENDPOINT"`
	RedisPassword   string   `json:"REDISPASSWORD"`
	RecaptchaSecret string   `json:"RECAPTCHASECRET"`
	AWSRegion       string   `json:"AWSREGION"`
	Timeout         int64    `json:"TIMEOUT"`
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

func GetConfigFromENV() (*Config, error) {
	config := Config{
		TestnetName:     os.Getenv("TESTNETNAME"),
		PrivateKey:      os.Getenv("PRIVATEKEY"),
		PublicKey:       os.Getenv("PUBLICKEY"),
		AccountAddress:  os.Getenv("ACCOUNTADDRESS"),
		Node:            os.Getenv("NODE"),
		Amount:          os.Getenv("AMOUNT"),
		RedisEndpoint:   os.Getenv("REDISENDPOINT"),
		RedisPassword:   os.Getenv("REDISPASSWORD"),
		RecaptchaSecret: os.Getenv("RECAPTCHASECRET"),
		AWSRegion:       os.Getenv("AWSREGION"),
	}
	accnum, err := strconv.ParseInt(os.Getenv("ACCOUNTNUMBER"), 10, 64)
	if err != nil {
		return nil, err
	}
	config.AccountNumber = accnum

	timeout, err := strconv.ParseInt("TIMEOUT", 10, 64)
	if err != nil {
		return nil, err
	}
	config.Timeout = timeout
	// parse comma-seperated list of origins
	config.Origins = strings.Split(os.Getenv("ORIGINS"), ",")
	return &config, nil
}
