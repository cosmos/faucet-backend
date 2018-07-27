package main

import (
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/cosmos/cosmos-sdk/cmd/gaia/app"
	"github.com/cosmos/faucet-backend/config"
	"github.com/cosmos/faucet-backend/context"
	"github.com/cosmos/faucet-backend/defaults"
	"github.com/dpapathanasiou/go-recaptcha"
	"github.com/gorilla/mux"
	"github.com/greg-szabo/dsync/ddb/sync"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"log"
	"net/http"
	"os"
)

func MainHandler(ctx *context.Context, w http.ResponseWriter, r *http.Request) (status int, err error) {
	status = http.StatusOK
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(struct {
		Message string `json:"message"`
		Name    string `json:"name"`
		Version string `json:"version"`
	}{
		Message: "",
		Name:    defaults.TestnetName,
		Version: defaults.Version,
	})
	return
}

func AddRoutes(ctx *context.Context) (r *mux.Router) {

	// Root and routes
	r = mux.NewRouter()
	r.Handle("/", context.Handler{ctx, MainHandler})
	r.Handle("/v1/claim", context.Handler{ctx, V1ClaimHandler}).Methods("POST", "OPTIONS")

	// Finally
	r.Use(loggingMiddleware)
	r.Use(createCORSMiddleware(ctx))
	if !ctx.DisableLimiter {
		r.Use(createThrottledMiddleware(ctx))
	}
	http.Handle("/", r)

	return
}

func Initialization(useDDb bool, useRDb bool, configFile string) (ctx *context.Context, err error) {

	ctx = context.New()
	ctx.Mutex = sync.Mutex{
		Name:      "f11",
		AWSRegion: defaults.AWSRegion,
	}

	if useDDb {
		log.Printf("loading config from %s table in DynamoDB", defaults.DynamoDBTable)

		awsCfg := aws.Config{
			Region: aws.String(defaults.AWSRegion),
		}

		// Use IAM or environment variables credential
		if (os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "") ||
			(os.Getenv("AWS_ACCESS_KEY") != "" && os.Getenv("AWS_SECRET_KEY") != "") {
			awsCfg.Credentials = credentials.NewEnvCredentials()
		}

		ctx.AwsSession = session.Must(session.NewSessionWithOptions(session.Options{Config: awsCfg}))
		ctx.DbSession = dynamodb.New(ctx.AwsSession)
		ctx.Cfg, err = config.GetConfigFromENV()
		if err != nil {
			return
		}

	} else {
		log.Printf("loading config from %s file", configFile)
		ctx.Cfg, err = config.GetConfigFromFile(configFile)
		if err != nil {
			return
		}
	}

	if useRDb {
		ctx.Store, err = createRedisStore(ctx)
		if err != nil {
			return
		}
	} else {
		ctx.Store, err = createMemStore()
		if err != nil {
			return
		}

	}

	log.Printf("config for %s loaded", ctx.Cfg.TestnetName)

	err = createThrottledLimiter(ctx)
	if err != nil {
		return
	}

	ctx.RpcClient = rpcclient.NewHTTP(ctx.Cfg.Node, "/websocket")
	ctx.Cdc = app.MakeCodec()

	recaptcha.Init(ctx.Cfg.RecaptchaSecret)

	return
}
