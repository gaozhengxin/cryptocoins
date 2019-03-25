package zcash

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	rpcutils "github.com/gaozhengxin/cryptocoins/src/go/rpcutils"
	"github.com/gaozhengxin/cryptocoins/src/go/btc"
	"github.com/gaozhengxin/cryptocoins/src/go/config"
)

var ChainConfig = chaincfg.Params {
	PubKeyHashAddrID: 0x4b,
}

var btcHandler = new(btc.BTCTransactionHandler)

var allowHighFees = true

type ZCASHTransactionHandler struct {}

func (h *ZCASHTransactionHandler) PublicKeyToAddress(pubKeyHex string) (address string, msg string, err error){
	if pubKeyHex[:2] == "0x" || pubKeyHex[:2] == "0X" {
		pubKeyHex = pubKeyHex[2:]
	}
	bb, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return
	}
	pubKey, err := btcec.ParsePubKey(bb, btcec.S256())
	if err != nil {
		return
	}
	b := pubKey.SerializeCompressed()
	pkHash := btcutil.Hash160(b)
	addressPubKeyHash, err := btcutil.NewAddressPubKeyHash(pkHash, &ChainConfig)
	if err != nil {
		return
	}
	address = addressPubKeyHash.EncodeAddress()
	address = "t" + address
	return
}

// NOT completed, may or not work
func (h *ZCASHTransactionHandler) BuildUnsignedTransaction(fromAddress, fromPublicKey, toAddress string, amount *big.Int, args []interface{}) (transaction interface{}, digests []string, err error) {
	transaction, digests, err = btcHandler.BuildUnsignedTransaction(fromAddress, fromPublicKey, toAddress, amount, args)
	return
}

// NOT completed, may or not work
func (h *ZCASHTransactionHandler) SignTransaction(hash []string, wif interface{}) (rsv []string, err error){
	return btcHandler.SignTransaction(hash, wif)
}

// NOT completed, may or not work
func (h *ZCASHTransactionHandler) MakeSignedTransaction(rsv []string, transaction interface{}) (signedTransaction interface{}, err error){
	return btcHandler.MakeSignedTransaction(rsv, transaction)
}

// NOT completed, may or not work
func (h *ZCASHTransactionHandler) SubmitTransaction(signedTransaction interface{}) (ret string, err error) {
	c, _ := rpcutils.NewClient(config.ZCASH_SERVER_HOST,config.ZCASH_SERVER_PORT,config.ZCASH_USER,config.ZCASH_PASSWD,config.ZCASH_USESSL)
	ret, err= btc.SendRawTransaction (c, signedTransaction.(*btc.AuthoredTx).Tx, allowHighFees)
	return
}

func (h *ZCASHTransactionHandler) GetTransactionInfo(txhash string) (fromAddress, toAddress string, transferAmount *big.Int, _ []interface{}, err error) {
	cmd := btcjson.NewGetRawTransactionCmd(txhash, nil)

	marshalledJSON, err := btcjson.MarshalCmd(1, cmd)
	if err != nil {
		return
	}

	c, _ := rpcutils.NewClient(config.ZCASH_SERVER_HOST,config.ZCASH_SERVER_PORT,config.ZCASH_USER,config.ZCASH_PASSWD,config.ZCASH_USESSL)
	retJSON, err := c.Send(string(marshalledJSON))
	if err != nil {
		return
	}

	result, err := parseRPCReturn(retJSON)
	if err != nil {
		return
	}
	rawTxStr := result.(string)

	cmd2 := btcjson.NewDecodeRawTransactionCmd(rawTxStr)

	marshalledJSON2, err := btcjson.MarshalCmd(1, cmd2)
	if err != nil {
		return
	}

	retJSON2, err := c.Send(string(marshalledJSON2))
	if err != nil {
		return
	}

	result, err = parseRPCReturn(retJSON2)
	if err != nil {
		return
	}

	toAddress = result.(map[string]interface{})["vout"].([]interface{})[0].(map[string]interface{})["scriptPubKey"].(map[string]interface{})["addresses"].([]interface{})[0].(string)
	flt := result.(map[string]interface{})["vout"].([]interface{})[0].(map[string]interface{})["value"].(float64)
	amt, err := btcutil.NewAmount(flt)
	transferAmount = big.NewInt(int64(amt.ToUnit(btcutil.AmountSatoshi)))

	// from where
	vintx := result.(map[string]interface{})["vin"].([]interface{})[0].(map[string]interface{})["txid"]
	if vintx == nil {
		coinbase := result.(map[string]interface{})["vin"].([]interface{})[0].(map[string]interface{})["coinbase"]
		if coinbase != nil {
			fromAddress = coinbase.(string)
			return
		}
	}

	// as which output in previous transaction
	vinvout := int(result.(map[string]interface{})["vin"].([]interface{})[0].(map[string]interface{})["vout"].(float64))

	cmd3 := btcjson.NewGetRawTransactionCmd(vintx.(string), nil)

	marshalledJSON3, err := btcjson.MarshalCmd(1, cmd3)
	if err != nil {
		return
	}

	retJSON3, err := c.Send(string(marshalledJSON3))
	if err != nil {
		return
	}

	var rawTx2 interface{}
	json.Unmarshal([]byte(retJSON3), &rawTx2)
	rawTxStr2 := rawTx2.(map[string]interface{})["result"].(string)

	cmd4 := btcjson.NewDecodeRawTransactionCmd(rawTxStr2)

	marshalledJSON4, err := btcjson.MarshalCmd(1, cmd4)
	if err != nil {
		return
	}

	retJSON4, err := c.Send(string(marshalledJSON4))
	if err != nil {
		return
	}

	var tx2 interface{}
	json.Unmarshal([]byte(retJSON4), &tx2)

	fromAddress = tx2.(map[string]interface{})["result"].(map[string]interface{})["vout"].([]interface{})[vinvout].(map[string]interface{})["scriptPubKey"].(map[string]interface{})["addresses"].([]interface{})[0].(string)

	return
}

// TODO
func (h *ZCASHTransactionHandler) GetAddressBalance(address string, args []interface{}) (balance *big.Int, err error){
	err = fmt.Errorf("function currently not available")
	return nil, err
}

func parseRPCReturn (retJSON string) (result interface{}, err error) {
	var ret interface{}
	json.Unmarshal([]byte(retJSON), &ret)
	result = ret.(map[string]interface{})["result"]
	if result == nil {
		errStr := ret.(map[string]interface{})["error"]
		if errStr == nil {
			err = fmt.Errorf("unknown error")
			return
		}
		err = fmt.Errorf(errStr.(string))
	}
	return
}
