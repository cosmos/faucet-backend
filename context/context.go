package context

import (
	"encoding/json"
	"fmt"
	sdkCtx "github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/wire"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authctx "github.com/cosmos/cosmos-sdk/x/auth/client/context"
	"github.com/cosmos/faucet-backend/config"
	"github.com/cosmos/faucet-backend/defaults"
	"github.com/greg-szabo/dsync/ddb/sync"
	"github.com/pkg/errors"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/throttled/throttled"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"
)

type Context struct {

	// Throttled Rate Limiter
	HttpRateLimiter throttled.HTTPRateLimiter

	// Throttled Rate Limiter Store
	Store throttled.GCRAStore

	// Distributed Mutexes
	SequenceMutex      sync.Mutex
	AccountNumberMutex sync.Mutex
	BrokenFlagMutex    sync.Mutex
	TestnetNameMutex   sync.Mutex

	// We only need to read AccountNumber once at startup, we store it for subsequent use
	AccountNumber int64

	// We only need to read TestnetName once at startup, we store it for subsequent use
	TestnetName string

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
		if status == http.StatusInternalServerError {
			fn.C.RaiseBrokenAccountDetails(err.Error())
		}
	}
}

// Check if the last run was unsuccessful (node down, wrong parameters) and try to fix the values from the network
func (ctx *Context) CheckAndFixAccountDetails() (err error) {

	ctx.BrokenFlagMutex.Lock()
	if ctx.BrokenFlagMutex.Value != "no" {
		var httpClient = &http.Client{Timeout: 5 * time.Second}
		var req *http.Response
		var rawBody []byte

		req, err = httpClient.Get(fmt.Sprintf("%s/accounts/%s", ctx.Cfg.LCDNode, ctx.Cfg.AccountAddress))
		if err != nil {
			ctx.BrokenFlagMutex.Unlock()
			return
		}
		defer req.Body.Close()

		if req.StatusCode == http.StatusOK {
			rawBody, err = ioutil.ReadAll(req.Body)
			if err != nil {
				ctx.BrokenFlagMutex.Unlock()
				return
			}
		} else {
			return errors.New(fmt.Sprintf("http error code %d calling LCD URL", req.StatusCode))
		}

		var accountDetails auth.Account
		err = ctx.Cdc.UnmarshalJSON(rawBody, &accountDetails)
		if err != nil {
			ctx.BrokenFlagMutex.Unlock()
			return
		}

		ctx.SequenceMutex.Lock()
		ctx.SequenceMutex.Value = strconv.FormatInt(accountDetails.GetSequence(), 10)
		ctx.SequenceMutex.Unlock()

		ctx.AccountNumberMutex.Lock()
		ctx.AccountNumberMutex.Value = strconv.FormatInt(accountDetails.GetAccountNumber(), 10)
		ctx.AccountNumberMutex.Unlock()

		// Reset broken flag
		ctx.BrokenFlagMutex.Value = "no"
	}
	ctx.BrokenFlagMutex.Unlock()

	return

}

// Raise the flag that the configuration is broken (parameter change, node timeout)
func (ctx *Context) RaiseBrokenAccountDetails(message string) {
	if message == "no" {
		message = ""
	}
	ctx.BrokenFlagMutex.Lock()
	ctx.BrokenFlagMutex.Value = message
	ctx.BrokenFlagMutex.Unlock()
}

// Get the testnet name from the node
func (ctx *Context) GetTestnetName() (err error) {
	var httpClient = &http.Client{Timeout: 2 * time.Second}
	var req *http.Response
	var rawBody []byte

	req, err = httpClient.Get(fmt.Sprintf("%s/status", ctx.Cfg.Node))
	if err != nil {
		return
	}
	defer req.Body.Close()

	if req.StatusCode == http.StatusOK {
		rawBody, err = ioutil.ReadAll(req.Body)
		if err != nil {
			return
		}
	} else {
		return errors.New(fmt.Sprintf("http error code %d calling Node URL", req.StatusCode))
	}

	rpcResponse := &rpctypes.RPCResponse{}
	ctx.Cdc.UnmarshalJSON(rawBody, &rpcResponse)
	if err != nil {
		return
	}

	resultStatus := &ctypes.ResultStatus{}
	err = ctx.Cdc.UnmarshalJSON(rpcResponse.Result, &resultStatus)
	if err != nil {
		return
	}

	if resultStatus.NodeInfo.Network == "" {
		return errors.New("Could not get testnet name from node")
	}
	ctx.TestnetName = resultStatus.NodeInfo.Network
	return

}
