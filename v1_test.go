package main

import (
	"github.com/cosmos/faucet-backend/context"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestClaimHandlerV1 tests the /v1/claim endpoint. Have your AWS credentials ready
func TestClaimHandlerV1(t *testing.T) {

	data := "{\"address\":\"cosmosaccaddr1kje2wjc66mc3u283dy80czej8m9su8ca5a8drz\"}"
	req, err := http.NewRequest("POST", "/v1/claim", strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.New()
	ctx.DisableSend = true
	ctx.DisableRecaptcha = true
	ctx.DisableLimiter = true
	rr := httptest.NewRecorder()
	handler := context.Handler{ctx, V1ClaimHandler}

	handler.ServeHTTP(rr, req)
	status := rr.Code
	assert.Equal(t,status,http.StatusOK)

	expected := "{\"message\":\"transaction committed\",\"hash\":\"SendDisabled\",\"height\":0}\n"
	assert.Equal(t, expected, rr.Body.String())
}
