package defaults

import "github.com/throttled/throttled"

//
// This file contains the input parameters for the lambda function.
// Lambda functions cannot take command-line parameters, hence they are baked into the binary.
// The rest of the configuration is read from the DynamoDB table defined here.
//

//
// Versioning
//
const Major = "0"
const Minor = "1"
const ContentType = "application/json; charset=utf8"

// This will be overwritten during build. Do not try to manage it here.
var Release = "0-dev"

// Calculated value
var Version = Major + "." + Minor + "." + Release

//
// Configuration database connection parameters. These will be overwritten during build.
// Todo: Put them into environment variables on Lambda so they are not baked into the code.
//

// Table name
var DynamoDBTable = ""

// Key attribute names
var DynamoDBTestnetNameColumn = "testnet-name"
var DynamoDBTestnetInstanceColumn = "testnet-instance"

// Primary key - the testnet name
var TestnetName = "devnet"

// Secondary key - the configuration version for this testnet
var TestnetInstance = "internal"

// The AWS region to use to open the AWS session
var AWSRegion = "us-east-1"

///
/// Rate limiter settings
///

var LimiterMaxRate = throttled.PerMin(60)

var LimiterMaxBurst = 0

//
// Values for testing - do not use in production environment
//

// (--no-limit) Disable rate limiter
var DisableLimiter = false

// (--no-send) Disable transaction send to the blockchain network
var DisableSend = false

// (--no-recaptcha) Disable recaptcha
var DisableRecaptcha = false
