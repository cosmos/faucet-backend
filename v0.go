package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	sdkCtx "github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/cmd/gaia/app"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	authctx "github.com/cosmos/cosmos-sdk/x/auth/client/context"
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
	"os/user"
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
		return "RD"
	}
	return "REDACTED"
}

func Initialization(initialContext *context.InitialContext) (ctx *context.Context, err error) {

	ctx = context.New()
	ctx.DisableLimiter = initialContext.DisableLimiter
	ctx.DisableRecaptcha = initialContext.DisableRecaptcha
	ctx.DisableSend = initialContext.DisableSend

	if initialContext.LocalExecution {
		log.Printf("loading from config file %s", initialContext.ConfigFile)
		ctx.Cfg, err = config.GetConfigFromFile(initialContext.ConfigFile)
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

	if initialContext.DisableRDb {
		ctx.Store, err = createMemStore()
		if err != nil {
			return
		}
	} else {
		ctx.Store, err = createRedisStore(ctx)
		if err != nil {
			return
		}

	}

	log.Printf("config for %s loaded", ctx.Cfg.TestnetName)

	cliContext := sdkCtx.NewCLIContext().
		WithCodec(ctx.Cdc).
		WithLogger(os.Stdout).
		WithAccountDecoder(authcmd.GetAccountDecoder(ctx.Cdc)).
		WithNodeURI(ctx.Cfg.Node)
	ctx.CLIContext = &cliContext

	txCtx := authctx.TxContext{
		ChainID:       ctx.Cfg.TestnetName,
		Gas:           20000,
	}.WithCodec(ctx.Cdc)

	ctx.TxContest = &txCtx

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

		ctx.Sequence.Lock()
		ctx.Sequence.Value = "0"
//Todo: create a GetAccountSequence without using the KeyBase
/*		seq, err :=	ctx.CLIContext.GetAccountSequence([]byte(ctx.Cfg.AccountAddress))
		if err != nil {
			return nil, err
		}
		ctx.Sequence.Value = string(seq)
*/		ctx.Sequence.Unlock()

		// Reset broken flag
		ctx.BrokenFlag.Value = "0"
	}
	ctx.BrokenFlag.Unlock()

	ctx.RpcClient = rpcclient.NewHTTP(ctx.Cfg.Node, "/websocket")
	ctx.Cdc = app.MakeCodec()

	recaptcha.Init(ctx.Cfg.RecaptchaSecret)

	return
}

func GetPrivkeyBytesFromString(privkeystring string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(privkeystring)
}

func GetStringFromPrivkeyBytes(privkeybytes []byte) (string) {
	return base64.StdEncoding.EncodeToString(privkeybytes)
}

// ValarDragon is the best! (He wrote this function.)
func GetPrivkeyBytesFromUserFile(name string, passphrase string) []byte {
	usr, err := user.Current()
	if err != nil {
		fmt.Println("Error couldn't get user", err)
		return nil
	}
	homeDir := usr.HomeDir
	gaiacliHome := fmt.Sprintf("%s%s.gaiacli", homeDir, string(os.PathSeparator))
	keybase, _ := keys.GetKeyBaseFromDir(gaiacliHome)
	privkey, _ := keybase.ExportPrivateKeyObject(name, passphrase)
	return privkey.Bytes()
}
