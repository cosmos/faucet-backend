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
	}
	accnum, err := strconv.ParseInt(os.Getenv("ACCOUNTNUMBER"), 10, 64)
	if err != nil {
		return nil, err
	}
	config.AccountNumber = accnum
	// parse comma-seperated list of origins
	config.Origins = strings.Split(os.Getenv("ORIGINS"), ",")
	return &config, nil
}
