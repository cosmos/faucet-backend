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
	"log"
	"net/http"
	"os"
	"os/user"
	"time"
)

// MainHandler handles the requests coming to `/`.
func MainHandler(ctx *context.Context, w http.ResponseWriter, r *http.Request) (status int, err error) {
	status = http.StatusOK
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(struct {
		Message string `json:"message"`
		Name    string `json:"name"`
		Version string `json:"version"`
	}{
		Message: "",
		Name:    ctx.TestnetName,
		Version: defaults.Version,
	})
	return
}

// AddRoutes adds the routes of the different calls to GorillaMux.
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

// redact changes a string to XXXXXX - used to redact passwords when logging.
func redact(s string) string {
	if len(s) < 2 {
		return "RD"
	}
	return "REDACTED"
}

// Initialization creates and populates the context and sets up connectivity to the testnet.
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

	ctx.Cdc = app.MakeCodec()

	err = ctx.GetTestnetName()
	if err != nil {
		log.Print("underlying full node seems to have issues")
		return
	}

	log.Printf("config loaded, testnet name: %s", ctx.TestnetName)

	ctx.SequenceMutex = sync.Mutex{
		Name:      fmt.Sprintf("%s-%s-sequence", ctx.Cfg.ApiEnvironment, ctx.TestnetName),
		AWSRegion: ctx.Cfg.AWSRegion,
		Expiry:    60 * time.Second,
	}.WithTimeout(70 * time.Second)

	ctx.AccountNumberMutex = sync.Mutex{
		Name:      fmt.Sprintf("%s-%s-accountnumber", ctx.Cfg.ApiEnvironment, ctx.TestnetName),
		AWSRegion: ctx.Cfg.AWSRegion,
		Expiry:    1 * time.Second,
	}.WithTimeout(3 * time.Second)

	ctx.BrokenFlagMutex = sync.Mutex{
		Name:      fmt.Sprintf("%s-%s-brokenflag", ctx.Cfg.ApiEnvironment, ctx.TestnetName),
		AWSRegion: ctx.Cfg.AWSRegion,
		Expiry:    1 * time.Second,
	}.WithTimeout(3 * time.Second)

	err = ctx.CheckAndFixAccountDetails()
	if err != nil {
		return
	}

	// This is not really a Mutex. We use the Mutex as a database store:
	// read the value once and reuse it without checking the database.
	ctx.AccountNumberMutex.Lock()
	ctx.AccountNumberMutex.Unlock()

	// Create CLIContext
	cliContext := sdkCtx.NewCLIContext().
		WithCodec(ctx.Cdc).
		WithLogger(os.Stdout).
		WithAccountDecoder(authcmd.GetAccountDecoder(ctx.Cdc)).
		WithNodeURI(ctx.Cfg.Node)
	ctx.CLIContext = &cliContext

	// Create TxContest
	txCtx := authctx.TxContext{
		ChainID: ctx.TestnetName,
		Gas:     20000,
	}.WithCodec(ctx.Cdc)
	ctx.TxContest = &txCtx

	// Create Throttled limiter
	err = createThrottledLimiter(ctx)
	if err != nil {
		return
	}

	recaptcha.Init(ctx.Cfg.RecaptchaSecret)

	log.Print("initialized context")

	return
}

// GetPrivkeyBytesFromString translates a string into a set of private key bytes.
func GetPrivkeyBytesFromString(privkeystring string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(privkeystring)
}

// GetStringFromPrivkeyBytes translates private key bytes into a readable (base64 encoded) string.
func GetStringFromPrivkeyBytes(privkeybytes []byte) string {
	return base64.StdEncoding.EncodeToString(privkeybytes)
}

// GetPrivkeyBytesFromUserFile gets the private key in a byte array format form the local keystore.
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
