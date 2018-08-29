package config

import (
	"github.com/go-ini/ini"
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

func GetConfigFromFile(configFile string) (*Config, error) {
	inicfg, err := ini.Load(configFile)
	if err != nil {
		return nil, err
	}

	cfg := Config{
		TestnetName:     inicfg.Section("").Key("TESTNETNAME").String(),
		PrivateKey:      inicfg.Section("").Key("PRIVATEKEY").String(),
		PublicKey:       inicfg.Section("").Key("PUBLICKEY").String(),
		AccountAddress:  inicfg.Section("").Key("ACCOUNTADDRESS").String(),
		Node:            inicfg.Section("").Key("NODE").String(),
		Amount:          inicfg.Section("").Key("AMOUNT").String(),
		Origins:         inicfg.Section("").Key("ORIGINS").Strings(","),
		RedisEndpoint:   inicfg.Section("").Key("REDISENDPOINT").String(),
		RedisPassword:   inicfg.Section("").Key("REDISPASSWORD").String(),
		RecaptchaSecret: inicfg.Section("").Key("RECAPTCHASECRET").String(),
		AWSRegion:       inicfg.Section("").Key("AWSREGION").String(),
	}
	cfg.AccountNumber, err = inicfg.Section("").Key("ACCOUNTNUMBER").Int64()
	if err != nil {
		return nil, err
	}
	cfg.Timeout, err = inicfg.Section("").Key("TIMEOUT").Int64()
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

	timeout, err := strconv.ParseInt(os.Getenv("TIMEOUT"), 10, 64)
	if err != nil {
		return nil, err
	}
	config.Timeout = timeout
	// parse comma-separated list of origins
	config.Origins = strings.Split(os.Getenv("ORIGINS"), ",")
	return &config, nil
}
