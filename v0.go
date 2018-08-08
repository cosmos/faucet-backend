package main

import (
	"encoding/json"
	"fmt"
	sdkCtx "github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/cmd/gaia/app"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	"github.com/cosmos/faucet-backend/config"
	"github.com/cosmos/faucet-backend/context"
	"github.com/cosmos/faucet-backend/defaults"
	"github.com/dpapathanasiou/go-recaptcha"
	"github.com/gorilla/mux"
	"github.com/greg-szabo/dsync/ddb/sync"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"log"
	"net/http"
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
	return fmt.Sprintf("%sREDACTED%s", string(s[0]), string(s[len(s)-1]))
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

	ctx.Sequence = sync.Mutex{
		Name:      ctx.Cfg.TestnetName,
		AWSRegion: ctx.Cfg.AWSRegion,
	}
	ctx.BrokenFlag = sync.Mutex{
		Name:      fmt.Sprintf("%s-brokenflag", ctx.Cfg.TestnetName),
		AWSRegion: ctx.Cfg.AWSRegion,
	}

	ctx.BrokenFlag.Lock()
	if ctx.BrokenFlag.Value == "broken" {
		coreCtx := sdkCtx.CoreContext{
			ChainID:         ctx.Cfg.TestnetName,
			Height:          0,
			Gas:             200000,
			TrustNode:       false,
			NodeURI:         ctx.Cfg.Node,
			FromAddressName: "faucetAccount",
			AccountNumber:   ctx.Cfg.AccountNumber,
			Sequence:        0,
			Client:          ctx.RpcClient,
			Decoder:         authcmd.GetAccountDecoder(ctx.Cdc),
			AccountStore:    "acc",
		}

		ctx.Sequence.Lock()
		seq, err := coreCtx.NextSequence([]byte(ctx.Cfg.AccountAddress))
		if err != nil {
			return nil, err
		}
		ctx.Sequence.Value = string(seq)
		ctx.Sequence.Unlock()
		// Reset broken flag
		ctx.BrokenFlag.Value = "0"
	}
	ctx.BrokenFlag.Unlock()

	ctx.RpcClient = rpcclient.NewHTTP(ctx.Cfg.Node, "/websocket")
	ctx.Cdc = app.MakeCodec()

	recaptcha.Init(ctx.Cfg.RecaptchaSecret)

	return
}
