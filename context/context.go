package context

import (
	"encoding/json"
	"github.com/cosmos/cosmos-sdk/wire"
	"github.com/cosmos/faucet-backend/config"
	"github.com/cosmos/faucet-backend/defaults"
	"github.com/greg-szabo/dsync/ddb/sync"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/throttled/throttled"
	"log"
	"net/http"
	sdkCtx "github.com/cosmos/cosmos-sdk/client/context"
	authctx "github.com/cosmos/cosmos-sdk/x/auth/client/context"
)

type Context struct {

	// Throttled Rate Limiter
	HttpRateLimiter throttled.HTTPRateLimiter

	// Throttled Rate Limiter Store
	Store throttled.GCRAStore

	// Distributed Mutexes
	Sequence   sync.Mutex
	BrokenFlag sync.Mutex

	// Application configuration
	Cfg *config.Config

	// Disable rate limiter for testing
	DisableLimiter bool

	// Disable sending of transaction to the network
	DisableSend bool

	// Disable ReCaptcha check for testing
	DisableRecaptcha bool

	// Amino MarshalBinary for transaction broadcast on blockchain network
	Cdc *wire.Codec

	// Blockchain network socket connection details
	RpcClient *rpcclient.HTTP

	// The new CLIContext for connecting to a network
	CLIContext *sdkCtx.CLIContext

	// The new TxContext for sending a transaction on the network
	TxContest *authctx.TxContext
}

type InitialContext struct {
	// --no-rdb disable RedisDB database usage
	DisableRDb bool

	// --send Send to a wallet locally
	Send string

	// --ip IP address of local webserver
	WebserverIp string

	// --port Port number of local webserver
	WebserverPort uint

	// --config Config file for local execution
	ConfigFile string

	// --webserver or --send was set
	LocalExecution bool

	// (--no-limit) Disable rate limiter
	DisableLimiter bool

	// (--no-send) Disable transaction send to the blockchain network
	DisableSend bool

	// (--no-recaptcha) Disable recaptcha
	DisableRecaptcha bool
}

func New() *Context {
	return &Context{}
}

func NewInitialContext() *InitialContext {
	return &InitialContext{}
}

type ErrorMessage struct {
	Message string `json:"message"`
}

type Handler struct {
	C *Context
	H func(*Context, http.ResponseWriter, *http.Request) (int, error)
}

func (fn Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", defaults.ContentType)
	if status, err := fn.H(fn.C, w, r); err != nil {
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(ErrorMessage{err.Error()})
		log.Printf("%d %s", status, err.Error())
	}
}
