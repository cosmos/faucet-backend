package context

import (
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/cosmos/cosmos-sdk/wire"
	"github.com/greg-szabo/dsync/ddb/sync"
	"github.com/greg-szabo/f11/config"
	"github.com/greg-szabo/f11/defaults"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/throttled/throttled"
	"log"
	"net/http"
)

type Context struct {

	// Throttled Rate Limiter
	HttpRateLimiter throttled.HTTPRateLimiter

	// Throttled Rate Limiter Store
	Store throttled.GCRAStore

	// AWS Session connection for API calls
	AwsSession *session.Session

	// AWS DynamoDB connection
	DbSession *dynamodb.DynamoDB

	Mutex sync.Mutex

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
}

type LocalContext struct {
	// --force-ddb Force DynamoDB database usage
	ForceDDb bool

	// --force-rdb Force RedisDB database usage
	ForceRDb bool

	// --send Send to a wallet locally
	Send string

	// --ip IP address of local webserver
	WebserverIp string

	// --port Port number of local webserver
	WebserverPort uint

	// --config Config file for local execution
	LocalConfig string
}

func New() *Context {
	return &Context{
		DisableLimiter:   defaults.DisableLimiter,
		DisableSend:      defaults.DisableSend,
		DisableRecaptcha: defaults.DisableRecaptcha,
	}

}

func NewLocal() *LocalContext {
	return &LocalContext{
		WebserverIp:   "127.0.0.1",
		WebserverPort: 3000,
		LocalConfig:   "config.json",
	}
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
