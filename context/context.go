// Context package defines primitives for generic context handling during execution.
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
	"time"
)

// Context holds current execution details.
type Context struct {

	// Throttled Rate Limiter
	HttpRateLimiter throttled.HTTPRateLimiter

	// Throttled Rate Limiter Store
	Store throttled.GCRAStore

	// SequenceMutex stores the current last sequence number of the account on the testnet
	SequenceMutex      sync.Mutex
	// AccountNumberMutex stores the account number on the testnet for the respective wallet
	AccountNumberMutex sync.Mutex
	// BrokenFlagMutex stores if the last execution of the application was successful (tokens were sent)
	BrokenFlagMutex    sync.Mutex

	// Deprecated: We only need to read AccountNumber once at startup, we store it for subsequent use
	AccountNumber int64

	// TestnetName returned from the full node
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

// InitialContext holds the input parameter details at the start of execution.
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

	// --no-limit Disable rate limiter
	DisableLimiter bool

	// --no-send Disable transaction send to the testnet (dry-run)
	DisableSend bool

	// --no-recaptcha Disable recaptcha
	DisableRecaptcha bool
}

// New creates a fresh Context.
func New() *Context {
	return &Context{}
}

// NewInitialContext creates a fresh InitialContext.
func NewInitialContext() *InitialContext {
	return &InitialContext{}
}

// ErrorMessage defines the message structure returned when an error happens.
type ErrorMessage struct {
	Message string `json:"message"`
}

// Handler is an abstraction layer to standardize web API returns, if an error happens.
type Handler struct {
	C *Context
	H func(*Context, http.ResponseWriter, *http.Request) (int, error)
}

// ServeHTTP is a wrapper around web API calls, that adds a default Content-Type and formats outgoing error messages.
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

// CheckAndFixAccountDetails checks, if the last run was unsuccessful (node down, wrong parameters)
// and tries to fix the values from the testnet.
func (ctx *Context) CheckAndFixAccountDetails() (err error) {

	ctx.BrokenFlagMutex.Lock()
	defer ctx.BrokenFlagMutex.Unlock()

	if ctx.BrokenFlagMutex.GetValueString() == "no" {
		return
	}

	var httpClient = &http.Client{Timeout: 5 * time.Second}
	var req *http.Response
	var rawBody []byte

	req, err = httpClient.Get(fmt.Sprintf("%s/accounts/%s", ctx.Cfg.LCDNode, ctx.Cfg.AccountAddress))
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
		return errors.New(fmt.Sprintf("http error code %d calling LCD URL", req.StatusCode))
	}

	var accountDetails auth.Account
	err = ctx.Cdc.UnmarshalJSON(rawBody, &accountDetails)
	if err != nil {
		return
	}

	ctx.SequenceMutex.Lock()
	ctx.SequenceMutex.SetValueInt64(accountDetails.GetSequence())
	ctx.SequenceMutex.Unlock()

	ctx.AccountNumberMutex.Lock()
	ctx.AccountNumberMutex.SetValueInt64(accountDetails.GetAccountNumber())
	ctx.AccountNumberMutex.Unlock()

	// Reset broken flag
	ctx.BrokenFlagMutex.SetValueString("no")
	return

}

// RaiseBrokenAccountDetails raises the flag that the configuration is broken (parameter change, node timeout).
func (ctx *Context) RaiseBrokenAccountDetails(message string) {
	if message == "no" {
		message = ""
	}
	ctx.BrokenFlagMutex.Lock()
	ctx.BrokenFlagMutex.SetValueString(message)
	ctx.BrokenFlagMutex.Unlock()
}

// GetTestnetName returns the testnet name from the node
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
