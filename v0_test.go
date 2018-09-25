package main

import (
	"github.com/cosmos/faucet-backend/context"
	"github.com/cosmos/faucet-backend/defaults"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetPrivkeyBytesFromString(t *testing.T) {
	result, err := GetPrivkeyBytesFromString("AQIDBAUGBwgJAAA=")
	assert.Nil(t, err)
	assert.Equal(t, result, []byte{1,2,3,4,5,6,7,8,9,0,0})
}

func TestGetStringFromPrivkeyBytes(t *testing.T) {
	assert.Equal(t, GetStringFromPrivkeyBytes([]byte{1,2,3,4,5,6,7,8,9,0,0}), "AQIDBAUGBwgJAAA=")
}

func TestMainHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.New()
	ctx.DisableSend = true
	ctx.DisableRecaptcha = true
	ctx.DisableLimiter = true
	rr := httptest.NewRecorder()
	handler := context.Handler{ctx, MainHandler}

	handler.ServeHTTP(rr, req)
	status := rr.Code
	assert.Equal(t,status,http.StatusOK)

	expected := "{\"message\":\"\",\"name\":\"" + ctx.TestnetName + "\",\"version\":\"" + defaults.Version + "\"}\n"
	assert.Equal(t, expected, rr.Body.String())
}
