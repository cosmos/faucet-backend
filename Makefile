PACKAGES=$(shell go list ./... | grep -v '/vendor/')
BUILD_NUMBER ?= 3
TESTNET_NAME ?= gaia-7002
DDB_TABLE ?= ddb-f11
DDB_INSTANCE ?= public
AWS_REGION ?= us-east-2

BUILD_FLAGS = -tags "netgo ledger" -ldflags "-extldflags \"-static\" -X github.com/greg-szabo/f11/defaults.Release=${BUILD_NUMBER} -X github.com/greg-szabo/f11/defaults.DynamoDBTable=${DDB_TABLE} -X github.com/greg-szabo/f11/defaults.TestnetName=${TESTNET_NAME} -X github.com/greg-szabo/f11/defaults.TestnetInstance=${DDB_INSTANCE} -X github.com/greg-szabo/f11/defaults.AWSRegion=${AWS_REGION}"

########################################
### Build

build:
	go build $(BUILD_FLAGS) -o build/f11 .

build-linux:
	#GOOS=linux GOARCH=amd64 $(MAKE) build
	docker run -it --rm -v $(GOPATH):/go golang:1.10.3 make -C /go/src/github.com/greg-szabo/f11 build

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
	@go test -count 1 -p 1 `go list github.com/greg-szabo/f11`

test_unit:
	@go test $(PACKAGES)


########################################
### Localnet (Requirements: pip3 install aws-sam-cli)

localnet-start:
	sam local start-api


########################################
### Release management (set up requirements manually)

package:
	if [ -z "$(S3BUCKET)" ]; then echo "Please set S3BUCKET for packaging." ; false ; fi
	zip "build/f11_${TESTNET_NAME}.zip" build/f11 template.yml
	aws s3 cp "build/f11_${TESTNET_NAME}.zip" "s3://$(S3BUCKET)/f11_${TESTNET_NAME}.zip"
#	aws s3 cp "f11swagger.yml" "s3://$(S3BUCKET)/f11swagger.yml"

deploy:
	sam deploy --template-file template.yml --stack-name "f11-${TESTNET_NAME}" --capabilities CAPABILITY_IAM --region "${AWS_REGION}"

.PHONY: build build-linux check_tools update_tools get_tools get_vendor_deps test test_cli test_unit package deploy

