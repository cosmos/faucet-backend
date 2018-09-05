package main

import (
	"encoding/json"
	"errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"time"

	"github.com/cosmos/cosmos-sdk/x/bank/client"
	f11context "github.com/cosmos/faucet-backend/context"
	"github.com/dpapathanasiou/go-recaptcha"
	"github.com/tendermint/tendermint/crypto/encoding/amino"
	"github.com/tendermint/tendermint/libs/bech32"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tomasen/realip"
	"log"
	"net/http"
	"strconv"
)

func V1ClaimHandler(ctx *f11context.Context, w http.ResponseWriter, r *http.Request) (status int, err error) {
	status = http.StatusInternalServerError

	var claim struct {
		Address  string `json:"address"`
		Response string `json:"response"`
	}

	// decode JSON response from body
	err = json.NewDecoder(r.Body).Decode(&claim)
	if err != nil {
		return
	}

	// make sure address is bech32 encoded
	hrp, decodedAddress, err := bech32.DecodeAndConvert(claim.Address)
	if err != nil {
		return
	}

	// encode the address in bech32
	encodedAddress, err := bech32.ConvertAndEncode(hrp, decodedAddress)
	if err != nil {
		return
	}

	// make sure captcha is valid
	clientIP := realip.FromRequest(r)

	if !ctx.DisableRecaptcha {
		var captchaPassed bool
		captchaPassed, err = recaptcha.Confirm(clientIP, claim.Response)
		if err != nil {
			return
		}
		if !captchaPassed {
			return status, errors.New("shoo robot, recaptcha failed")
		}
	} else {
		log.Print("Recaptcha disabled")
	}

	message := "transaction committed"
	var height int64
	hash := "SendDisabled"
	if !ctx.DisableSend {
		height, hash, status, err = V1SendTx(ctx, encodedAddress)
		if err != nil {
			return
		}
	}
	status = http.StatusOK

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(struct {
		Message string `json:"message"`
		Hash    string `json:"hash"`
		Height  int64  `json:"height"`
	}{
		Message: message,
		Height:  height,
		Hash:    hash,
	})
	return
}

func V1SendTx(ctx *f11context.Context, toBech32 string) (height int64, hash string, status int, err error) {
	status = http.StatusInternalServerError
	// Get Hex addresses
	from, err := sdk.AccAddressFromBech32(ctx.Cfg.AccountAddress)
	if err != nil {
		return
	}

	to, err := sdk.AccAddressFromBech32(toBech32)
	if err != nil {
		status = http.StatusBadRequest
		return
	}

	publicKey, err := sdk.GetAccPubKeyBech32(ctx.Cfg.PublicKey)
	if err != nil {
		return
	}

	// Parse coins
	coins, err := sdk.ParseCoins(ctx.Cfg.Amount)
	if err != nil {
		return
	}

	//Todo: (low prio) Implement account check for enough coins by deriving coin number from sequence number (c - s = remaining coins)

	// build the transaction
	msg := client.BuildMsg(from, to, coins)

	// No fee
	fee := sdk.Coin{}

	// There's nothing to see here, move along.
	memo := "faucet drop"

	// In case the previous run flagged a broken setup, try to fix it.
	err = ctx.CheckAndFixAccountDetails()
	if err != nil {
		return
	}

	ctx.SequenceMutex.Lock()
	sequence, err := strconv.ParseInt(ctx.SequenceMutex.Value, 10, 64)
	if err != nil {
		ctx.SequenceMutex.Unlock()
		return
	}

	// Message
	signMsg := auth.StdSignMsg{
		ChainID:       ctx.TestnetName,
		AccountNumber: ctx.AccountNumber,
		Sequence:      sequence,
		Msgs:          []sdk.Msg{msg},
		Memo:          memo,
		Fee:           auth.NewStdFee(ctx.TxContest.Gas, fee),
	}
	bz := signMsg.Bytes()

	// Get private key
	privateKeyBytes, err := GetPrivkeyBytesFromString(ctx.Cfg.PrivateKey)
	if err != nil {
		ctx.SequenceMutex.Unlock()
		return
	}
	privateKey, err := cryptoAmino.PrivKeyFromBytes(privateKeyBytes)

	// Sign message
	sig, err := privateKey.Sign(bz)

	sigs := []auth.StdSignature{{
		PubKey:        publicKey,
		Signature:     sig,
		AccountNumber: ctx.AccountNumber,
		Sequence:      sequence,
	}}

	// marshal bytes
	tx := auth.NewStdTx(signMsg.Msgs, signMsg.Fee, sigs, memo)

	// Broadcast to Tendermint
	txBytes, err := ctx.Cdc.MarshalBinary(tx)
	if err != nil {
		ctx.SequenceMutex.Unlock()
		return
	}
	log.Printf("Sending transaction sequence %s", ctx.SequenceMutex.Value)

	cres := make(chan AsyncResponse, 1)
	go func() {
		res, err := ctx.CLIContext.BroadcastTx(txBytes)
		cres <- AsyncResponse{
			Result: res,
			Error:  err,
		}
	}()

	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(time.Duration(ctx.Cfg.Timeout) * time.Second)
		timeout <- true
	}()

	select {
	case response := <-cres:
		var res *ctypes.ResultBroadcastTxCommit
		res, err = response.Result, response.Error
		if err != nil {
			ctx.SequenceMutex.Unlock()
			return
		}
		log.Printf("Sent transaction sequence %s", ctx.SequenceMutex.Value)
		sequence++
		ctx.SequenceMutex.Value = strconv.FormatInt(sequence, 10)
		ctx.SequenceMutex.Unlock()
		return res.Height, res.Hash.String(), http.StatusOK, nil
	case <-timeout:
		ctx.BrokenFlagMutex.Lock()
		ctx.BrokenFlagMutex.Value = "broken"
		ctx.BrokenFlagMutex.Unlock()
		ctx.SequenceMutex.Unlock()
		err = errors.New("broadcasting transaction timed out")
		return

	}
}

type AsyncResponse struct {
	Result *ctypes.ResultBroadcastTxCommit
	Error  error
}
