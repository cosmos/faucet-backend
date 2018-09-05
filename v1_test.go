package main

import (
	"net/http"
)

import (
	"github.com/cosmos/faucet-backend/context"
	"github.com/cosmos/faucet-backend/defaults"
	"net/http/httptest"
	"testing"
)

// Todo: implement V1ClaimHandler testing again. The first version was created when the complete implementation wasn't ready yet.
func Test_ClaimHandlerV1(t *testing.T) {
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
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := "{\"message\":\"\",\"name\":\"" + ctx.TestnetName + "\",\"version\":\"" + defaults.Version + "\"}\n"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
	/*
		handler := context.Handler{ctx, V1ClaimHandler}
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}

		expected := "{\"message\":\"request submitted\"}\n"
		if rr.Body.String() != expected {
			t.Errorf("handler returned unexpected body: got %v want %v",
				rr.Body.String(), expected)
		}
	*/
}
