package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/gorillamux"
	sdkversion "github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/faucet-backend/defaults"
	tendermintversion "github.com/tendermint/tendermint/version"
	"log"

	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/faucet-backend/context"
	"net/http"
	"os"
	"os/user"
	"time"
)

// Indicator if the AWS Lambda function is in the startup phase
var lambdaInitialized = false

// Translates Gorilla Mux calls to AWS API Gateway calls
var lambdaProxy func(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

func LambdaHandler(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	if !lambdaInitialized {
		// stdout and stderr are sent to AWS CloudWatch Logs
		log.Print("cold start")

		var err error
		ctx, err := Initialization(true, true, "")
		if err != nil {
			log.Fatalf("initialization failed: %v\n", err)
		}

		r := AddRoutes(ctx)

		muxLambda := gorillamux.New(r)

		lambdaInitialized = true
		lambdaProxy = muxLambda.Proxy

	}

	//Todo: Add lambda timeout function so a response is made before the function times out in AWS
	//(Default Lambda timeout is 3 seconds.)

	return lambdaProxy(req)

}

func LocalExecution(localCtx *context.LocalContext) {
	log.Print("local execution start")

	var err error
	ctx, err := Initialization(localCtx.ForceDDb, localCtx.ForceRDb, localCtx.LocalConfig)
	if err != nil {
		log.Fatalf("initialization failed: %v\n", err)
	}

	r := AddRoutes(ctx)

	if localCtx.Send != "" {
		height, hash, errType, err := V1SendTx(ctx, localCtx.Send)
		if err != nil {
			log.Fatalf("(%d): %v", errType, err)
		}
		log.Printf("transaction committed. Hash: %s, Block height: %d", hash, height)
		return
	}

	srv := &http.Server{
		Addr: fmt.Sprintf("%s:%d", localCtx.WebserverIp, localCtx.WebserverPort),
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 30,
		IdleTimeout:  time.Second * 60,
		Handler:      r,
	}

	// Run our server in a goroutine so that it doesn't block.
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
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

func main() {
	var versionSwitch bool  //--version
	var extract string      //--extract
	var localExecution bool //--webserver

	localCtx := context.NewLocal()

	// All command-line parameters are only for development and troubleshooting purposes
	flag.BoolVar(&versionSwitch, "version", false, "Return version number and exit.")
	flag.StringVar(&extract, "extract", "", "Extract private key bytes from your local storage. Get passphrase from $PASSPHRASE environment variable")
	flag.StringVar(&localCtx.Send, "send", "", "send a transaction with the local configuration")

	flag.BoolVar(&localExecution, "webserver", false, "run a local web-server instead of as an AWS Lambda function")
	flag.StringVar(&localCtx.LocalConfig, "config", "config.json", "read config from local file (default: config.json)")
	flag.StringVar(&localCtx.WebserverIp, "ip", "127.0.0.1", "IP to listen on (default: 127.0.0.1) - for development and troubleshooting purposes")
	flag.UintVar(&localCtx.WebserverPort, "port", 3000, "Port to listen on (default: 3000) - for development and troubleshooting purposes")
	flag.BoolVar(&localCtx.ForceDDb, "force-ddb", false, "Force the use of DynamoDB even when run locally")
	flag.BoolVar(&localCtx.ForceRDb, "force-rdb", false, "Force the use of RedisDB even when run locally")

	flag.BoolVar(&defaults.DisableLimiter, "no-limit", false, "Disable rate-limiter")
	flag.BoolVar(&defaults.DisableSend, "no-send", false, "Do not send the transaction to the blockchain network")
	flag.BoolVar(&defaults.DisableRecaptcha, "no-recaptcha", false, "Disable recaptcha checks")
	flag.Parse()

	//--version
	if versionSwitch {
		fmt.Println(defaults.Version)
		fmt.Printf("Testnet: %s\n", defaults.TestnetName)
		fmt.Printf("SDK: %v\n", sdkversion.Version)
		fmt.Printf("Tendermint: %v\n", tendermintversion.Version)
	} else {
		//--extract
		if extract != "" {
			privateKeyBytes := GetPrivkeyBytesFromUserFile(extract, os.Getenv("PASSPHRASE"))
			privateKeyString := base64.StdEncoding.EncodeToString(privateKeyBytes)
			fmt.Println(privateKeyString)
		} else {
			//--webserver or --send
			if localCtx.Send != "" || localExecution {
				LocalExecution(localCtx)
			} else {
				//no input parameters
				lambda.Start(LambdaHandler)
			}
		}
	}
}
