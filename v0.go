package main

import (
	"encoding/json"
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
	"fmt"
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
		Name:    ctx.Cfg.TestnetName,
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

func redact(s string) string {
	if len(s) < 2 {
		return "REDACTED"
	}
	return fmt.Sprintf("%sREDACTED%s",string(s[0]),string(s[len(s)-1]))
}

func Initialization(useRDb bool, configFile string) (ctx *context.Context, err error) {

	ctx = context.New()

	if configFile != "" {
		log.Printf("loading from config file %s", configFile)
		ctx.Cfg, err = config.GetConfigFromFile(configFile)
		if err != nil {
			return
		}
	} else {
		log.Printf("loading config from environment variables")
		ctx.Cfg, err = config.GetConfigFromENV()
		if err != nil {
			return
		}
	}


	printCfg := *ctx.Cfg
	printCfg.PrivateKey = redact(printCfg.PrivateKey)
	printCfg.RedisEndpoint = redact(printCfg.RedisEndpoint)
	printCfg.RedisPassword = redact(printCfg.RedisPassword)
	printCfg.RecaptchaSecret = redact(printCfg.RecaptchaSecret)
	log.Printf("%+v", printCfg)

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

	ctx.Mutex = sync.Mutex{
		Name:      ctx.Cfg.TestnetName,
		AWSRegion: ctx.Cfg.AWSRegion,
	}

	ctx.RpcClient = rpcclient.NewHTTP(ctx.Cfg.Node, "/websocket")
	ctx.Cdc = app.MakeCodec()

	recaptcha.Init(ctx.Cfg.RecaptchaSecret)

	return
}
