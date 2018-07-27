PACKAGES=$(shell go list ./... | grep -v '/vendor/')
BUILD_NUMBER ?= 3

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
### Localnet (Requirements: pip3 install aws-sam-cli)

localnet-start:
	sam local start-api --env-vars config.json


########################################
### Release management (set up requirements manually)

package:
	if [ -z "$(AWS_NAME)" ]; then echo "Please set AWS_NAME for packaging." ; false ; fi
	if [ -z "$(S3_BUCKET)" ]; then echo "Please set S3_BUCKET for packaging." ; false ; fi
	zip "build/f11_${AWS_NAME}.zip" build/f11 template.yml
	aws s3 cp "build/f11_${AWS_NAME}.zip" "s3://$(S3_BUCKET)/f11_${AWS_NAME}.zip"
#	aws s3 cp "f11swagger.yml" "s3://$(S3BUCKET)/f11swagger.yml"

deploy:
	if [ -z "$(AWS_REGION)" ]; then echo "Please set AWS_REGION for deployment." ; false ; fi
	sam deploy --template-file template.yml --stack-name "f11-${AWS_NAME}" --capabilities CAPABILITY_IAM --region "${AWS_REGION}"

.PHONY: build build-linux check_tools update_tools get_tools get_vendor_deps test test_cli test_unit package deploy

