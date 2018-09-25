// Default package implements versioning primitives.
package defaults

import "github.com/throttled/throttled"

// Major version number.
const Major = "0"

// Minor version number.
const Minor = "4"

// Default Content-Type used for web calls.
const ContentType = "application/json; charset=utf8"

// Release number. It will be overwritten during build. Do not try to manage it here.
var Release = "0-dev"

// Version compiled into a string.
var Version = Major + "." + Minor + "." + Release

// LimiterMaxRate sets the throttling limit
var LimiterMaxRate = throttled.PerMin(10)

// LimiterMaxBurst sets the maximum burst when the limit has been reached.
var LimiterMaxBurst = 0
