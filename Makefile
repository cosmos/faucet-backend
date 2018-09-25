PACKAGES=$(shell go list ./... | grep -v '/vendor/')
BUILD_NUMBER ?= 4

BUILD_FLAGS = -tags "netgo ledger" -ldflags "-extldflags \"-static\" -X github.com/cosmos/faucet-backend/defaults.Release=${BUILD_NUMBER}"

########################################
### Build

build:
	CGO_ENABLED=0 LEDGER_ENABLED=false go build $(BUILD_FLAGS) -o build/f11 .

build-linux:
	#GOOS=linux GOARCH=amd64 $(MAKE) build
	docker run -it --rm -v $(GOPATH):/go golang:1.10.3 make -C /go/src/github.com/cosmos/faucet-backend build

########################################
### Tools & dependencies

DEP = github.com/golang/dep/cmd/dep
DEP_CHECK := $(shell command -v dep 2> /dev/null)

check_tools:
	cd tools && $(MAKE) check_tools

update_tools:
	cd tools && $(MAKE) update_tools

get_tools:
	cd tools && $(MAKE) get_tools

get_vendor_deps:
	@rm -rf vendor/
	@echo "--> Running dep ensure"
	@dep ensure -v


########################################
### Testing

test: test_unit

test_cli:
	@go test -count 1 -p 1 `go list github.com/cosmos/faucet-backend`

test_unit:
	@go test $(PACKAGES)


########################################
### Localnet

localnet-start:
	build/f11 -webserver -no-recaptcha -no-rdb -no-limit

localnet-lambda:
	# (Requirements: pip3 install aws-sam-cli)
	# Set up env.vars in template.yml since the --env-vars option doesn't seem to work
	sam local start-api

########################################
### Release management (set up requirements manually)

package:
	zip "build/f11.zip" build/f11

#sam-deploy:
#	sam deploy --template-file resources/template.yml --stack-name "f11-staging" --capabilities CAPABILITY_IAM --region "us-east-1"

create-lambda-staging:
	if [ -z "$(IAM_ROLE_ARN)" ]; then echo "Please set IAM_ROLE_ARN to something like arn:aws:iam::000000000000:role/faucet-lambda" ; false ; fi
	aws lambda create-function \
	--function-name F11-staging \
	--runtime go1.x \
	--role $(IAM_ROLE_ARN) \
	--handler build/f11 \
	--description "Gaia F11 Staging" \
	--timeout 120 \
	--publish \
	--environment "`cat f11.json | tr -d '\n'`" \
	--zip-file fileb://build/f11.zip \
	| jq -r .FunctionArn | tee tmp/lambdastagingarn.tmp

create-lambda-prod:
	if [ -z "$(IAM_ROLE_ARN)" ]; then echo "Please set IAM_ROLE_ARN to something like arn:aws:iam::000000000000:role/faucet-lambda" ; false ; fi
	mkdir -p tmp

	#Create Lambda function
	aws lambda create-function \
	--function-name F11-prod \
	--runtime go1.x \
	--role $(IAM_ROLE_ARN) \
	--handler build/f11 \
	--description "Gaia F11 PROD" \
	--timeout 120 \
	--publish \
	--environment "`cat f11.json | tr -d '\n'`" \
	--zip-file fileb://build/f11.zip \
	| jq -r .FunctionArn | tee tmp/lambdaprodarn.tmp

create-api-staging:

#TODO: Make it a swagger template
create-api-prod:
	if [ -z "`which jq`" ]; then echo "Please install jq." ; false ; fi
	mkdir -p tmp

	#Create the API Gateway
	aws apigateway create-rest-api \
	--name f11-prod \
	--description "F11 PROD API" \
	--endpoint-configuration "types=REGIONAL" | jq -r .id | tee tmp/apiid.tmp

###
### Path: GET /
###

	#Get the / resource ID
	aws apigateway get-resources \
	--rest-api-id `cat tmp/apiid.tmp` \
	| jq -r '.items[] | select(.path=="/")'.id | tee tmp/apislashid.tmp

	#Create the GET method for /
	aws apigateway put-method \
	--rest-api-id `cat tmp/apiid.tmp` \
	--resource-id `cat tmp/apislashid.tmp` \
	--http-method GET \
	--authorization-type NONE \
	--no-api-key-required \
	--operation-name "GetVersion"

	#Create integration for /
	aws apigateway put-integration \
	--rest-api-id `cat tmp/apiid.tmp` \
	--resource-id `cat tmp/apislashid.tmp` \
	--http-method GET \
	--type AWS_PROXY \
    --integration-http-method POST \
    --uri arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/`cat tmp/lambdaprodarn.tmp`/invocations \
    --content-handling CONVERT_TO_TEXT

	#Create integration response for /
	aws apigateway put-integration-response \
	--rest-api-id `cat tmp/apiid.tmp` \
	--resource-id `cat tmp/apislashid.tmp` \
	--http-method GET \
	--status-code 200 \
	--response-templates "application/json=Empty"

	#Create method response for /
	aws apigateway put-method-response \
	--rest-api-id `cat tmp/apiid.tmp` \
	--resource-id `cat tmp/apislashid.tmp` \
	--http-method GET \
	--status-code 200 \
	--response-models "application/json=Empty"

###
### Path: OPTIONS /v1/claim
###

	#Create the /v1 resource
	aws apigateway create-resource \
	--rest-api-id `cat tmp/apiid.tmp` \
	--parent-id `cat tmp/apislashid.tmp` \
	--path-part "v1" | jq -r .id | tee tmp/apiv1id.tmp

	#Create the /v1/claim resource
	aws apigateway create-resource \
	--rest-api-id `cat tmp/apiid.tmp` \
	--parent-id `cat tmp/apiv1id.tmp` \
	--path-part "claim" | jq -r .id | tee tmp/apiclaimid.tmp

	#Create the OPTIONS method for /v1/claim
	aws apigateway put-method \
	--rest-api-id `cat tmp/apiid.tmp` \
	--resource-id `cat tmp/apiclaimid.tmp` \
	--http-method OPTIONS \
	--authorization-type NONE \
	--no-api-key-required \
	--operation-name "CORS"

	#Create integration for OPTIONS /v1/claim
	aws apigateway put-integration \
	--rest-api-id `cat tmp/apiid.tmp` \
	--resource-id `cat tmp/apiclaimid.tmp` \
	--http-method OPTIONS \
	--type AWS_PROXY \
    --integration-http-method POST \
    --uri arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/`cat tmp/lambdaprodarn.tmp`/invocations \
    --content-handling CONVERT_TO_TEXT

	#Create integration response for OPTIONS /v1/claim
	aws apigateway put-integration-response \
	--rest-api-id `cat tmp/apiid.tmp` \
	--resource-id `cat tmp/apiclaimid.tmp` \
	--http-method OPTIONS \
	--status-code 200 \
	--response-templates "application/json=Empty"

	#Create method response for OPTIONS /v1/claim
	aws apigateway put-method-response \
	--rest-api-id `cat tmp/apiid.tmp` \
	--resource-id `cat tmp/apiclaimid.tmp` \
	--http-method OPTIONS \
	--status-code 200 \
	--response-models "application/json=Empty"

###
### Path: POST /v1/claim
###

	#Create the POST method for /v1/claim
	aws apigateway put-method \
	--rest-api-id `cat tmp/apiid.tmp` \
	--resource-id `cat tmp/apiclaimid.tmp` \
	--http-method POST \
	--authorization-type NONE \
	--no-api-key-required \
	--operation-name "ClaimTokens"

	#Create integration for POST /v1/claim
	aws apigateway put-integration \
	--rest-api-id `cat tmp/apiid.tmp` \
	--resource-id `cat tmp/apiclaimid.tmp` \
	--http-method POST \
	--type AWS_PROXY \
    --integration-http-method POST \
    --uri arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/`cat tmp/lambdaprodarn.tmp`/invocations \
    --content-handling CONVERT_TO_TEXT

	#Create integration response for POST /v1/claim
	aws apigateway put-integration-response \
	--rest-api-id `cat tmp/apiid.tmp` \
	--resource-id `cat tmp/apiclaimid.tmp` \
	--http-method POST \
	--status-code 200 \
	--response-templates "application/json=Empty"

	#Create method response for POST /v1/claim
	aws apigateway put-method-response \
	--rest-api-id `cat tmp/apiid.tmp` \
	--resource-id `cat tmp/apiclaimid.tmp` \
	--http-method POST \
	--status-code 200 \
	--response-models "application/json=Empty"

###
### Deploy
###

	#Last step: deploy API
	aws apigateway create-deployment \
	--rest-api-id `cat tmp/apiid.tmp` \
	--stage-name prod \
	--stage-description "PROD deployment" \
	--description "automated PROD deployment"

	echo rm tmp/apiid.tmp tmp/apires.tmp tmp/apislashid.tmp tmp/apiclaimid.tmp

update-staging: build-linux package
	aws lambda update-function-code --function-name "F11-staging" --zip-file fileb://build/f11.zip --region us-east-1

update-prod: build-linux package
	aws lambda update-function-code --function-name "F11-prod" --zip-file fileb://build/f11.zip --region us-east-1

list-lambda:
	aws lambda list-functions --region us-east-1

.PHONY: build build-linux check_tools update_tools get_tools get_vendor_deps test test_cli test_unit package create-lambda-staging create-lambda-prod update-staging update-prod
