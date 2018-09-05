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
const Minor = "3"
const ContentType = "application/json; charset=utf8"

// This will be overwritten during build. Do not try to manage it here.
var Release = "0-dev"

// Calculated value
var Version = Major + "." + Minor + "." + Release

///
/// Rate limiter settings
///

var LimiterMaxRate = throttled.PerMin(60)

var LimiterMaxBurst = 0
