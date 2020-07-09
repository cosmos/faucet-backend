> This repo is archived as of July 9, 2020 but made available for historical record.

# Faucet-backend

## Overview

Faucet-backend (F11 for short) is a faucet API backend implemented both as an AWS Lambda function and as a stand-alone web service.

## How to use locally

For developers, it is easiest to run the code locally as a web service:

### Build

```bash
make get_tools get_vendor_deps build
```
- `get_tools` gets the `dep` package manager installed in `$GOPATH/bin`. This is optional if you already have `dep` in your `$PATH`.
- `get_vendor_deps` populates the `vendor` folder using `dep`. You only need to run this once, unless you switch the git branch or revision, or change the code and add dependencies.
- `build` builds a binary that runs on the local OS.

### Prepare to run

```bash
export AWS_ACCESS_KEY_ID=abcdefghijklm
export AWS_SECRET_ACCESS_KEY=xyzabcdefgh
cp f11.conf.template f11.conf
```
- `export` sets your AWS access.
- `cp` copies the config file in place.

**Edit f11.conf with your favorite editor and fill in the blanks.**

To get a private key, run
```bash
export PASSPHRASE="mykeypassword"
build/f11 -extract cosmosaddrmyfaucetkey
```
 with the name of your public key that is stored in your default key store.

### Run

```bash
build/f11 -webserver -config f11.conf
```

- it will run the local webserver on port 3000 and accept connections.

```bash
curl localhost:3000
curl localhost:3000/v1/claim -X POST -d '{"address":"cosmosaddr12345"}'
```

You can also run the binary to send one transaction and exit, with:
```bash
build/f11 -send cosmosaddr12345
```

## How to use on AWS Lambda

### Build
```bash
make build-linux package
```
AWS Lambda will only accept Linux binaries. Since most of our developers use OSX, the `build-linux` target will use docker to build a linux binary.

### Create Lambda function
```bash
cp f11.json.template f11.json
```
Before running the `make` command, open up the created `f11.json` file and fill in the environment variables. Find the descriptions of each variable in the `f11.conf.template` file.

Also create the IAM role (it's called `faucet-lambda` in the example) and copy its ARN. The role needs to give access to the function to access the DynamoDB table used for Mutexes (`AmazonDynamoDBLocksAccess` policy described in the `resources/` folder) and to execute as a Lambda function (`AWSLambdaBasicExecutionRole` policy described in AWS IAM).
When all is set, run:
```bash
IAM_ROLE_ARN=arn:aws:iam::000000000000:role/faucet-lambda make create-lambda-staging
```

You can deploy a new version by re-uploading to the same infrastructure:
```bash
make update-lambda-staging
```

### Create API gateway
```bash
make create-api-staging
```
This command will use a swagger template to create an API Gateway and connect the endpoints to the Lambda function. This does not need to be re-run, when the lambda function code is replaced by a newer version.

Note: `create-api-prod` uses native `awscli` commands instead of a swagger template to create the API Gateway and the endpoints. This implementation requires the Lambda function ARN. This is saved under `tmp/lambdaprodarn.tmp` during the Lambda function creation. I think the swagger implementation in `create-api-staging` is nicer, but both are kept for now to evaluate which one is more resilient.

# Improvements for the future and developer details

- middleware.go: Let the API Gateway handle CORS, instead of handling it in code.
- make more measurements on the usefulness of throttled. Maybe we don't need it since we have recaptcha.

## Timeouts

- Because Lambda functions (and web services for that matter) can be distributed across several servers, the code uses distributed mutexes to synchronize some information.
- Because code can be fallible and mutexes can be abandoned, a mutex expiry was introduced so a new process can take over an old processes mutex, if the old process didn't release it in a timely fashion.
- Mutexes also have timeout values. After this value is reached the Mutex is considered locked by another process for good and the current process panics. (Note that the other process can release the mutex within the timeout or the mutex can be considered abandoned within the timeout.)
- The Lambda function has a timeout value. After this value, AWS kills the function. We have to make sure that this value is high enough so we don't kill a working process.

The following list shows the current chosen timeout values to see how long an execution can take.

#### Independent functions

##### RaiseBrokenAccountDetails()
1. up to **3 seconds** to get the BrokenFlag, with 1 second expiry (more of some distributed data, than a real mutex - shows if during the previous run, we ran into some errors)

##### CheckAndFixAccountDetails()
1. up to **3 seconds** to get the BrokenFlag, with 1 second expiry
1. if the previous run was broken:
   1. up to **5 seconds** to query gaiacli for account details
   1. up to **70 seconds** to get the sequence number, with 60 seconds expiry
   1. up to **3 seconds** to get the account number, with 1 second expiry (this didn't need to be a mutex, it's a distributed read-only data in most cases, hence the quick expiry)

##### V1SendTx()
1. CheckAndFixAccountDetails() compiles into **81 seconds** maximum, **3 seconds** expected (Usually it was already run once during initialization.)
1. up to **70 seconds** to get the sequence number, with 60 seconds expiry (another process might be executing a transaction which locks the sequence number - although this is unlikely because of the code arrangement, this can be disputed)
1. up to **TIMEOUT=60 seconds** value to async broadcast the transaction
1. up to **3 seconds** to get the BrokenFlag, with 1 second expiry

#### Workflow of a transaction

##### Initialization()
1. up to **2 seconds** to get the testnet name from the gaiad node
1. CheckAndFixAccountDetails() compiles into **81 seconds** maximum, **3 seconds** regular (only runs the maximum if the previous run was broken)
1. up to **3 seconds** to get the account number, with 1 second expiry (this didn't need to be a mutex, it's a distributed read-only data in most cases, hence the quick expiry)

##### V1ClaimHandler()
1. V1SendTx() compiles into **214 seconds** as the worst-case scenario. **120 seconds** is a more reasonable maximum in a general run
1. RaiseBrokenAccountDetails() compiles into **3 seconds**

#### Lambda function timeout considerations
- The Lambda function runtime will strongly depend on the underlying full node and LCD node stability. If the network is congested, the node servers are overloaded, then maximum timeout values can be reached. On a stable set of nodes, the timeout is more likely stay around 1-3 seconds in all cases.
- In an ideal setup, the Lambda function should be able to run within **10 seconds**.
- In a broken setup case, it should be able to time out within **70 seconds**. (Experimentation shows around 10 seconds in practice.)
