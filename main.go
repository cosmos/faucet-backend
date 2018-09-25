// main package that executes the code
//
// F11 or faucet-backend is a web API that allows users to request tokens on a testnet.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/gorillamux"
	sdkversion "github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/faucet-backend/defaults"
	tendermintversion "github.com/tendermint/tendermint/version"
	"log"
	"os/signal"
	"syscall"

	"github.com/cosmos/faucet-backend/context"
	"net/http"
	"os"
	"time"
)

// lambdaInitialized is an indicator that tells if the AWS Lambda function is in the startup phase.
var lambdaInitialized = false

// Translates Gorilla Mux calls to AWS API Gateway calls
var lambdaProxy func(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

// LambdaHandler is the callback function when the application is set up as an AWS Lambda function.
func LambdaHandler(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	if !lambdaInitialized {
		// stdout and stderr are sent to AWS CloudWatch Logs
		log.Print("cold start")

		var err error
		ctx, err := Initialization(context.NewInitialContext())
		if err != nil {
			log.Printf("initialization failed: %v\n", err)
			errbody, _ := json.Marshal(context.ErrorMessage{
//				Message: err.Error(),
				Message: "System could not be initialized, please contact the administrator.",
			})
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       string(errbody),
			}, nil
		}

		r := AddRoutes(ctx)
		muxLambda := gorillamux.New(r)
		lambdaProxy = muxLambda.Proxy

		lambdaInitialized = true
	}

	return lambdaProxy(req)

}

// WebserverHandler is the function that is called when the `--webserver` parameter is invoked.
// It sets up a local webserver for handling incoming requests.
func WebserverHandler(localCtx *context.InitialContext) {
	log.Print("webserver execution start")

	var err error
	ctx, err := Initialization(localCtx)
	if err != nil {
		log.Fatalf("initialization failed: %v\n", err)
	}

	r := AddRoutes(ctx)

	srv := &http.Server{
		Addr: fmt.Sprintf("%s:%d", localCtx.WebserverIp, localCtx.WebserverPort),
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 30,
		IdleTimeout:  time.Second * 60,
		Handler:      r,
	}

	var gracefulStop = make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)
	go func() {
		sig := <-gracefulStop
		log.Printf("caught signal: %+v", sig)
		log.Print("waiting 2 seconds to finish processing")
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// SendTransactionHandler is the function that is called when the `--send` parameter is invoked.
// It sends tokens to the specified address and then exits.
func SendTransactionHandler(localCtx *context.InitialContext) {
	log.Print("Send one transaction")

	var err error
	localCtx.LocalExecution = true // Read config from local file
	ctx, err := Initialization(localCtx)
	if err != nil {
		log.Fatalf("initialization failed: %v\n", err)
	}

	height, hash, errType, err := V1SendTx(ctx, localCtx.Send)
	if err != nil {
		log.Fatalf("(%d): %v", errType, err)
	}
	log.Printf("transaction committed. Hash: %s, Block height: %d", hash, height)
	return
}

func main() {
	var versionSwitch bool //--version
	var extract string     //--extract

	initialCtx := context.NewInitialContext()

	flag.BoolVar(&versionSwitch, "version", false, "Return version number and exit.")
	flag.StringVar(&extract, "extract", "", "Extract private key bytes from your local storage. Get passphrase from $PASSPHRASE environment variable")
	flag.StringVar(&initialCtx.Send, "send", "", "send a transaction with the local configuration")

	flag.BoolVar(&initialCtx.LocalExecution, "webserver", false, "run a local web-server instead of as an AWS Lambda function")
	flag.StringVar(&initialCtx.ConfigFile, "config", "f11.conf", "read config from this local file")
	flag.StringVar(&initialCtx.WebserverIp, "ip", "127.0.0.1", "IP to listen on")
	flag.UintVar(&initialCtx.WebserverPort, "port", 3000, "Port to listen on")
	flag.BoolVar(&initialCtx.DisableRDb, "no-rdb", false, "Disable the use of RedisDB")

	flag.BoolVar(&initialCtx.DisableLimiter, "no-limit", false, "Disable rate-limiter")
	flag.BoolVar(&initialCtx.DisableSend, "no-send", false, "Do not send the transaction to the blockchain network")
	flag.BoolVar(&initialCtx.DisableRecaptcha, "no-recaptcha", false, "Disable recaptcha checks")
	flag.Parse()

	//--version
	if versionSwitch {
		fmt.Println(defaults.Version)
		fmt.Printf("SDK: %v\n", sdkversion.Version)
		fmt.Printf("Tendermint: %v\n", tendermintversion.Version)
	} else {
		//--extract
		if extract != "" {
			privateKeyBytes := GetPrivkeyBytesFromUserFile(extract, os.Getenv("PASSPHRASE"))
			privateKeyString := GetStringFromPrivkeyBytes(privateKeyBytes)
			fmt.Println(privateKeyString)
		} else {
			//--send
			if initialCtx.Send != "" {
				SendTransactionHandler(initialCtx)
			} else {
				//--webserver
				if initialCtx.LocalExecution {
					WebserverHandler(initialCtx)
				} else {
					//Lambda function on AWS
					lambda.Start(LambdaHandler)
				}
			}
		}
	}
}
