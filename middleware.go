package main

import (
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/greg-szabo/f11/context"
	"github.com/greg-szabo/f11/defaults"
	"github.com/rs/cors"
	"github.com/throttled/throttled"
	"github.com/throttled/throttled/store/goredisstore"
	"github.com/throttled/throttled/store/memstore"
	"github.com/tomasen/realip"
	"log"
	"net/http"
)

// Create logs for each request
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s", realip.FromRequest(r), r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

type addContext struct {
	ctx         *context.Context
	Contextware func(ctx *context.Context, next http.Handler) http.Handler
}

func (fn addContext) Middleware(next http.Handler) http.Handler {
	return fn.Contextware(fn.ctx, next)
}

func createThrottledMiddleware(ctx *context.Context) mux.MiddlewareFunc {
	throttledContextMiddleware := addContext{
		ctx: ctx,
		Contextware: func(ctx *context.Context, next http.Handler) http.Handler {
			return ctx.HttpRateLimiter.RateLimit(next)
		},
	}
	return throttledContextMiddleware.Middleware
}

// Todo: Let the API Gateway handle CORS, instead of handling it in code.
// Create CORS middleware
func createCORSMiddleware(ctx *context.Context) mux.MiddlewareFunc {
	corsContextMiddleware := addContext{
		ctx: ctx,
		Contextware: func(ctx *context.Context, next http.Handler) http.Handler {
			return cors.New(cors.Options{
				AllowedOrigins: ctx.Cfg.Origins,
				AllowedMethods: []string{"GET", "POST", "OPTIONS"},
				AllowedHeaders: []string{"*"},
			}).Handler(next)
		},
	}
	return corsContextMiddleware.Middleware
}

// Todo: Better define IP throttling requirements and storage
// Create throttled rate limiter with redisstore for remote execution
func createRedisStore(ctx *context.Context) (throttled.GCRAStore, error) {
	return goredisstore.New(redis.NewClient(&redis.Options{
		Addr:     ctx.Cfg.RedisEndpoint,
		Password: ctx.Cfg.RedisPassword,
		DB:       0,
	}), ctx.Cfg.TestnetName+"-"+defaults.TestnetInstance)
}

// Create throttled rate limiter with memstore for local execution
func createMemStore() (throttled.GCRAStore, error) {
	return memstore.New(65536)
}

// Finish creating throttled rate limiter
func createThrottledLimiter(ctx *context.Context) (err error) {
	var rateLimiter *throttled.GCRARateLimiter
	rateLimiter, err = throttled.NewGCRARateLimiter(ctx.Store, throttled.RateQuota{MaxRate: defaults.LimiterMaxRate, MaxBurst: defaults.LimiterMaxBurst})
	if err != nil {
		return
	}
	ctx.HttpRateLimiter = throttled.HTTPRateLimiter{
		DeniedHandler: nil,
		Error:         nil,
		RateLimiter:   rateLimiter,
		VaryBy:        &throttled.VaryBy{Headers: []string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"}},
	}
	return
}
