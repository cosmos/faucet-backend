package main

import (
	"github.com/cosmos/faucet-backend/context"
	"github.com/cosmos/faucet-backend/defaults"
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_MainHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.New()
	rr := httptest.NewRecorder()
	handler := context.Handler{ctx, MainHandler}

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := "{\"message\":\"\",\"name\":\"" + ctx.Cfg.TestnetName + "\",\"version\":\"" + defaults.Version + "\"}\n"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}
